// Package tgfile downloads a file from a Telegram message via the bot's MTProto
// connection, exposing progress through status.Status.
package tgfile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/gotd/td/tg"

	"mirrorbot/internal/metrics"
	"mirrorbot/internal/status"
	"mirrorbot/internal/tgbot"
	"mirrorbot/internal/util"
)

var errCancelled = errors.New("cancelled by user")

// Download downloads a single Telegram media object (document or photo).
type Download struct {
	bot      *tgbot.Bot
	location tg.InputFileLocationClass
	fileName string
	dir      string
	gid      string
	listener status.Listener

	completed atomic.Int64
	total     atomic.Int64
	speed     atomic.Int64
	index     atomic.Int64
	cancelled atomic.Bool
	failed    atomic.Bool

	path         string
	observerStop chan struct{}
}

// New creates a Telegram media download from a fetched media location.
func New(bot *tgbot.Bot, loc tg.InputFileLocationClass, fileName string, size int64, dir, gid string, listener status.Listener) *Download {
	if fileName == "" {
		fileName = "telegram_file"
	}
	d := &Download{bot: bot, location: loc, fileName: fileName, dir: dir, gid: gid, listener: listener}
	d.total.Store(size)
	return d
}

// Start runs the download in the current goroutine.
func (d *Download) Start(ctx context.Context) {
	start := time.Now()
	result := metrics.ResultError
	metrics.MirrorsStarted.WithLabelValues(metrics.SourceTGFile).Inc()
	defer func() {
		metrics.MirrorsFinished.WithLabelValues(metrics.SourceTGFile, result).Inc()
		metrics.DownloadBytes.WithLabelValues(metrics.SourceTGFile).Add(float64(d.completed.Load()))
		metrics.DownloadDuration.WithLabelValues(metrics.SourceTGFile).Observe(time.Since(start).Seconds())
	}()

	if err := os.MkdirAll(d.dir, 0o755); err != nil {
		d.fail(err)
		return
	}
	d.path = filepath.Join(d.dir, d.fileName)
	f, err := os.Create(d.path)
	if err != nil {
		d.fail(err)
		return
	}

	d.listener.OnDownloadStart()
	d.startObserver()
	err = d.bot.DownloadMedia(ctx, d.location, &countWriter{d: d, w: f})
	d.stopObserver()
	f.Close()

	if err != nil {
		if d.cancelled.Load() || errors.Is(err, errCancelled) {
			result = metrics.ResultCancelled
			d.listener.OnDownloadError(errCancelled)
			return
		}
		d.fail(err)
		return
	}
	result = metrics.ResultComplete
	d.listener.OnDownloadComplete()
}

func (d *Download) fail(err error) {
	d.failed.Store(true)
	d.listener.OnDownloadError(err)
}

// --- status.Status ---

func (d *Download) Name() string           { return d.fileName }
func (d *Download) CompletedLength() int64 { return d.completed.Load() }
func (d *Download) TotalLength() int64     { return d.total.Load() }
func (d *Download) Speed() int64           { return d.speed.Load() }
func (d *Download) GID() string            { return d.gid }
func (d *Download) Path() string           { return d.path }

func (d *Download) Percentage() float32 {
	total := d.total.Load()
	if total == 0 {
		return 0
	}
	return float32(d.completed.Load()*100) / float32(total)
}

func (d *Download) ETA() *time.Duration {
	left := d.total.Load() - d.completed.Load()
	if left < 0 {
		left = 0
	}
	eta := util.CalculateETA(left, d.speed.Load())
	return &eta
}

func (d *Download) StatusType() status.StatusType {
	if d.cancelled.Load() {
		return status.Canceled
	}
	if d.failed.Load() {
		return status.Failed
	}
	return status.Downloading
}

func (d *Download) Index() int     { return int(d.index.Load()) }
func (d *Download) SetIndex(i int) { d.index.Store(int64(i)) }
func (d *Download) Cancel() bool   { d.cancelled.Store(true); return true }

// --- helpers ---

func (d *Download) startObserver() {
	d.observerStop = make(chan struct{})
	go func() {
		last := d.completed.Load()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-d.observerStop:
				return
			case <-ticker.C:
				now := d.completed.Load()
				d.speed.Store(now - last)
				last = now
			}
		}
	}()
}

func (d *Download) stopObserver() {
	if d.observerStop != nil {
		close(d.observerStop)
	}
}

type countWriter struct {
	d *Download
	w interface{ Write([]byte) (int, error) }
}

func (c *countWriter) Write(p []byte) (int, error) {
	if c.d.cancelled.Load() {
		return 0, errCancelled
	}
	n, err := c.w.Write(p)
	c.d.completed.Add(int64(n))
	return n, err
}
