package cancelmirror

import (
	"MirrorBotGo/db"
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"
	"fmt"
	"strconv"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"go.uber.org/zap"
)

func CancelMirrorHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	var dl engine.MirrorStatus
	message := ctx.EffectiveMessage
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
	if status == engine.MirrorStatusDownloading || status == engine.MirrorStatusWaiting || status == engine.MirrorStatusFailed || status == engine.MirrorStatusCloning || status == engine.MirrorStatusSeeding || status == engine.MirrorStatusUploading {
		dl.CancelMirror()
	} else {
		engine.SendMessage(b, "Can only cancel downloads/seeds/clones.", message)
		return nil
	}
	return nil
}

func CancelAllMirrorsHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsUserOwner(ctx.EffectiveUser.Id) {
		return nil
	}
	count := 0
	message := ctx.EffectiveMessage
	if engine.GetAllMirrorsCount() == 0 {
		engine.SendMessage(b, "No Mirror to cancel.", message)
		return nil
	}
	for _, dl := range engine.GetAllMirrors() {
		status := dl.GetStatusType()
		if status == engine.MirrorStatusDownloading || status == engine.MirrorStatusWaiting || status == engine.MirrorStatusFailed || status == engine.MirrorStatusCloning || status == engine.MirrorStatusSeeding || status == engine.MirrorStatusUploading {
			if dl.CancelMirror() {
				count += 1
			}
		}
	}
	engine.SendMessage(b, fmt.Sprintf("%d mirror(s) cancelled.", count), message)
	return nil
}

func CancelMirrorByIDHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsUserOwner(ctx.EffectiveUser.Id) {
		return nil
	}
	message := ctx.EffectiveMessage
	id := utils.ParseMessageArgs(message.Text)
	idInt, err := strconv.Atoi(id)
	if err != nil {
		engine.SendMessage(b, err.Error(), message)
		return nil
	}
	dl := engine.GetMirrorByIndex(idInt)
	if dl == nil {
		engine.SendMessage(b, "mirror doesnt exist with that ID", message)
		return nil
	}
	dl.CancelMirror()
	engine.SendMessage(b, fmt.Sprintf("CancelMirror() called on <code>%s</code>", dl.Name()), message)
	return nil
}

func LoadCancelMirrorHandler(updater *ext.Updater, l *zap.SugaredLogger) {
	defer l.Info("CancelMirror Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("cancel", CancelMirrorHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("cancelall", CancelAllMirrorsHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("cid", CancelMirrorByIDHandler))
}
