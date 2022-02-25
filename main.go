// MirrorBotGo project main.go
package main

import (
	"MirrorBotGo/db"
	"MirrorBotGo/engine"
	"MirrorBotGo/modules/authorization"
	"MirrorBotGo/modules/botlog"
	"MirrorBotGo/modules/cancelmirror"
	"MirrorBotGo/modules/clone"
	"MirrorBotGo/modules/goexec"
	"MirrorBotGo/modules/list"
	"MirrorBotGo/modules/mirror"
	"MirrorBotGo/modules/mirrorstatus"
	"MirrorBotGo/modules/ping"
	"MirrorBotGo/modules/start"
	"MirrorBotGo/modules/stats"
	"MirrorBotGo/utils"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func RegisterAllHandlers(updater *ext.Updater, l *zap.SugaredLogger) {
	start.LoadStartHandler(updater, l)
	mirror.LoadMirrorHandlers(updater, l)
	mirrorstatus.LoadMirrorStatusHandler(updater, l)
	cancelmirror.LoadCancelMirrorHandler(updater, l)
	list.LoadListHandler(updater, l)
	goexec.LoadExecHandler(updater, l)
	authorization.LoadAuthorizationHandlers(updater, l)
	stats.LoadStatsHandler(updater, l)
	ping.LoadPingHandler(updater, l)
	clone.LoadCloneHandler(updater, l)
	botlog.LoadLogHandler(updater, l)
}

func main() {
	engine.InitLog()
	handle, err := engine.GetLogFileHandle()
	if err != nil {
		log.Println("Cannot open log file: ", err)
	}
	log.SetOutput(io.MultiWriter(os.Stdout, handle))
	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeLevel = zapcore.CapitalLevelEncoder
	cfg.EncodeTime = zapcore.RFC3339TimeEncoder
	handleSync, _, err := zap.Open(engine.LogFile)
	if err != nil {
		log.Println("Cannot open log file for zap: ", err)
	}
	core1 := zapcore.NewCore(zapcore.NewConsoleEncoder(cfg), os.Stdout, zap.InfoLevel)
	core2 := zapcore.NewCore(zapcore.NewConsoleEncoder(cfg), handleSync, zap.InfoLevel)
	logger := zap.New(zapcore.NewTee(core1, core2))
	defer logger.Sync() // flushes buffer, if any
	l := logger.Sugar()
	token := utils.GetBotToken()
	l.Info("Starting Bot.")
	l.Info("token: ", token)
	b, err := gotgbot.NewBot(token, &gotgbot.BotOpts{
		Client:      http.Client{},
		GetTimeout:  gotgbot.DefaultGetTimeout,
		PostTimeout: gotgbot.DefaultPostTimeout,
	})
	if err != nil {
		l.Fatal(err)
	}
	updater := ext.NewUpdater(&ext.UpdaterOpts{})
	l.Info("Starting updater")
	RegisterAllHandlers(&updater, l)
	db.Init()
	engine.Init()
	go utils.ExitCleanup()
	err = updater.StartPolling(b, nil)
	if err != nil {
		l.Fatalf("Error occurred at start of polling :  %s", err.Error())
		return
	}
	l.Info("Started Updater.")
	updater.Idle()
}
