package clone

import (
	"MirrorBotGo/db"
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"go.uber.org/zap"
)

func Clone(b *gotgbot.Bot, ctx *ext.Context, sendStatusMessage bool) error {
	message := ctx.EffectiveMessage
	link := utils.ParseMessageArgs(message.Text)
	if link == "" {
		_, err := engine.SendMessage(b, "Provide GDrive Shareable link to clone.", message)
		if err != nil {
			engine.L().Error("SendMessage: " + err.Error())
		}
	} else {
		var parentId string
		if strings.Contains(link, "|") {
			data := strings.SplitN(link, "|", 2)
			parentId = utils.GetFileIdByGDriveLink(strings.TrimSpace(data[1]))
			link = strings.TrimSpace(data[0])
		} else {
			parentId = utils.GetGDriveParentId()
		}
		fileId := utils.GetFileIdByGDriveLink(link)
		if fileId == "" {
			_, err := engine.SendMessage(b, "FileId extraction failed, make sure GDrive link is correct.", message)
			if err != nil {
				engine.L().Error("SendMessage: " + err.Error())
			}
		} else {
			listener := engine.NewCloneListener(b, ctx, parentId)
			engine.NewGDriveClone(fileId, parentId, &listener)
			if sendStatusMessage {
				engine.SendStatusMessage(b, message)
			}
			if !engine.Spinner.IsRunning() {
				engine.Spinner.Start(b)
			}
		}
	}

	return nil
}

func CloneHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	return Clone(b, ctx, true)
}

func SilentCloneHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	return Clone(b, ctx, false)
}

func LoadCloneHandler(updater *ext.Updater, l *zap.SugaredLogger) {
	defer l.Info("Clone Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("clone", CloneHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("clones", SilentCloneHandler))
}
