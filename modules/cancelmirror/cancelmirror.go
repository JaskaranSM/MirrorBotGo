package cancelmirror

import (
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"

	"github.com/PaulSonOfLars/gotgbot"
	"github.com/PaulSonOfLars/gotgbot/ext"
	"github.com/PaulSonOfLars/gotgbot/handlers"
	"go.uber.org/zap"
)

func CancelMirrorHandler(b ext.Bot, u *gotgbot.Update) error {
	if !utils.IsUserSudo(u.EffectiveUser.Id) {
		return nil
	}
	message := u.EffectiveMessage
	if message.ReplyToMessage == nil {
		engine.SendMessage(b, "Reply to mirror start message to cancel it.", message)
		return nil
	}
	dl := engine.GetMirrorByUid(message.ReplyToMessage.MessageId)
	if dl == nil {
		engine.SendMessage(b, "Mirror doesnt exists.", message)
		return nil
	}
	if dl.GetStatusType() != engine.MirrorStatusDownloading {
		engine.SendMessage(b, "Do not cancel uploads bruh.", message)
		return nil
	}
	dl.CancelMirror()
	return nil
}

func LoadCancelMirrorHandler(updater *gotgbot.Updater, l *zap.SugaredLogger) {
	defer l.Info("CancelMirror Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("cancel", CancelMirrorHandler))
}
