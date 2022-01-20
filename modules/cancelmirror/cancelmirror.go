package cancelmirror

import (
	"MirrorBotGo/db"
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"
	"fmt"

	"github.com/PaulSonOfLars/gotgbot"
	"github.com/PaulSonOfLars/gotgbot/ext"
	"github.com/PaulSonOfLars/gotgbot/handlers"
	"go.uber.org/zap"
)

func CancelMirrorHandler(b ext.Bot, u *gotgbot.Update) error {
	if !db.IsAuthorized(u.EffectiveMessage) {
		return nil
	}
	var dl engine.MirrorStatus
	message := u.EffectiveMessage
	gid := utils.ParseMessageArgs(message.Text)
	if message.ReplyToMessage == nil && gid == "" {
		engine.SendMessage(b, "Reply to mirror start message or provide gid to cancel it.", message)
		return nil
	}
	if message.ReplyToMessage != nil {
		dl = engine.GetMirrorByUid(message.ReplyToMessage.MessageId)
	} else if gid != "" {
		dl = engine.GetMirrorByGid(gid)
	} else {
		dl = nil
	}
	if dl == nil {
		engine.SendMessage(b, "Mirror doesnt exists.", message)
		return nil
	}
	status := dl.GetStatusType()
	if status == engine.MirrorStatusDownloading || status == engine.MirrorStatusWaiting || status == engine.MirrorStatusFailed || status == engine.MirrorStatusStreaming {
		dl.CancelMirror()
	} else {
		engine.SendMessage(b, "Can only cancel downloads.", message)
		return nil
	}
	return nil
}

func CancelAllMirrorsHandler(b ext.Bot, u *gotgbot.Update) error {
	if !utils.IsUserOwner(u.EffectiveUser.Id) {
		return nil
	}
	count := 0
	message := u.EffectiveMessage
	if engine.GetAllMirrorsCount() == 0 {
		engine.SendMessage(b, "No Mirror to cancel.", message)
		return nil
	}
	for _, dl := range engine.GetAllMirrors() {
		status := dl.GetStatusType()
		if status == engine.MirrorStatusDownloading || status == engine.MirrorStatusWaiting || status == engine.MirrorStatusFailed || status == engine.MirrorStatusStreaming {
			dl.CancelMirror()
			count += 1
		}
	}
	engine.SendMessage(b, fmt.Sprintf("%d mirror(s) cancelled.", count), message)
	return nil
}

func LoadCancelMirrorHandler(updater *gotgbot.Updater, l *zap.SugaredLogger) {
	defer l.Info("CancelMirror Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("cancel", CancelMirrorHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("cancelall", CancelAllMirrorsHandler))
}
