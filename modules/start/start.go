package start

import (
	"MirrorBotGo/db"
	"MirrorBotGo/engine"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"go.uber.org/zap"
)

func StartHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	msg := ctx.EffectiveMessage
	engine.SendMessage(b, "Hi I am mirror bot", msg)
	return nil
}

func LoadStartHandler(updater *ext.Updater, l *zap.SugaredLogger) {
	defer l.Info("Start Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("start", StartHandler))
}
