// Package metrics defines the Prometheus collectors used across the bot. All
// collectors register with the default registry, so the Go runtime and process
// collectors (registered by client_golang) are exported alongside them at
// /metrics.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Result label values.
const (
	ResultComplete  = "complete"
	ResultError     = "error"
	ResultCancelled = "cancelled"
	ResultOK        = "ok"
)

// Source label values (download sources / mirror kinds).
const (
	SourceTorrent = "torrent"
	SourceHTTP    = "http"
	SourceTGFile  = "tgfile"
	SourceBulkTG  = "bulktg"
	SourceGDrive  = "gdrive"
)

var (
	// Commands counts Telegram commands received, labelled by command name.
	Commands = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mirrorbot_commands_total",
		Help: "Telegram commands received, by command.",
	}, []string{"command"})

	// TelegramOps counts transport operations by op and result.
	TelegramOps = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mirrorbot_telegram_ops_total",
		Help: "Telegram transport operations, by op (send/edit/delete/document/download/answer_callback) and result.",
	}, []string{"op", "result"})

	// TelegramFloodWaits counts FLOOD_WAIT responses encountered.
	TelegramFloodWaits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mirrorbot_telegram_floodwaits_total",
		Help: "Telegram FLOOD_WAIT responses encountered.",
	})

	// TelegramRetries counts transport retry attempts.
	TelegramRetries = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mirrorbot_telegram_retries_total",
		Help: "Telegram transport retry attempts.",
	})

	// MirrorsStarted counts mirrors started, by source.
	MirrorsStarted = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mirrorbot_mirrors_started_total",
		Help: "Mirror downloads started, by source.",
	}, []string{"source"})

	// MirrorsFinished counts mirror download outcomes, by source and result.
	MirrorsFinished = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mirrorbot_mirrors_finished_total",
		Help: "Mirror download outcomes, by source and result.",
	}, []string{"source", "result"})

	// DownloadBytes counts bytes downloaded, by source.
	DownloadBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mirrorbot_download_bytes_total",
		Help: "Bytes downloaded, by source.",
	}, []string{"source"})

	// DownloadDuration observes download durations, by source.
	DownloadDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "mirrorbot_download_duration_seconds",
		Help:    "Download duration in seconds, by source.",
		Buckets: prometheus.ExponentialBuckets(1, 2, 13), // ~1s .. ~1h
	}, []string{"source"})

	// Transfers counts Google Drive transfers, by type (upload/download/clone) and result.
	Transfers = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mirrorbot_gdrive_transfers_total",
		Help: "Google Drive transfers, by type and result.",
	}, []string{"type", "result"})

	// TransferBytes counts Google Drive transfer bytes, by type.
	TransferBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mirrorbot_gdrive_transfer_bytes_total",
		Help: "Google Drive transfer bytes, by type.",
	}, []string{"type"})

	// DriveSARotations counts service-account rotations.
	DriveSARotations = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mirrorbot_gdrive_sa_rotations_total",
		Help: "Google Drive service-account rotations (quota/rate-limit fallbacks).",
	})

	// ArchiveOps counts archive operations, by op (tar/untar) and result.
	ArchiveOps = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mirrorbot_archive_ops_total",
		Help: "Archive operations, by op and result.",
	}, []string{"op", "result"})

	// ArchiveBytes counts archive bytes processed, by op.
	ArchiveBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mirrorbot_archive_bytes_total",
		Help: "Archive bytes processed, by op.",
	}, []string{"op"})

	// BulkFilesQueued counts files queued into bulk sessions.
	BulkFilesQueued = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mirrorbot_bulk_files_queued_total",
		Help: "Files queued into bulk Telegram sessions.",
	})

	// Panics counts recovered panics, by location.
	Panics = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mirrorbot_panics_total",
		Help: "Recovered panics, by location.",
	}, []string{"where"})
)

// RegisterGauge registers a gauge whose value is read live from fn on scrape.
// Used for state-derived gauges (active mirrors, seeding, bulk sessions).
func RegisterGauge(name, help string, fn func() float64) {
	promauto.NewGaugeFunc(prometheus.GaugeOpts{Name: name, Help: help}, fn)
}

// Handler returns the HTTP handler exposing the default registry.
func Handler() http.Handler { return promhttp.Handler() }
