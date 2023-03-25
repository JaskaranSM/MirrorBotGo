package clone

import (
	"MirrorBotGo/db"
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"go.uber.org/zap"
)

func extract(link string, b *gotgbot.Bot, ctx *ext.Context) (string, error) {
	if !db.IsExtractable(link) {
		return "", fmt.Errorf("link is not extractable")
	}
	extractors, err := db.GetExtractors()
	if err != nil {
		return "", err
	}
	secrets, err := db.GetSecrets()
	if err != nil {
		return "", err
	}
	newLink, err := engine.ExtractDDL(link, extractors, secrets, b, ctx)
	if err != nil {
		return "", err
	}
	return newLink, nil
}

func Clone(b *gotgbot.Bot, ctx *ext.Context, sendStatusMessage bool) error {
	message := ctx.EffectiveMessage
	link := utils.ParseMessageArgs(message.Text)
	if link == "" {
		engine.SendMessage(b, "Provide GDrive Shareable link to clone.", message)
	} else {
		var parentId string
		if strings.Contains(link, "|") {
			data := strings.SplitN(link, "|", 2)
			parentId = utils.GetFileIdByGDriveLink(strings.TrimSpace(data[1]))
			link = strings.TrimSpace(data[0])
		} else {
			parentId = utils.GetGDriveParentId()
		}
		newLink, err := extract(link, b, ctx)
		if err != nil {
			engine.L().Infof("clone: extraction failed: %s", link)
		} else {
			link = newLink
		}
		fileId := utils.GetFileIdByGDriveLink(link)
		if fileId == "" {
			engine.SendMessage(b, "FileId extraction failed, make sure GDrive link is correct.", message)
		} else {
			listener := engine.NewCloneListener(b, ctx, parentId)
			engine.NewGDriveCloneTransferService(fileId, parentId, &listener)
			if sendStatusMessage {
				err := engine.SendStatusMessage(b, message, false)
				if err != nil {
					engine.SendMessage(b, err.Error(), message)
				}
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
