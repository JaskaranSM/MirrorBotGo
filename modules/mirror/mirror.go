package mirror

import (
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"

	"github.com/PaulSonOfLars/gotgbot"
	"github.com/PaulSonOfLars/gotgbot/ext"
	"github.com/PaulSonOfLars/gotgbot/handlers"
	"go.uber.org/zap"
)

func Mirror(b ext.Bot, u *gotgbot.Update, isTar bool) error {
	message := u.EffectiveMessage
	link := utils.ParseMessageArgs(message.Text)
	listener := engine.NewMirrorListener(b, u)
	isTorrent, err := utils.IsTorrentLink(link)
	if err != nil {
		engine.SendMessage(b, err.Error(), message)
		return nil
	}
	if isTorrent {
		engine.NewTorrentDownload(link, &listener)
	} else {
		engine.NewHttpDownload(link, &listener)
	}
	engine.SendStatusMessage(b, message)
	if !engine.Spinner.IsRunning() {
		engine.Spinner.Start(b)
	}
	return nil
}

func MirrorHandler(b ext.Bot, u *gotgbot.Update) error {
	if !utils.IsUserSudo(u.EffectiveUser.Id) {
		return nil
	}
	return Mirror(b, u, false)
}

func LoadMirrorHandlers(updater *gotgbot.Updater, l *zap.SugaredLogger) {
	defer l.Info("Mirror Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("mirror", MirrorHandler))
}
