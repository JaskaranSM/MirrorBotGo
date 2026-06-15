package gdrive

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"mirrorbot/internal/status"
	"mirrorbot/internal/util"
)

const (
	maxRetries = 5
	chunkSize  = 50 * 1024 * 1024 // 50 MiB resumable upload chunks
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

	completed atomic.Int64
	total     atomic.Int64
	speed     atomic.Int64
	index     atomic.Int64
	cancelled atomic.Bool
	completedFlag atomic.Bool
	failedFlag    atomic.Bool

	sem chan struct{}
	wg  sync.WaitGroup

	errMu  sync.Mutex
	err    error
	fileMu sync.Mutex
	fileID string

	observerOnce sync.Once
	observerStop chan struct{}
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

// --- speed observer ---

func (t *Transfer) startObserver() {
	t.observerOnce.Do(func() {
		t.observerStop = make(chan struct{})
		go func() {
			last := t.completed.Load()
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-t.observerStop:
					return
				case <-ticker.C:
					now := t.completed.Load()
					t.speed.Store(now - last)
					last = now
				}
			}
		}()
	})
}

func (t *Transfer) stopObserver() {
	if t.observerStop != nil {
		close(t.observerStop)
	}
}

func (t *Transfer) addCompleted(n int64) { t.completed.Add(n) }
