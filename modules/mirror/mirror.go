package mirror

import (
	"MirrorBotGo/db"
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"

	"github.com/PaulSonOfLars/gotgbot"
	"github.com/PaulSonOfLars/gotgbot/ext"
	"github.com/PaulSonOfLars/gotgbot/handlers"
	"go.uber.org/zap"
)

func Mirror(b ext.Bot, u *gotgbot.Update, isTar bool, doUnArchive bool) error {
	message := u.EffectiveMessage
	var link string
	if message.ReplyToMessage != nil && message.ReplyToMessage.Document != nil {
		doc := message.ReplyToMessage.Document
		if doc.MimeType == "application/x-bittorrent" {
			file, err := b.GetFile(doc.FileId)
			if err != nil {
				b.Logger.Error(err)
			}
			link = utils.FormatTGFileLink(file.FilePath, b.Token)
		}
	} else {
		link = utils.ParseMessageArgs(message.Text)
	}
	if link == "" {
		engine.SendMessage(b, "No Source Provided.", message)
		return nil
	}
	listener := engine.NewMirrorListener(b, u, isTar, doUnArchive)
	err := engine.NewAriaDownload(link, &listener)
	if err != nil {
		engine.SendMessage(b, err.Error(), message)
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
