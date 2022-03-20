package botlog

import (
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"
	"os"

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
	handle, err := os.Open(engine.LogFile)
	if err != nil {
		engine.L().Error(err)
		return nil
	}
	_, err = b.SendDocument(
		chat.Id, handle, &gotgbot.SendDocumentOpts{
			ReplyToMessageId: msg.MessageId,
		},
	)
	engine.L().Error(err)
	return nil
}

func LoadLogHandler(updater *ext.Updater, l *zap.SugaredLogger) {
	defer l.Info("Log Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("log", LogHandler))
}
