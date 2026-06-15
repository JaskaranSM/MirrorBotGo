// Package status defines the unified progress interface every trackable unit
// of work implements, and the renderer that turns a set of them into the
// Telegram status message. The render output is byte-compatible with the
// original MirrorBotGo.
package status

import "time"

// StatusType is the human-readable state label shown in the status message.
type StatusType string

// Status type constants. The string values are shown verbatim to users and
// must not change.
const (
	Downloading  StatusType = "Downloading"
	Uploading    StatusType = "Uploading"
	Archiving    StatusType = "Archiving"
	UnArchiving  StatusType = "UnArchiving"
	Initializing StatusType = "Initializing"
	Cloning      StatusType = "Cloning"
	Seeding      StatusType = "Seeding"
	Waiting      StatusType = "Queued"
	Failed       StatusType = "Failed"
	Canceled     StatusType = "Cancelled"
	UploadQueued StatusType = "Queued for upload"
)

// Status is implemented by every download, upload, clone, and archive unit.
// The renderer and manager depend only on this interface.
type Status interface {
	Name() string
	CompletedLength() int64
	TotalLength() int64
	Speed() int64
	// ETA returns the estimated time remaining, or nil if unknown.
	ETA() *time.Duration
	GID() string
	// Path is the local filesystem path or parent dir associated with the unit.
	Path() string
	Percentage() float32
	StatusType() StatusType
	// Index is the stable display ordering number (the "I:" field).
	Index() int
	SetIndex(int)
	// Cancel requests cancellation; returns true if accepted.
	Cancel() bool
}

// Listener receives download lifecycle callbacks from a source. The mirror
// package's MirrorListener implements it; sources depend only on this interface.
type Listener interface {
	// OnDownloadStart is called once the source has begun and a status exists.
	OnDownloadStart()
	// OnDownloadComplete is called when the local download has finished.
	OnDownloadComplete()
	// OnDownloadError is called on failure or cancellation.
	OnDownloadError(err error)
}

// TorrentInfo is an optional interface that torrent statuses implement to
// surface the extra fields shown in the status line (peers, seeders, pieces).
type TorrentInfo interface {
	PiecesCompleted() int
	PiecesTotal() int
	Peers() int
	Seeders() int
}
