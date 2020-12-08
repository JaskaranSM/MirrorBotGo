package clone

import (
	"MirrorBotGo/db"
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"
	"log"

	"github.com/PaulSonOfLars/gotgbot"
	"github.com/PaulSonOfLars/gotgbot/ext"
	"github.com/PaulSonOfLars/gotgbot/handlers"
	"go.uber.org/zap"
)

func CloneHandler(b ext.Bot, u *gotgbot.Update) error {
	if !db.IsAuthorized(u.EffectiveMessage) {
		return nil
	}
	message := u.EffectiveMessage
	link := utils.ParseMessageArgs(message.Text)
	if link == "" {
		_, err := engine.SendMessage(b, "Provide GDrive Shareable link to clone.", message)
		if err != nil {
			log.Println("SendMessage: " + err.Error())
		}
	} else {
		fileId := utils.GetFileIdByGDriveLink(link)
		if fileId == "" {
			_, err := engine.SendMessage(b, "FileId extraction failed, make sure GDrive link is correct.", message)
			if err != nil {
				log.Println("SendMessage: " + err.Error())
			}
		} else {
			msg, _ := engine.SendMessage(b, "Cloning: <code>"+link+"</code>", message)
			drive_client := engine.NewGDriveClient(0, nil)
			drive_client.Init("")
			drive_client.Authorize()
			out_link := drive_client.Clone(fileId)
			engine.DeleteMessage(b, msg)
			_, err := engine.SendMessage(b, out_link, message)
			if err != nil {
				log.Println("SendMessage: " + err.Error())
			}
		}
	}

	return nil
}

func LoadCloneHandler(updater *gotgbot.Updater, l *zap.SugaredLogger) {
	defer l.Info("Clone Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("clone", CloneHandler))
}
