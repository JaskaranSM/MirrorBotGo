package botlog

import (
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"go.uber.org/zap"
)

func LogHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsUserOwner(ctx.EffectiveUser.Id) {
		return nil
	}
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	b.SendDocument(
		chat.Id, engine.LogFile, &gotgbot.SendDocumentOpts{
			ReplyToMessageId: msg.MessageId,
		},
	)
	return nil
}

func LoadLogHandler(updater *ext.Updater, l *zap.SugaredLogger) {
	defer l.Info("Start Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("log", LogHandler))
}
