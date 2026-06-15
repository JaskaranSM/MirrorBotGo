// Package config loads all runtime configuration from environment variables,
// so the bot is trivial to configure via docker-compose.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the fully-resolved runtime configuration.
type Config struct {
	// Telegram
	BotToken  string
	TGAppID   int
	TGAppHash string

	// Authorization
	OwnerID         int64
	SudoUsers       []int64
	AuthorizedChats []int64

	// Local storage
	DownloadDir string

	// Google Drive
	UseSA               bool
	SADir               string
	GDriveCredentials   string
	GDriveTokenFile     string
	GDriveParentID      string
	IsTeamDrive         bool
	IndexURL            string
	GDriveConcurrency   int

	// Database
	DBDriver string // "sqlite" | "postgres"
	DBDSN    string

	// Status / UX
	StatusUpdateInterval        time.Duration
	StatusMessagesPerPage       int
	AutoDeleteTimeout           int // seconds; 0 = disabled
	StatusMessageAutoDeleteTime int // seconds; 0 = disabled
	SpamFilterMessages          int
	SpamFilterDuration          int // seconds

	// Torrent engine
	TorrentListenPort                 int
	TorrentPeerIDPrefix               string
	TorrentMaxUploadRate              string
	TorrentEstablishedConnsPerTorrent int
	TorrentSeed                       bool
	TorrentUseTrackerList             bool
	TorrentTrackerListURL             string

	// Health
	HealthAddr string
}

// Load reads configuration from the environment and validates required fields.
func Load() (*Config, error) {
	c := &Config{
		BotToken:  getEnv("BOT_TOKEN", ""),
		TGAppID:   getEnvInt("TG_APP_ID", 0),
		TGAppHash: getEnv("TG_APP_HASH", ""),

		OwnerID:         getEnvInt64("OWNER_ID", 0),
		SudoUsers:       getEnvCSVInt64("SUDO_USERS"),
		AuthorizedChats: getEnvCSVInt64("AUTHORIZED_CHATS"),

		DownloadDir: getEnv("DOWNLOAD_DIR", "./downloads"),

		UseSA:             getEnvBool("USE_SA", true),
		SADir:             getEnv("SA_DIR", "accounts"),
		GDriveCredentials: getEnv("GDRIVE_CREDENTIALS_FILE", "credentials.json"),
		GDriveTokenFile:   getEnv("GDRIVE_TOKEN_FILE", "token.json"),
		GDriveParentID:    getEnv("GDRIVE_PARENT_ID", ""),
		IsTeamDrive:       getEnvBool("IS_TEAM_DRIVE", true),
		IndexURL:          getEnv("INDEX_URL", ""),
		GDriveConcurrency: getEnvInt("GDRIVE_CONCURRENCY", 10),

		DBDriver: strings.ToLower(getEnv("DB_DRIVER", "sqlite")),
		DBDSN:    getEnv("DB_DSN", "file:data/mirrorbot.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"),

		StatusUpdateInterval:        time.Duration(getEnvInt("STATUS_UPDATE_INTERVAL", 5)) * time.Second,
		StatusMessagesPerPage:       getEnvInt("STATUS_MESSAGES_PER_PAGE", 5),
		AutoDeleteTimeout:           getEnvInt("AUTO_DELETE_TIMEOUT", 0),
		StatusMessageAutoDeleteTime: getEnvInt("STATUS_MESSAGE_AUTO_DELETE_TIME", 0),
		SpamFilterMessages:          getEnvInt("SPAM_FILTER_MESSAGES", 1),
		SpamFilterDuration:          getEnvInt("SPAM_FILTER_DURATION", 10),

		TorrentListenPort:                 getEnvInt("TORRENT_LISTEN_PORT", 42069),
		TorrentPeerIDPrefix:               getEnv("TORRENT_PEER_ID_PREFIX", "-qB5000-"),
		TorrentMaxUploadRate:              getEnv("TORRENT_MAX_UPLOAD_RATE", "100KiB"),
		TorrentEstablishedConnsPerTorrent: getEnvInt("TORRENT_ESTABLISHED_CONNS_PER_TORRENT", 100),
		TorrentSeed:                       getEnvBool("TORRENT_SEED", false),
		TorrentUseTrackerList:             getEnvBool("TORRENT_USE_TRACKER_LIST", false),
		TorrentTrackerListURL:             getEnv("TORRENT_TRACKER_LIST_URL", "https://raw.githubusercontent.com/ngosang/trackerslist/master/trackers_best.txt"),

		HealthAddr: getEnv("HEALTH_ADDR", ":7870"),
	}

	if c.StatusMessagesPerPage <= 0 {
		c.StatusMessagesPerPage = 5
	}
	if c.GDriveConcurrency <= 0 {
		c.GDriveConcurrency = 10
	}

	return c, c.validate()
}

func (c *Config) validate() error {
	var missing []string
	if c.BotToken == "" {
		missing = append(missing, "BOT_TOKEN")
	}
	if c.TGAppID == 0 {
		missing = append(missing, "TG_APP_ID")
	}
	if c.TGAppHash == "" {
		missing = append(missing, "TG_APP_HASH")
	}
	if c.DBDriver != "sqlite" && c.DBDriver != "postgres" {
		return fmt.Errorf("DB_DRIVER must be 'sqlite' or 'postgres', got %q", c.DBDriver)
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	return nil
}

// IsOwner reports whether the user id is the owner or a sudo user.
func (c *Config) IsOwner(userID int64) bool {
	if userID == c.OwnerID {
		return true
	}
	for _, id := range c.SudoUsers {
		if id == userID {
			return true
		}
	}
	return false
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}

func getEnvInt64(key string, def int64) int64 {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			return n
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			return b
		}
	}
	return def
}

func getEnvCSVInt64(key string) []int64 {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return nil
	}
	var out []int64
	for _, part := range strings.Split(v, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if n, err := strconv.ParseInt(part, 10, 64); err == nil {
			out = append(out, n)
		}
	}
	return out
}
