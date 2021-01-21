package botlog

import (
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"
	"os"

	"github.com/PaulSonOfLars/gotgbot"
	"github.com/PaulSonOfLars/gotgbot/ext"
	"github.com/PaulSonOfLars/gotgbot/handlers"
	"go.uber.org/zap"
)

func LogHandler(b ext.Bot, u *gotgbot.Update) error {
	if !utils.IsUserOwner(u.EffectiveUser.Id) {
		return nil
	}
	chat := u.EffectiveChat
	msg := u.EffectiveMessage
	reader, _ := os.Open(engine.LogFile)
	file := b.NewFileReader(engine.LogFile, reader)
	b.ReplyDocument(chat.Id, file, msg.MessageId)
	return nil
}

func LoadLogHandler(updater *gotgbot.Updater, l *zap.SugaredLogger) {
	defer l.Info("Start Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("log", LogHandler))
}
