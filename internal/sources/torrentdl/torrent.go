// Package torrentdl embeds an anacrolix/torrent engine and exposes each torrent
// as a status.Status (and status.TorrentInfo), replacing the external kedge service.
package torrentdl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
	"golang.org/x/time/rate"

	"mirrorbot/internal/metrics"
	"mirrorbot/internal/status"
	"mirrorbot/internal/util"
)

var errCancelled = errors.New("cancelled by user")

// EngineConfig configures the embedded torrent client.
type EngineConfig struct {
	DownloadDir                string
	ListenPort                 int
	PeerIDPrefix               string
	MaxUploadRate              string
	EstablishedConnsPerTorrent int
	UseTrackerList             bool
	TrackerListURL             string
}

// Engine wraps a single shared torrent client.
type Engine struct {
	client   *torrent.Client
	trackers []string
}

// NewEngine builds and starts the torrent client.
func NewEngine(cfg EngineConfig) (*Engine, error) {
	c := torrent.NewDefaultClientConfig()
	c.DataDir = cfg.DownloadDir
	if cfg.ListenPort > 0 {
		c.ListenPort = cfg.ListenPort
	}
	c.Seed = true // completed torrents may seed; non-seed mirrors are dropped after upload
	if cfg.EstablishedConnsPerTorrent > 0 {
		c.EstablishedConnsPerTorrent = cfg.EstablishedConnsPerTorrent
	}
	if cfg.PeerIDPrefix != "" {
		c.PeerID = makePeerID(cfg.PeerIDPrefix)
	}
	if bps := parseRate(cfg.MaxUploadRate); bps > 0 {
		c.UploadRateLimiter = rate.NewLimiter(rate.Limit(bps), 1<<18)
	}

	client, err := torrent.NewClient(c)
	if err != nil {
		return nil, fmt.Errorf("torrent client: %w", err)
	}
	e := &Engine{client: client}
	if cfg.UseTrackerList && cfg.TrackerListURL != "" {
		e.trackers = fetchTrackers(cfg.TrackerListURL)
	}
	return e, nil
}

// Close shuts down the torrent client.
func (e *Engine) Close() { e.client.Close() }

// AddMagnet adds a magnet URI. baseDir is the per-mirror storage directory.
func (e *Engine) AddMagnet(uri, baseDir string, seed bool, listener status.Listener) (*Download, error) {
	spec, err := torrent.TorrentSpecFromMagnetUri(uri)
	if err != nil {
		return nil, fmt.Errorf("parse magnet: %w", err)
	}
	return e.add(spec, baseDir, seed, listener)
}

// AddMetaInfo adds a torrent from .torrent file bytes.
func (e *Engine) AddMetaInfo(data []byte, baseDir string, seed bool, listener status.Listener) (*Download, error) {
	mi, err := metainfo.Load(strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("load metainfo: %w", err)
	}
	spec := torrent.TorrentSpecFromMetaInfo(mi)
	return e.add(spec, baseDir, seed, listener)
}

func (e *Engine) add(spec *torrent.TorrentSpec, baseDir string, seed bool, listener status.Listener) (*Download, error) {
	// Use mmap storage rather than file storage: anacrolix's file backend issues
	// blocking syscalls that make the Go runtime spawn a new OS thread on every
	// blocked I/O, which balloons thread count under load. mmap avoids that.
	spec.Storage = storage.NewMMap(baseDir)
	t, isNew, err := e.client.AddTorrentSpec(spec)
	if err != nil {
		return nil, fmt.Errorf("add torrent: %w", err)
	}
	if !isNew {
		return nil, errors.New("torrent already exists")
	}
	if len(e.trackers) > 0 {
		t.AddTrackers([][]string{e.trackers})
	}
	d := &Download{
		t:        t,
		gid:      t.InfoHash().HexString(),
		baseDir:  baseDir,
		seed:     seed,
		listener: listener,
	}
	return d, nil
}

// Download is one torrent, implementing status.Status and status.TorrentInfo.
type Download struct {
	t        *torrent.Torrent
	gid      string
	baseDir  string
	seed     bool
	listener status.Listener

	speed         atomic.Int64
	index         atomic.Int64
	cancelled     atomic.Bool
	seeding       atomic.Bool
	completedFlag atomic.Bool

	cancelCh   chan struct{}
	cancelOnce sync.Once
	recorded   atomic.Bool // terminal metric recorded at most once

	mu            sync.Mutex
	completedTime time.Time
}

// recordTerminal emits the source's terminal metrics once (download completed
// or cancelled/errored); subsequent calls (e.g. cancel after seeding) are no-ops.
func (d *Download) recordTerminal(start time.Time, result string) {
	if d.recorded.CompareAndSwap(false, true) {
		metrics.MirrorsFinished.WithLabelValues(metrics.SourceTorrent, result).Inc()
		metrics.DownloadBytes.WithLabelValues(metrics.SourceTorrent).Add(float64(d.t.BytesCompleted()))
		metrics.DownloadDuration.WithLabelValues(metrics.SourceTorrent).Observe(time.Since(start).Seconds())
	}
}

func (d *Download) cancelChan() chan struct{} {
	d.cancelOnce.Do(func() { d.cancelCh = make(chan struct{}) })
	return d.cancelCh
}

