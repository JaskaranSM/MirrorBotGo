package mirror

import (
	"MirrorBotGo/mirrorManager"
	"MirrorBotGo/utils"

	"github.com/PaulSonOfLars/gotgbot"
	"github.com/PaulSonOfLars/gotgbot/ext"
	"github.com/PaulSonOfLars/gotgbot/handlers"
	"go.uber.org/zap"
)

func Mirror(b ext.Bot, u *gotgbot.Update, isTar bool) error {
	message := u.EffectiveMessage
	link := utils.ParseMessageArgs(message.Text)
	listener := mirrorManager.NewMirrorListener(b, u)
	mirrorManager.NewTorrentDownload(link, &listener)
	mirrorManager.SendStatusMessage(b, message)
	if !mirrorManager.Spinner.IsRunning() {
		mirrorManager.Spinner.Start(b)
	}
	return nil
}

func MirrorHandler(b ext.Bot, u *gotgbot.Update) error {
	if !utils.IsUserOwner(u.EffectiveUser.Id) {
		return nil
	}
	return Mirror(b, u, false)
}

func LoadMirrorHandlers(updater *gotgbot.Updater, l *zap.SugaredLogger) {
	defer l.Info("Mirror Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("mirror", MirrorHandler))
}
