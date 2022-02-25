package mirrorstatus

import (
	"MirrorBotGo/db"
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"go.uber.org/zap"
)

func MirrorStatusHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	message := ctx.EffectiveMessage
	if engine.GetAllMirrorsCount() == 0 {
		out, _ := engine.SendMessage(b, "No Active Mirrors.", message)
		engine.AutoDeleteMessages(b, utils.GetAutoDeleteTimeOut(), out, message)
		return nil
	}
	engine.SendStatusMessage(b, message)
	engine.DeleteMessage(b, message)
	return nil
}

func MirrorStatusPreviousHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	cq := ctx.CallbackQuery
	cq.Answer(b, nil)
	status_msg := engine.GetMessageByChatId(ctx.EffectiveChat.Id)
	if status_msg == nil {
		return nil
	}
	if engine.GetAllMirrorsCount() > engine.STATUS_MESSAGE_CHUNKSIZE {
		status_msg.Date = status_msg.Date - 1
	}
	engine.UpdateAllMessages(b)
	return nil
}

func MirrorStatusFirstHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	cq := ctx.CallbackQuery
	cq.Answer(b, nil)
	status_msg := engine.GetMessageByChatId(ctx.EffectiveChat.Id)
	if status_msg == nil {
		return nil
	}
	if engine.GetAllMirrorsCount() > engine.STATUS_MESSAGE_CHUNKSIZE {
		status_msg.Date = 0
	}
	engine.UpdateAllMessages(b)
	return nil
}

func MirrorStatusNextHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	cq := ctx.CallbackQuery
	cq.Answer(b, nil)
	status_msg := engine.GetMessageByChatId(ctx.EffectiveChat.Id)
	if status_msg == nil {
		return nil
	}
	if engine.GetAllMirrorsCount() > engine.STATUS_MESSAGE_CHUNKSIZE {
		status_msg.Date = status_msg.Date + 1
	}
	engine.UpdateAllMessages(b)
	return nil
}

func MirrorStatusLastHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	cq := ctx.CallbackQuery
	cq.Answer(b, nil)
	status_msg := engine.GetMessageByChatId(ctx.EffectiveChat.Id)
	if status_msg == nil {
		return nil
	}
	if engine.GetAllMirrorsCount() > engine.STATUS_MESSAGE_CHUNKSIZE {
		status_msg.Date = int64(len(engine.GetAllMirrorsChunked(engine.STATUS_MESSAGE_CHUNKSIZE)) - 1)
	}
	engine.UpdateAllMessages(b)
	return nil
}

func LoadMirrorStatusHandler(updater *ext.Updater, l *zap.SugaredLogger) {
	defer l.Info("MirrorStatus Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("status", MirrorStatusHandler))
	updater.Dispatcher.AddHandler(handlers.NewCallback(func(cq *gotgbot.CallbackQuery) bool {
		return cq.Data == "previous"
	}, MirrorStatusPreviousHandler))
	updater.Dispatcher.AddHandler(handlers.NewCallback(func(cq *gotgbot.CallbackQuery) bool {
		return cq.Data == "next"
	}, MirrorStatusNextHandler))
	updater.Dispatcher.AddHandler(handlers.NewCallback(func(cq *gotgbot.CallbackQuery) bool {
		return cq.Data == "first"
	}, MirrorStatusFirstHandler))
	updater.Dispatcher.AddHandler(handlers.NewCallback(func(cq *gotgbot.CallbackQuery) bool {
		return cq.Data == "last"
	}, MirrorStatusLastHandler))
}
