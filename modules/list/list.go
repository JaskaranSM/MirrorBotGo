package list

import (
	"MirrorBotGo/db"
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"
	"fmt"

	"github.com/PaulSonOfLars/gotgbot"
	"github.com/PaulSonOfLars/gotgbot/ext"
	"github.com/PaulSonOfLars/gotgbot/handlers"
	"go.uber.org/zap"
)

func ListHandler(b ext.Bot, u *gotgbot.Update) error {
	if !db.IsAuthorized(u.EffectiveMessage) {
		return nil
	}
	var out *ext.Message
	var err error
	message := u.EffectiveMessage
	name := utils.ParseMessageArgs(message.Text)
	if name == "" {
		engine.SendMessage(b, "Provide search query.", message)
		return nil
	}
	outMsg := ""
	drive := engine.NewGDriveClient(0, nil)
	drive.Init("")
	drive.Authorize()
	files := drive.ListFilesByParentId(utils.GetGDriveParentId(), name, 20)
	if len(files) == 0 {
		outMsg += "No Result Found."
	}
	for _, file := range files {
		if file.MimeType == drive.GDRIVE_DIR_MIMETYPE {
			outMsg += fmt.Sprintf("⁍ <a href='%s'>%s</a> (folder)\n", drive.FormatLink(file.Id), file.Name)
		} else {
			outMsg += fmt.Sprintf("⁍ <a href='%s'>%s</a> (%s)\n", drive.FormatLink(file.Id), file.Name, utils.GetHumanBytes(file.Size))
		}
	}
	out, err = engine.SendMessage(b, outMsg, message)
	if err != nil {
		engine.SendMessage(b, err.Error(), message)
		return nil
	}
	engine.AutoDeleteMessages(b, utils.GetAutoDeleteTimeOut(), out, message)
	return nil
}

func LoadListHandler(updater *gotgbot.Updater, l *zap.SugaredLogger) {
	defer l.Info("List Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("list", ListHandler))
}
