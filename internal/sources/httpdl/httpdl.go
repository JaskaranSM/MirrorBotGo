// Package httpdl is a streaming HTTP(S) download source implementing status.Status.
package httpdl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"mirrorbot/internal/metrics"
	"mirrorbot/internal/status"
	"mirrorbot/internal/util"
)

var errCancelled = errors.New("cancelled by user")

// Download is an HTTP download unit.
type Download struct {
	url      string
	dir      string
	gid      string
	listener status.Listener
	client   *http.Client

	nameMu    sync.RWMutex
	name      string
	path      string
	completed atomic.Int64
	total     atomic.Int64
	speed     atomic.Int64
	index     atomic.Int64
	cancelled atomic.Bool
	failed    atomic.Bool
}

// New creates an HTTP download for url saving into dir.
func New(url, dir, gid string, listener status.Listener) *Download {
	return &Download{
		url:      url,
		dir:      dir,
		gid:      gid,
		listener: listener,
		client:   &http.Client{},
		name:     util.GetFileBaseName(url),
	}
}

// Start runs the download in the current goroutine, invoking listener callbacks.
func (d *Download) Start(ctx context.Context) {
	start := time.Now()
	result := metrics.ResultError
	metrics.MirrorsStarted.WithLabelValues(metrics.SourceHTTP).Inc()
	defer func() {
		metrics.MirrorsFinished.WithLabelValues(metrics.SourceHTTP, result).Inc()
		metrics.DownloadBytes.WithLabelValues(metrics.SourceHTTP).Add(float64(d.completed.Load()))
		metrics.DownloadDuration.WithLabelValues(metrics.SourceHTTP).Observe(time.Since(start).Seconds())
	}()

	if err := os.MkdirAll(d.dir, 0o755); err != nil {
		d.fail(err)
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.url, nil)
	if err != nil {
		d.fail(err)
		return
	}
	resp, err := d.client.Do(req)
	if err != nil {
		d.fail(err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		d.fail(fmt.Errorf("http status %d", resp.StatusCode))
		return
	}

	name := filenameFromResponse(resp, d.url)
	d.setName(name)
	dest := filepath.Join(d.dir, name)
	d.nameMu.Lock()
	d.path = dest
	d.nameMu.Unlock()
	if resp.ContentLength > 0 {
		d.total.Store(resp.ContentLength)
	}

	out, err := os.Create(dest)
	if err != nil {
		d.fail(err)
		return
	}

	d.listener.OnDownloadStart()
	d.startObserver()
	_, err = io.Copy(&countWriter{d: d, w: out}, resp.Body)
	out.Close()
	d.stopObserver()

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

func (d *Download) Name() string {
	d.nameMu.RLock()
	defer d.nameMu.RUnlock()
	return d.name
}
func (d *Download) setName(n string) { d.nameMu.Lock(); d.name = n; d.nameMu.Unlock() }

func (d *Download) CompletedLength() int64 { return d.completed.Load() }
func (d *Download) TotalLength() int64     { return d.total.Load() }
func (d *Download) Speed() int64           { return d.speed.Load() }
func (d *Download) GID() string            { return d.gid }

func (d *Download) Path() string {
	d.nameMu.RLock()
	defer d.nameMu.RUnlock()
	return d.path
}

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

var observerStops = struct {
	mu sync.Mutex
	m  map[*Download]chan struct{}
}{m: map[*Download]chan struct{}{}}

func (d *Download) startObserver() {
	stop := make(chan struct{})
	observerStops.mu.Lock()
	observerStops.m[d] = stop
	observerStops.mu.Unlock()
	go func() {
		last := d.completed.Load()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
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
	observerStops.mu.Lock()
	if stop, ok := observerStops.m[d]; ok {
		close(stop)
		delete(observerStops.m, d)
	}
	observerStops.mu.Unlock()
}

type countWriter struct {
	d *Download
	w io.Writer
}

func (c *countWriter) Write(p []byte) (int, error) {
	if c.d.cancelled.Load() {
		return 0, errCancelled
	}
	n, err := c.w.Write(p)
	c.d.completed.Add(int64(n))
	return n, err
}

func filenameFromResponse(resp *http.Response, url string) string {
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			if fn := params["filename"]; fn != "" {
				return filepath.Base(fn)
			}
		}
	}
	name := util.GetFileBaseName(url)
	if i := indexAny(name, "?#"); i >= 0 {
		name = name[:i]
	}
	if name == "" || name == "/" || name == "." {
		return "download"
	}
	return name
}

func indexAny(s, chars string) int {
	for i, r := range s {
		for _, c := range chars {
			if r == c {
				return i
			}
		}
	}
	return -1
}
