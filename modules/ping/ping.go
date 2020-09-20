package ping

import (
	"MirrorBotGo/db"
	"MirrorBotGo/engine"
	"fmt"
	"math"
	"time"

	"github.com/PaulSonOfLars/gotgbot"
	"github.com/PaulSonOfLars/gotgbot/ext"
	"github.com/PaulSonOfLars/gotgbot/handlers"
	"go.uber.org/zap"
)

func PingHandler(b ext.Bot, u *gotgbot.Update) error {
	if !db.IsAuthorized(u.EffectiveMessage) {
		return nil
	}
	startTime := time.Now()
	message, err := engine.SendMessage(b, "Starting ping", u.EffectiveMessage)
	if err != nil {
		b.Logger.Error(err)
	}
	endTime := time.Now()
	elapsed := int(math.Round(float64(endTime.Sub(startTime).Milliseconds())))
	_, _ = engine.EditMessage(b, fmt.Sprintf("Pong %d ms", elapsed), message)
	return nil
}

func LoadPingHandler(updater *gotgbot.Updater, l *zap.SugaredLogger) {
	defer l.Info("Ping Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("ping", PingHandler))
}