// Start begins watching the torrent (metadata -> download -> completion). Call
// it after registering the Download's status, to avoid a completion firing
// before the status is tracked.
func (d *Download) Start(ctx context.Context) {
	start := time.Now()
	metrics.MirrorsStarted.WithLabelValues(metrics.SourceTorrent).Inc()
	cancel := d.cancelChan()
	select {
	case <-d.t.GotInfo():
	case <-ctx.Done():
		return
	case <-cancel:
		d.recordTerminal(start, metrics.ResultCancelled)
		d.listener.OnDownloadError(errCancelled)
		return
	}

	d.t.DownloadAll()
	d.listener.OnDownloadStart()

	last := d.CompletedLength()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-cancel:
			d.recordTerminal(start, metrics.ResultCancelled)
			d.listener.OnDownloadError(errCancelled)
			return
		case <-ticker.C:
			now := d.CompletedLength()
			d.speed.Store(now - last)
			last = now
			if !d.completedFlag.Load() && d.t.Complete().Bool() {
				d.completedFlag.Store(true)
				d.mu.Lock()
				d.completedTime = time.Now()
				d.mu.Unlock()
				d.recordTerminal(start, metrics.ResultComplete)
				if d.seed {
					d.seeding.Store(true)
				}
				d.listener.OnDownloadComplete()
				if !d.seed {
					return
				}
			}
		}
	}
}

// Drop removes the torrent from the client (does not delete files).
func (d *Download) Drop() { d.t.Drop() }

// --- status.Status ---

func (d *Download) Name() string {
	if n := d.t.Name(); n != "" {
		return n
	}
	return "getting metadata"
}

// hasInfo reports whether torrent metadata is available. Most size/piece
// queries panic if called before this is true.
func (d *Download) hasInfo() bool { return d.t.Info() != nil }

func (d *Download) CompletedLength() int64 {
	if !d.hasInfo() {
		return 0
	}
	if d.seeding.Load() {
		stats := d.t.Stats()
		return d.t.Length() + stats.BytesWrittenData.Int64()
	}
	return d.t.BytesCompleted()
}

func (d *Download) TotalLength() int64 {
	if !d.hasInfo() {
		return 0
	}
	return d.t.Length()
}
func (d *Download) Speed() int64 { return d.speed.Load() }
func (d *Download) GID() string  { return d.gid }
func (d *Download) Path() string { return filepath.Join(d.baseDir, d.t.Name()) }

func (d *Download) Percentage() float32 {
	if !d.hasInfo() {
		return 0
	}
	total := d.t.Length()
	if total == 0 {
		return 0
	}
	return float32(d.t.BytesCompleted()*100) / float32(total)
}

func (d *Download) ETA() *time.Duration {
	if d.seeding.Load() {
		d.mu.Lock()
		st := time.Since(d.completedTime)
		d.mu.Unlock()
		return &st
	}
	if !d.hasInfo() {
		zero := time.Duration(0)
		return &zero
	}
	left := d.t.Length() - d.t.BytesCompleted()
	eta := util.CalculateETA(left, d.speed.Load())
	return &eta
}

func (d *Download) StatusType() status.StatusType {
	if d.cancelled.Load() {
		return status.Canceled
	}
	if d.seeding.Load() {
		return status.Seeding
	}
	if d.t.Info() == nil {
		return status.Waiting
	}
	return status.Downloading
}

func (d *Download) Index() int     { return int(d.index.Load()) }
func (d *Download) SetIndex(i int) { d.index.Store(int64(i)) }

func (d *Download) Cancel() bool {
	if d.cancelled.CompareAndSwap(false, true) {
		close(d.cancelChan())
	}
	return true
}

// --- status.TorrentInfo ---

func (d *Download) Peers() int   { return d.t.Stats().ActivePeers }
func (d *Download) Seeders() int { return d.t.Stats().ConnectedSeeders }
func (d *Download) PiecesTotal() int {
	if !d.hasInfo() {
		return 0
	}
	return d.t.NumPieces()
}
func (d *Download) PiecesCompleted() int { return d.t.Stats().PiecesComplete }

// --- helpers ---

func makePeerID(prefix string) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	id := []byte(prefix)
	if len(id) > 20 {
		return string(id[:20])
	}
	for len(id) < 20 {
		id = append(id, letters[rand.Intn(len(letters))])
	}
	return string(id)
}

// parseRate parses sizes like "100KiB", "1MiB", "500KB", "2MB", "1048576" into bytes/sec.
func parseRate(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	mult := 1.0
	switch {
	case strings.HasSuffix(s, "KiB"):
		mult, s = 1024, strings.TrimSuffix(s, "KiB")
	case strings.HasSuffix(s, "MiB"):
		mult, s = 1024*1024, strings.TrimSuffix(s, "MiB")
	case strings.HasSuffix(s, "GiB"):
		mult, s = 1024*1024*1024, strings.TrimSuffix(s, "GiB")
	case strings.HasSuffix(s, "KB"):
		mult, s = 1000, strings.TrimSuffix(s, "KB")
	case strings.HasSuffix(s, "MB"):
		mult, s = 1000*1000, strings.TrimSuffix(s, "MB")
	case strings.HasSuffix(s, "K"):
		mult, s = 1024, strings.TrimSuffix(s, "K")
	case strings.HasSuffix(s, "M"):
		mult, s = 1024*1024, strings.TrimSuffix(s, "M")
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v * mult
}

func fetchTrackers(url string) []string {
	resp, err := http.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
