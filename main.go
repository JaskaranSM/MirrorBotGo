// MirrorBotGo project main.go
package main

import (
	"MirrorBotGo/mirrorManager"
	"MirrorBotGo/modules/mirror"
	"MirrorBotGo/modules/mirrorstatus"
	"MirrorBotGo/modules/start"
	"MirrorBotGo/utils"
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
}

func main() {
	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeLevel = zapcore.CapitalLevelEncoder
	cfg.EncodeTime = zapcore.RFC3339TimeEncoder

	logger := zap.New(zapcore.NewCore(zapcore.NewConsoleEncoder(cfg), os.Stdout, zap.InfoLevel))
	defer logger.Sync() // flushes buffer, if any
	l := logger.Sugar()
	token := utils.GetBotToken()
	l.Info("Starting Bot.")
	l.Info("token: ", token)
	updater, err := gotgbot.NewUpdater(logger, token)
	updater.Bot.Requester = ext.BaseRequester{Client: http.Client{Timeout: time.Second * 45}}
	if err != nil {
		l.Fatalw("failed to start updater", zap.Error(err))
	}
	RegisterAllHandlers(updater, l)
	mirrorManager.Init()
	mirrorManager.Clean()
	updater.StartPolling()
	l.Info("Started Updater.")
	updater.Idle()
}
