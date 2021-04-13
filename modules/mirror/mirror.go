package mirror

import (
	"MirrorBotGo/db"
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"
	"log"
	"strings"

	"github.com/PaulSonOfLars/gotgbot"
	"github.com/PaulSonOfLars/gotgbot/ext"
	"github.com/PaulSonOfLars/gotgbot/handlers"
	"go.uber.org/zap"
)

func Mirror(b ext.Bot, u *gotgbot.Update, isTar bool, doUnArchive bool) error {
	message := u.EffectiveMessage
	var link string
	var isTgDownload bool = false
	var parentId string
	if message.ReplyToMessage != nil && message.ReplyToMessage.Document != nil {
		doc := message.ReplyToMessage.Document
		if doc.MimeType == "application/x-bittorrent" {
			file, err := b.GetFile(doc.FileId)
			if err != nil {
				b.Logger.Error(err)
			}
			if strings.Contains(message.Text, "|") {
				data := strings.SplitN(message.Text, "|", 2)
				if len(data) > 1 {
					parentId = utils.GetFileIdByGDriveLink(strings.TrimSpace(data[1]))
				}
			}
			link = utils.FormatTGFileLink(file.FilePath, b.Token)
		} else {
			isTgDownload = true
		}
	} else if message.ReplyToMessage != nil {
		if message.ReplyToMessage.Audio != nil || message.ReplyToMessage.Video != nil {
			isTgDownload = true
		}
	} else {
		link = utils.ParseMessageArgs(message.Text)
	}
	if !isTgDownload && link == "" {
		engine.SendMessage(b, "No Source Provided.", message)
		return nil
	}
	if isTgDownload {
		if strings.Contains(message.Text, "|") {
			data := strings.SplitN(message.Text, "|", 2)
			if len(data) > 1 {
				parentId = utils.GetFileIdByGDriveLink(strings.TrimSpace(data[1]))
			}
		}
	}
	if strings.Contains(link, "|") {
		data := strings.SplitN(link, "|", 2)
		parentId = utils.GetFileIdByGDriveLink(strings.TrimSpace(data[1]))
		link = strings.TrimSpace(data[0])
	}
	log.Println("ALT: ", parentId)
	fileId := utils.GetFileIdByGDriveLink(link)
	listener := engine.NewMirrorListener(b, u, isTar, doUnArchive, parentId)
	if isTgDownload {
		err := engine.NewTelegramDownload(message.ReplyToMessage, &listener)
		if err != nil {
			engine.SendMessage(b, err.Error(), message)
			return nil
		}
	} else if fileId != "" {
		engine.NewGDriveDownload(fileId, &listener)
	} else if utils.IsMegaLink(link) {
		err := engine.NewMegaDownload(link, &listener)
		if err != nil {
			engine.SendMessage(b, err.Error(), message)
			return nil
		}
	} else {
		err := engine.NewAriaDownload(link, &listener)
		if err != nil {
			engine.SendMessage(b, err.Error(), message)
			return nil
		}
	}
	if !isTgDownload && fileId == "" && link == "" {
		engine.SendMessage(b, "No Source Provided.", message)
		return nil
	}
	engine.SendStatusMessage(b, message)
	if !engine.Spinner.IsRunning() {
		engine.Spinner.Start(b)
	}
	return nil
}

func MirrorHandler(b ext.Bot, u *gotgbot.Update) error {
	if !db.IsAuthorized(u.EffectiveMessage) {
		return nil
	}
	return Mirror(b, u, false, false)
}

func TarMirrorHandler(b ext.Bot, u *gotgbot.Update) error {
	if !db.IsAuthorized(u.EffectiveMessage) {
		return nil
	}
	return Mirror(b, u, true, false)
}

func UnArchMirrorHandler(b ext.Bot, u *gotgbot.Update) error {
	if !db.IsAuthorized(u.EffectiveMessage) {
		return nil
	}
	return Mirror(b, u, false, true)
}

func LoadMirrorHandlers(updater *gotgbot.Updater, l *zap.SugaredLogger) {
	defer l.Info("Mirror Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("mirror", MirrorHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("tarmirror", TarMirrorHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("unarchmirror", UnArchMirrorHandler))
}
