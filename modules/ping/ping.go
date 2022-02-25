package ping

import (
	"MirrorBotGo/db"
	"MirrorBotGo/engine"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"go.uber.org/zap"
)

func PingHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	startTime := time.Now()
	message, err := engine.SendMessage(b, "Starting ping", ctx.EffectiveMessage)
	if err != nil {
		log.Println(err)
	}
	endTime := time.Now()
	elapsed := int(math.Round(float64(endTime.Sub(startTime).Milliseconds())))
	_, _ = engine.EditMessage(b, fmt.Sprintf("Pong %d ms", elapsed), message)
	return nil
}

func LoadPingHandler(updater *ext.Updater, l *zap.SugaredLogger) {
	defer l.Info("Ping Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("ping", PingHandler))
}
