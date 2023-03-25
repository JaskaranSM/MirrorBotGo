package mirrorstatus

import (
	"MirrorBotGo/db"
	"MirrorBotGo/engine"
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
	err := engine.SendStatusMessage(b, message, true)
	if err != nil {
		engine.SendMessage(b, err.Error(), message)
		return nil
	}
	engine.Spinner.Start(b)
	return nil
}

func MirrorStatusPreviousHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	cq := ctx.CallbackQuery
	_, err := cq.Answer(b, nil)
	if err != nil {
		engine.L().Errorf("MirrorStatusPreviousHandler: callback: %v", err)
		return nil
	}
	statusMsg := engine.GetMessageByChatId(ctx.EffectiveChat.Id)
	if statusMsg == nil {
		return nil
	}
	if engine.GetAllMirrorsCount() > engine.StatusMessageChunkSize {
		statusMsg.Date = statusMsg.Date - 1
	}
	engine.UpdateAllMessages(b)
	return nil
}

func MirrorStatusFirstHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	cq := ctx.CallbackQuery
	_, err := cq.Answer(b, nil)
	if err != nil {
		engine.L().Errorf("MirrorStatusFirstHandler: callback: %v", err)
		return nil
	}
	statusMsg := engine.GetMessageByChatId(ctx.EffectiveChat.Id)
	if statusMsg == nil {
		return nil
	}
	if engine.GetAllMirrorsCount() > engine.StatusMessageChunkSize {
		statusMsg.Date = 0
	}
	engine.UpdateAllMessages(b)
	return nil
}

func MirrorStatusNextHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	cq := ctx.CallbackQuery
	_, err := cq.Answer(b, nil)
	if err != nil {
		engine.L().Errorf("MirrorStatusNextHandler: callback: %v", err)
		return nil
	}
	statusMsg := engine.GetMessageByChatId(ctx.EffectiveChat.Id)
	if statusMsg == nil {
		return nil
	}
	if engine.GetAllMirrorsCount() > engine.StatusMessageChunkSize {
		statusMsg.Date = statusMsg.Date + 1
	}
	engine.UpdateAllMessages(b)
	return nil
}

func MirrorStatusLastHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	cq := ctx.CallbackQuery
	_, err := cq.Answer(b, nil)
	if err != nil {
		engine.L().Errorf("MirrorStatusLastHandler: callback: %v", err)
		return nil
	}
	statusMsg := engine.GetMessageByChatId(ctx.EffectiveChat.Id)
	if statusMsg == nil {
		return nil
	}
	if engine.GetAllMirrorsCount() > engine.StatusMessageChunkSize {
		statusMsg.Date = int64(len(engine.GetAllMirrorsChunked(engine.StatusMessageChunkSize)) - 1)
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
