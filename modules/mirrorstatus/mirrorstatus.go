package mirrorstatus

import (
	"MirrorBotGo/mirrorManager"
	"MirrorBotGo/utils"

	"github.com/PaulSonOfLars/gotgbot"
	"github.com/PaulSonOfLars/gotgbot/ext"
	"github.com/PaulSonOfLars/gotgbot/handlers"
	"go.uber.org/zap"
)

func MirrorStatusHandler(b ext.Bot, u *gotgbot.Update) error {
	message := u.EffectiveMessage
	if !utils.IsUserOwner(u.EffectiveUser.Id) {
		return nil
	}
	if mirrorManager.GetAllMirrorsCount() == 0 {
		mirrorManager.SendMessage(b, "No Active Downloads.", message)
		return nil
	}
	mirrorManager.SendStatusMessage(b, message)
	mirrorManager.DeleteMessage(b, message)
	return nil
}

func LoadMirrorStatusHandler(updater *gotgbot.Updater, l *zap.SugaredLogger) {
	defer l.Info("MirrorStatus Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("status", MirrorStatusHandler))
}
