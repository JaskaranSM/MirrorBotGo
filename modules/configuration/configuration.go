package configuration

import (
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

func SetUploadChunkSizeHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsUserOwner(ctx.EffectiveUser.Id) {
		return nil
	}
	message := ctx.EffectiveMessage
	chunkSizeString := utils.ParseMessageArgs(message.Text)
	if chunkSizeString == "" {
		engine.SendMessage(b, "Provide arg bruh", message)
		return nil
	}
	chunkSizeBytes, err := humanize.ParseBytes(chunkSizeString)
	if err != nil {
		engine.L().Errorf("Error parsing chunksize value, %s", err.Error())
		engine.SendMessage(b, fmt.Sprintf("Error parsing chunksize value, %s", err.Error()), message)
		return nil
	}
	if chunkSizeBytes%256 != 0 {
		engine.L().Errorf("Error setting chunksize: chunk size must be multiple of 256")
		engine.SendMessage(b, "Error setting chunksize: chunk size must be multiple of 256", message)
		return nil
	}
	engine.L().Infof("Setting upload chunksize to %s | %d", utils.GetHumanBytes(int64(chunkSizeBytes)), chunkSizeBytes)
	engine.SetUploadChunkSize(int(chunkSizeBytes))
	engine.SendMessage(b, fmt.Sprintf("Upload chunk size has been set to %s", utils.GetHumanBytes(int64(engine.GetUploadChunkSize()))), message)
	return nil
}

func GetUploadChunkSizeHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsUserOwner(ctx.EffectiveUser.Id) {
		return nil
	}
	message := ctx.EffectiveMessage
	engine.SendMessage(b, fmt.Sprintf("Upload chunksize is: <code>%s</code> | <code>%d</code>", utils.GetHumanBytes(int64(engine.GetUploadChunkSize())), engine.GetUploadChunkSize()), message)
	return nil
}

func LoadConfigurationHandlers(updater *ext.Updater, l *zap.SugaredLogger) {
	defer l.Info("Configuration Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("setuploadchunksize", SetUploadChunkSizeHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("getuploadchunksize", GetUploadChunkSizeHandler))
}
