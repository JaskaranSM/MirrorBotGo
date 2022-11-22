package list

import (
	"MirrorBotGo/db"
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"go.uber.org/zap"
)

func ListHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	var out *gotgbot.Message
	var err error
	message := ctx.EffectiveMessage
	name := utils.ParseMessageArgs(message.Text)
	if name == "" {
		engine.SendMessage(b, "Provide search query.", message)
		return nil
	}
	outMsg := ""
	drive := engine.NewGDriveClient(0, nil)
	drive.Init("")
	drive.Authorize()
	files, err := drive.ListFilesByParentId(utils.GetGDriveParentId(), name, 20, 1)
	if err != nil {
		engine.SendMessage(b, err.Error(), message)
		return nil
	}
	if len(files) == 0 {
		outMsg += "No Result Found."
	}
	for _, file := range files {
		if file.MimeType == drive.GDRIVE_DIR_MIMETYPE {
			outMsg += fmt.Sprintf("⁍ <a href='%s'>%s</a> (folder)", drive.FormatLink(file.Id), file.Name)
		} else {
			outMsg += fmt.Sprintf("⁍ <a href='%s'>%s</a> (%s)", drive.FormatLink(file.Id), file.Name, utils.GetHumanBytes(file.Size))
		}
		in_url := utils.GetIndexUrl()
		if in_url != "" {
			in_url = in_url + "/" + file.Name
			if file.MimeType == drive.GDRIVE_DIR_MIMETYPE {
				in_url += "/"
			}
			outMsg += fmt.Sprintf(" | <a href='%s'>Index url</a>", in_url)
		}
		outMsg += "\n"
	}
	out, err = engine.SendMessage(b, outMsg, message)
	if err != nil {
		engine.SendMessage(b, err.Error(), message)
		return nil
	}
	engine.AutoDeleteMessages(b, utils.GetAutoDeleteTimeOut(), out, message)
	return nil
}

func LoadListHandler(updater *ext.Updater, l *zap.SugaredLogger) {
	defer l.Info("List Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("list", ListHandler))
}
