package gdrive

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"mirrorbot/internal/metrics"
	"mirrorbot/internal/status"
	"mirrorbot/internal/util"
)

const (
	maxRetries = 5
	chunkSize  = 50 * 1024 * 1024 // 50 MiB resumable upload chunks

	// speedEMA is the smoothing factor for the exponential-moving-average
	// speed estimate. It keeps the displayed speed meaningful even though
	// progress is reported in lumpy per-chunk jumps.
	speedEMA = 0.25
)

var errCancelled = errors.New("cancelled by user")

// Client is the embedded Google Drive transfer client.
type Client struct {
	auth        *Auth
	concurrency int
}

// New constructs a Drive Client. concurrency bounds parallel per-file transfers.
func New(cfg Config, concurrency int) (*Client, error) {
	a, err := NewAuth(cfg)
	if err != nil {
		return nil, err
	}
	if concurrency < 1 {
		concurrency = 1
	}
	return &Client{auth: a, concurrency: concurrency}, nil
}

// NewTransfer creates a Transfer status of the given type and gid.
func (c *Client) NewTransfer(typ status.StatusType, gid string) *Transfer {
	t := &Transfer{
		client:    c,
		typ:       typ,
		gid:       gid,
		sem:       make(chan struct{}, c.concurrency),
		startTime: time.Now(),
	}
	return t
}

// Transfer tracks one Drive operation (upload/download/clone) and implements
// status.Status.
type Transfer struct {
	client    *Client
	typ       status.StatusType
	gid       string
	startTime time.Time

	nameMu sync.RWMutex
	name   string
	path   string

	completed     atomic.Int64
	total         atomic.Int64
	speed         atomic.Int64
	index         atomic.Int64
	cancelled     atomic.Bool
	completedFlag atomic.Bool
	failedFlag    atomic.Bool

	sem chan struct{}
	wg  sync.WaitGroup

	errMu  sync.Mutex
	err    error
	fileMu sync.Mutex
	fileID string

	speedMu      sync.Mutex
	speedF       float64
	lastProgress time.Time
}

// --- status.Status implementation ---

func (t *Transfer) Name() string {
	t.nameMu.RLock()
	defer t.nameMu.RUnlock()
	if t.name == "" {
		return "getting metadata"
	}
	return t.name
}

func (t *Transfer) setName(n string) { t.nameMu.Lock(); t.name = n; t.nameMu.Unlock() }

func (t *Transfer) CompletedLength() int64 { return t.completed.Load() }
func (t *Transfer) TotalLength() int64     { return t.total.Load() }
func (t *Transfer) Speed() int64           { return t.speed.Load() }
func (t *Transfer) GID() string            { return t.gid }

func (t *Transfer) Path() string {
	t.nameMu.RLock()
	defer t.nameMu.RUnlock()
	return t.path
}

func (t *Transfer) Percentage() float32 {
	total := t.total.Load()
	if total == 0 {
		return 0
	}
	return float32(t.completed.Load()*100) / float32(total)
}

func (t *Transfer) ETA() *time.Duration {
	left := t.total.Load() - t.completed.Load()
	if left < 0 {
		left = 0
	}
	eta := util.CalculateETA(left, t.speed.Load())
	return &eta
}

func (t *Transfer) StatusType() status.StatusType {
	if t.cancelled.Load() {
		return status.Canceled
	}
	if t.failedFlag.Load() {
		return status.Failed
	}
	return t.typ
}

func (t *Transfer) Index() int     { return int(t.index.Load()) }
func (t *Transfer) SetIndex(i int) { t.index.Store(int64(i)) }

func (t *Transfer) Cancel() bool {
	t.cancelled.Store(true)
	return true
}

// FileID returns the resulting Drive file id (set on completion).
func (t *Transfer) FileID() string {
	t.fileMu.Lock()
	defer t.fileMu.Unlock()
	return t.fileID
}

func (t *Transfer) setFileID(id string) {
	t.fileMu.Lock()
	t.fileID = id
	t.fileMu.Unlock()
}

func (t *Transfer) setErr(err error) {
	t.errMu.Lock()
	t.err = err
	t.errMu.Unlock()
	t.failedFlag.Store(true)
}

// --- progress tracking ---

// addCompleted records n bytes of transfer progress and updates the smoothed
// speed estimate. Progress is often reported in large, lumpy jumps (e.g. one
// 50 MiB upload chunk at a time), so raw 1s samples read 0 between jumps;
// instead the speed is a time-weighted exponential moving average computed
// from the progress events themselves.
func (t *Transfer) addCompleted(n int64) {
	if n == 0 {
		return
	}
	t.completed.Add(n)
	now := time.Now()
	t.speedMu.Lock()
	defer t.speedMu.Unlock()
	if n < 0 {
		// Rollback (retry of a failed chunk): re-anchor so the next progress
		// event is measured against its own duration, not the retry slack.
		t.lastProgress = now
		return
	}
	if !t.lastProgress.IsZero() {
		dt := now.Sub(t.lastProgress).Seconds()
		if dt > 0 {
			inst := float64(n) / dt
			if t.speedF == 0 {
				t.speedF = inst
			} else {
				t.speedF += (inst - t.speedF) * speedEMA
			}
			t.speed.Store(int64(t.speedF))
		}
	}
	t.lastProgress = now
}

// metricType maps the transfer's status type to a metrics label.
func (t *Transfer) metricType() string {
	switch t.typ {
	case status.Uploading:
		return "upload"
	case status.Downloading:
		return "download"
	case status.Cloning:
		return "clone"
	default:
		return "unknown"
	}
}

// recordDone emits the Prometheus result + byte counters for a finished transfer.
func (t *Transfer) recordDone(err error) {
	typ := t.metricType()
	switch {
	case err == nil:
		metrics.Transfers.WithLabelValues(typ, metrics.ResultComplete).Inc()
		metrics.TransferBytes.WithLabelValues(typ).Add(float64(t.completed.Load()))
	case t.cancelled.Load() || errors.Is(err, errCancelled):
		metrics.Transfers.WithLabelValues(typ, metrics.ResultCancelled).Inc()
	default:
		metrics.Transfers.WithLabelValues(typ, metrics.ResultError).Inc()
	}
}
