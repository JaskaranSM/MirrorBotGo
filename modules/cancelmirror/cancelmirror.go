package cancelmirror

import (
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"
	"fmt"

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

func CancelAllMirrorsHandler(b ext.Bot, u *gotgbot.Update) error {
	if !utils.IsUserSudo(u.EffectiveUser.Id) {
		return nil
	}
	count := 0
	message := u.EffectiveMessage
	if engine.GetAllMirrorsCount() == 0 {
		engine.SendMessage(b, "No Mirror to cancel.", message)
		return nil
	}
	for _, dl := range engine.GetAllMirrors() {
		if dl.GetStatusType() != engine.MirrorStatusUploading {
			dl.CancelMirror()
			count += 1
		}
	}
	engine.SendMessage(b, fmt.Sprintf("%d mirrors cancelled.", count), message)
	return nil
}

func LoadCancelMirrorHandler(updater *gotgbot.Updater, l *zap.SugaredLogger) {
	defer l.Info("CancelMirror Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("cancel", CancelMirrorHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("cancelall", CancelAllMirrorsHandler))
}
