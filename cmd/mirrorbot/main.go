// Command mirrorbot is a single-binary Telegram mirror bot: it clones torrents,
// HTTP links, Google Drive links, and Telegram files into Google Drive.
package main

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"mirrorbot/internal/app"
	"mirrorbot/internal/config"
	"mirrorbot/internal/health"
	"mirrorbot/internal/store"
	"mirrorbot/internal/tgbot"
	"mirrorbot/internal/util"
)

const logFileName = "log.txt"

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	logFile := setupLogging()

	if err := os.MkdirAll(cfg.DownloadDir, 0o755); err != nil {
		log.Fatalf("create download dir: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(cfg.DBDriver, cfg.DBDSN)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(ctx); err != nil {
		log.Fatalf("migrate store: %v", err)
	}

	bot, err := tgbot.New(cfg.BotToken, cfg.TGAppID, cfg.TGAppHash, cfg.SpamFilterMessages, cfg.SpamFilterDuration, os.Getenv("TG_DEBUG") == "1")
	if err != nil {
		log.Fatalf("create bot: %v", err)
	}

	application, err := app.New(ctx, cfg, bot, st, logFile)
	if err != nil {
		log.Fatalf("create app: %v", err)
	}
	application.Register()

	h := health.New()
	h.Start(cfg.HealthAddr)

	log.Println("starting bot")
	runErr := application.Run(ctx)

	// Shutdown cleanup: cancel mirrors and wipe the scratch directory.
	application.CancelAll()
	application.Close()
	if err := util.RemoveByPath(cfg.DownloadDir); err != nil {
		log.Printf("cleanup download dir: %v", err)
	}

	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		log.Fatalf("bot stopped: %v", runErr)
	}
	log.Println("bot stopped cleanly")
}

func setupLogging() string {
	f, err := os.OpenFile(logFileName, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("could not open log file: %v", err)
		return ""
	}
	log.SetOutput(io.MultiWriter(os.Stdout, f))
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	return logFileName
}
