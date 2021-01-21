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
	"time"

	"github.com/PaulSonOfLars/gotgbot"
	"github.com/PaulSonOfLars/gotgbot/ext"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func RegisterAllHandlers(updater *gotgbot.Updater, l *zap.SugaredLogger) {
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
	updater, err := gotgbot.NewUpdater(logger, token)
	l.Info("Got Updater")
	updater.UpdateGetter = ext.BaseRequester{
		Client: http.Client{
			Transport:     nil,
			CheckRedirect: nil,
			Jar:           nil,
			Timeout:       time.Second * 65,
		},
		ApiUrl: ext.ApiUrl,
	}
	updater.Bot.Requester = ext.BaseRequester{Client: http.Client{Timeout: time.Second * 65}}
	if err != nil {
		l.Fatalw("failed to start updater", zap.Error(err))
	}
	l.Info("Starting updater")
	RegisterAllHandlers(updater, l)
	db.Init()
	engine.Init()
	go utils.ExitCleanup()
	updater.StartPolling()
	l.Info("Started Updater.")
	updater.Idle()
}
