// Package bulktg downloads multiple Telegram media files sequentially into a
// single folder, presenting the whole batch as one status.Status entry.
package bulktg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/gotd/td/tg"

	"mirrorbot/internal/status"
	"mirrorbot/internal/tgbot"
	"mirrorbot/internal/util"
)

var errCancelled = errors.New("cancelled by user")

// Item is one file to download.
type Item struct {
	Location tg.InputFileLocationClass
	FileName string
	Size     int64
}

// Download downloads a batch of Telegram files into baseDir.
type Download struct {
	bot        *tgbot.Bot
	items      []Item
	folderName string
	baseDir    string
	gid        string
	listener   status.Listener

	completed atomic.Int64
	total     atomic.Int64
	speed     atomic.Int64
	index     atomic.Int64
	cancelled atomic.Bool
	failed    atomic.Bool

	observerStop chan struct{}
}

// New creates a bulk download. baseDir is the folder all files are written into;
// folderName is its display name.
func New(bot *tgbot.Bot, items []Item, folderName, baseDir, gid string, listener status.Listener) *Download {
	var total int64
	for _, it := range items {
		total += it.Size
	}
	d := &Download{bot: bot, items: items, folderName: folderName, baseDir: baseDir, gid: gid, listener: listener}
	d.total.Store(total)
	return d
}

// Start downloads each file in order into baseDir, then signals completion.
func (d *Download) Start(ctx context.Context) {
	if err := os.MkdirAll(d.baseDir, 0o755); err != nil {
		d.fail(err)
		return
	}
	d.listener.OnDownloadStart()
	d.startObserver()
	defer d.stopObserver()

	seen := map[string]int{}
	for _, it := range d.items {
		if d.cancelled.Load() {
			d.listener.OnDownloadError(errCancelled)
			return
		}
		name := uniqueName(seen, filepath.Base(it.FileName))
		dest := filepath.Join(d.baseDir, name)
		f, err := os.Create(dest)
		if err != nil {
			d.fail(err)
			return
		}
		err = d.bot.DownloadMedia(ctx, it.Location, &countWriter{d: d, w: f})
		f.Close()
		if err != nil {
			if d.cancelled.Load() || errors.Is(err, errCancelled) {
				d.listener.OnDownloadError(errCancelled)
				return
			}
			d.fail(err)
			return
		}
	}
	d.listener.OnDownloadComplete()
}

func (d *Download) fail(err error) {
	d.failed.Store(true)
	d.listener.OnDownloadError(err)
}

// --- status.Status ---

func (d *Download) Name() string           { return d.folderName }
func (d *Download) CompletedLength() int64  { return d.completed.Load() }
func (d *Download) TotalLength() int64      { return d.total.Load() }
func (d *Download) Speed() int64            { return d.speed.Load() }
func (d *Download) GID() string             { return d.gid }
func (d *Download) Path() string            { return d.baseDir }

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

// uniqueName avoids overwriting files with duplicate names within the batch.
func uniqueName(seen map[string]int, name string) string {
	if _, ok := seen[name]; !ok {
		seen[name] = 1
		return name
	}
	ext := filepath.Ext(name)
	base := name[:len(name)-len(ext)]
	for {
		seen[name]++
		candidate := fmt.Sprintf("%s_%d%s", base, seen[name], ext)
		if _, ok := seen[candidate]; !ok {
			seen[candidate] = 1
			return candidate
		}
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
