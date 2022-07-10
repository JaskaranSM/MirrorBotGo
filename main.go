// MirrorBotGo project main.go
package main

import (
	"MirrorBotGo/db"
	"MirrorBotGo/engine"
	"MirrorBotGo/modules/authorization"
	"MirrorBotGo/modules/botlog"
	"MirrorBotGo/modules/cancelmirror"
	"MirrorBotGo/modules/clone"
	"MirrorBotGo/modules/configuration"
	"MirrorBotGo/modules/goexec"
	"MirrorBotGo/modules/list"
	"MirrorBotGo/modules/mirror"
	"MirrorBotGo/modules/mirrorstatus"
	"MirrorBotGo/modules/ping"
	"MirrorBotGo/modules/shell"
	"MirrorBotGo/modules/start"
	"MirrorBotGo/modules/stats"
	"MirrorBotGo/utils"
	"net/http"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"go.uber.org/zap"
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
	shell.LoadShellHandlers(updater, l)
	configuration.LoadConfigurationHandlers(updater, l)
}

func main() {
	l := engine.GetLogger()
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
	updater := ext.NewUpdater(&ext.UpdaterOpts{
		DispatcherOpts: ext.DispatcherOpts{
			MaxRoutines: -1,
		},
	})
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
