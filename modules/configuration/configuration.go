package configuration

import (
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"
	"fmt"
	"strconv"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"go.uber.org/zap"
)

func SetGotdDownloadThreadsCountHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsUserOwner(ctx.EffectiveUser.Id) {
		return nil
	}
	message := ctx.EffectiveMessage
	threadCountString := utils.ParseMessageArgs(message.Text)
	if threadCountString == "" {
		engine.SendMessage(b, "Provide arg bruh", message)
		return nil
	}
	threadCountInt, err := strconv.Atoi(threadCountString)
	if err != nil {
		engine.L().Errorf("Error parsing gotd thread count: %s", err.Error())
		engine.SendMessage(b, fmt.Sprintf("Error parsing thread count: %s", err.Error()), message)
	}
	if threadCountInt <= 0 {
		engine.L().Errorf("Error setting gotd thread count: thread count must be above 0")
		engine.SendMessage(b, "Error setting gotd thread count: thread count must be above 0", message)
		return nil
	}
	engine.L().Infof("Setting gotd download threads count %d", threadCountInt)
	engine.SetGotdDownloadThreadsCount(threadCountInt)
	engine.SendMessage(b, fmt.Sprintf("Gotd download threads count has been set to %d", threadCountInt), message) //engine.GetGotdDownloadThreadsCount()), message)
	return nil
}

func GetGotdDownloadThreadsCountHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsUserOwner(ctx.EffectiveUser.Id) {
		return nil
	}
	message := ctx.EffectiveMessage
	engine.SendMessage(b, fmt.Sprintf("Gotd download thread count: <code>%d</code>", engine.GetGotdDownloadThreadsCount()), message)
	return nil
}

func MegaLoginHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsUserOwner(ctx.EffectiveUser.Id) {
		return nil
	}
	message := ctx.EffectiveMessage
	out := ""
	err := engine.PerformMegaLogin()
	if err != nil {
		out = fmt.Sprintf("Mega login failed: %s", err.Error())
	} else {
		out = "Mega login success."
	}
	engine.SendMessage(b, out, message)
	return nil
}

func LoadConfigurationHandlers(updater *ext.Updater, l *zap.SugaredLogger) {
	defer l.Info("Configuration Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("setgotdthreads", SetGotdDownloadThreadsCountHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("getgotdthreads", GetGotdDownloadThreadsCountHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("megalogin", MegaLoginHandler))
}
