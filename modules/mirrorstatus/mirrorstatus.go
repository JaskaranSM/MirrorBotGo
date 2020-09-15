package mirrorstatus

import (
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"

	"github.com/PaulSonOfLars/gotgbot"
	"github.com/PaulSonOfLars/gotgbot/ext"
	"github.com/PaulSonOfLars/gotgbot/handlers"
	"go.uber.org/zap"
)

func MirrorStatusHandler(b ext.Bot, u *gotgbot.Update) error {
	message := u.EffectiveMessage
	if !utils.IsUserSudo(u.EffectiveUser.Id) {
		return nil
	}
	if engine.GetAllMirrorsCount() == 0 {
		out, _ := engine.SendMessage(b, "No Active Downloads.", message)
		engine.AutoDeleteMessages(b, utils.GetAutoDeleteTimeOut(), out, message)
		return nil
	}
	engine.SendStatusMessage(b, message)
	engine.DeleteMessage(b, message)
	return nil
}

func LoadMirrorStatusHandler(updater *gotgbot.Updater, l *zap.SugaredLogger) {
	defer l.Info("MirrorStatus Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("status", MirrorStatusHandler))
}
