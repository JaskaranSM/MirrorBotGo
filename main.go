// MirrorBotGo project main.go
package main

import (
	"MirrorBotGo/engine"
	"MirrorBotGo/modules/authorization"
	"MirrorBotGo/modules/botlog"
	"MirrorBotGo/modules/cancelmirror"
	"MirrorBotGo/modules/clone"
	"MirrorBotGo/modules/configuration"
	"MirrorBotGo/modules/list"
	"MirrorBotGo/modules/mirror"
	"MirrorBotGo/modules/mirrorstatus"
	"MirrorBotGo/modules/ping"
	"MirrorBotGo/modules/shell"
	"MirrorBotGo/modules/start"
	"MirrorBotGo/modules/stats"
	"MirrorBotGo/utils"
	"net/http"
	"os"
	"os/signal"

	_ "net/http/pprof"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"go.uber.org/zap"
)

func ExitCleanup() {
	killSignal := make(chan os.Signal, 1)
	signal.Notify(killSignal, os.Interrupt)
	<-killSignal
	engine.CancelAllMirrors()
	engine.L().Info("Exit Cleanup")
	err := utils.RemoveByPath(utils.GetDownloadDir())
	if err != nil {
		engine.L().Errorf("Error while removing dir: %s : %v\n", utils.GetDownloadDir(), err)
	}
	os.Exit(1)
}

func RegisterAllHandlers(updater *ext.Updater, l *zap.SugaredLogger) {
	start.LoadStartHandler(updater, l)
	mirror.LoadMirrorHandlers(updater, l)
	mirrorstatus.LoadMirrorStatusHandler(updater, l)
	cancelmirror.LoadCancelMirrorHandler(updater, l)
	list.LoadListHandler(updater, l)
	authorization.LoadAuthorizationHandlers(updater, l)
	stats.LoadStatsHandler(updater, l)
	ping.LoadPingHandler(updater, l)
	clone.LoadCloneHandler(updater, l)
	botlog.LoadLogHandler(updater, l)
	shell.LoadShellHandlers(updater, l)
	configuration.LoadConfigurationHandlers(updater, l)
}

func main() {
	router := engine.NewHealthRouter()
	router.StartWebServer(utils.GetHealthCheckRouterURL())
	l := engine.GetLogger()
	token := utils.GetBotToken()
	l.Info("Starting Bot.")
	b, err := gotgbot.NewBot(token, &gotgbot.BotOpts{
		Client: http.Client{},
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
	go ExitCleanup()
	err = updater.StartPolling(b, nil)
	if err != nil {
		l.Fatalf("Error occurred at start of polling :  %s", err.Error())
		return
	}
	l.Info("Started Updater.")
	updater.Idle()
}
