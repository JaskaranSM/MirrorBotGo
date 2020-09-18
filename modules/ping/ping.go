package ping

import (
	"fmt"
	"math"
	"time"

	"github.com/PaulSonOfLars/gotgbot"
	"github.com/PaulSonOfLars/gotgbot/ext"
	"github.com/PaulSonOfLars/gotgbot/handlers"
	"go.uber.org/zap"
)

func PingHandler(b ext.Bot, u *gotgbot.Update) error {
	startTime := time.Now()
	message, err := b.SendMessage(u.Message.Chat.Id, "Starting Ping!")
	if err != nil {
		b.Logger.Error(err)
	}
	endTime := time.Now()
	elapsed := int(math.Round(float64(endTime.Sub(startTime).Milliseconds())))
	_, _ = b.EditMessageText(u.Message.Chat.Id, message.MessageId, fmt.Sprintf("Pong %d ms", elapsed))
	return nil
}

func LoadPingHandler(updater *gotgbot.Updater, l *zap.SugaredLogger) {
	defer l.Info("Stats Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("ping", PingHandler))
}

