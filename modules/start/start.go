package start

import (
	"MirrorBotGo/utils"

	"github.com/PaulSonOfLars/gotgbot"
	"github.com/PaulSonOfLars/gotgbot/ext"
	"github.com/PaulSonOfLars/gotgbot/handlers"
	"go.uber.org/zap"
)

func StartHandler(b ext.Bot, u *gotgbot.Update) error {
	if !utils.IsUserSudo(u.EffectiveUser.Id) {
		return nil
	}
	msg := u.EffectiveMessage
	_, err := msg.ReplyHTML("Hi I am mirror bot")
	if err != nil {
		b.Logger.Error(err)
	}
	return nil
}

func LoadStartHandler(updater *gotgbot.Updater, l *zap.SugaredLogger) {
	defer l.Info("Start Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("start", StartHandler))
}
