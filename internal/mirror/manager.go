// Package mirror holds the mirror state manager, the lifecycle listeners that
// drive a download -> (archive) -> upload pipeline, and the Telegram status
// display loop.
package mirror

import (
	"sort"
	"sync"

	"mirrorbot/internal/status"
)

// Manager tracks all active, canceled, and seeding mirrors keyed by the status
// message id (UID). It assigns each unit a stable, monotonically increasing
// display index. Safe for concurrent use.
type Manager struct {
	mu       sync.RWMutex
	all      map[int64]status.Status
	canceled map[int64]status.Status
	seeding  map[int64]status.Status
	index    int
}

// NewManager returns an empty Manager.
func NewManager() *Manager {
	return &Manager{
		all:      make(map[int64]status.Status),
		canceled: make(map[int64]status.Status),
		seeding:  make(map[int64]status.Status),
	}
}

// NextIndex returns the next display index. Callers set it on a status before
// Set so indexes can be inherited across lifecycle transitions.
func (m *Manager) NextIndex() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.index++
	return m.index
}

// Set stores s as the active status for uid (the status' index is used as-is).
func (m *Manager) Set(uid int64, s status.Status) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.all[uid] = s
}

// Remove deletes the active status for uid.
func (m *Manager) Remove(uid int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.all, uid)
}

// MoveToCancel moves the active status for uid into the canceled set.
func (m *Manager) MoveToCancel(uid int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.all[uid]; ok {
		m.canceled[uid] = s
		delete(m.all, uid)
	}
}

// MoveToSeeding moves the active status for uid into the seeding set.
func (m *Manager) MoveToSeeding(uid int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.all[uid]; ok {
		m.seeding[uid] = s
		delete(m.all, uid)
	}
}

// RemoveSeeding deletes a seeding entry (e.g. when seeding is stopped).
func (m *Manager) RemoveSeeding(uid int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.seeding, uid)
}

// GetSeeding returns the seeding status for uid, or nil.
func (m *Manager) GetSeeding(uid int64) status.Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.seeding[uid]
}

// AllSeeding returns seeding statuses (unordered).
func (m *Manager) AllSeeding() []status.Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]status.Status, 0, len(m.seeding))
	for _, s := range m.seeding {
		out = append(out, s)
	}
	return out
}

// Get returns the active status for uid, or nil.
func (m *Manager) Get(uid int64) status.Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.all[uid]
}

// ByGID returns the active or seeding status with the given gid, or nil.
func (m *Manager) ByGID(gid string) status.Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.all {
		if s.GID() == gid {
			return s
		}
	}
	for _, s := range m.seeding {
		if s.GID() == gid {
			return s
		}
	}
	return nil
}

// ByIndex returns the active status with the given display index, or nil.
func (m *Manager) ByIndex(idx int) status.Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.all {
		if s.Index() == idx {
			return s
		}
	}
	return nil
}

// All returns active statuses sorted by index ascending.
func (m *Manager) All() []status.Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]status.Status, 0, len(m.all))
	for _, s := range m.all {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Index() < out[j].Index() })
	return out
}

// Count returns the number of active mirrors.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.all)
}

// SeedingCount returns the number of seeding mirrors.
func (m *Manager) SeedingCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.seeding)
}

// Chunked splits the index-sorted active mirror list into pages of chunkSize.
// It always returns at least one (possibly empty) page.
func (m *Manager) Chunked(chunkSize int) [][]status.Status {
	items := m.All()
	if chunkSize < 1 {
		chunkSize = 1
	}
	var chunks [][]status.Status
	for chunkSize < len(items) {
		chunks = append(chunks, items[0:chunkSize])
		items = items[chunkSize:]
	}
	return append(chunks, items)
}
