package mirror

import (
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"

	"github.com/PaulSonOfLars/gotgbot"
	"github.com/PaulSonOfLars/gotgbot/ext"
	"github.com/PaulSonOfLars/gotgbot/handlers"
	"go.uber.org/zap"
)

func MirrorTorrent(b ext.Bot, u *gotgbot.Update, isTar bool) error {
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
	listener := engine.NewMirrorListener(b, u, isTar)
	err := engine.NewTorrentDownload(link, &listener)
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

func MirrorHttp(b ext.Bot, u *gotgbot.Update, isTar bool) error {
	message := u.EffectiveMessage
	link := utils.ParseMessageArgs(message.Text)
	if link == "" {
		engine.SendMessage(b, "No Source Provided.", message)
		return nil
	}
	listener := engine.NewMirrorListener(b, u, isTar)
	err := engine.NewHttpDownload(link, &listener)
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

func MirrorTorrentHandler(b ext.Bot, u *gotgbot.Update) error {
	if !utils.IsUserSudo(u.EffectiveUser.Id) {
		return nil
	}
	return MirrorTorrent(b, u, false)
}

func MirrorHttpHandler(b ext.Bot, u *gotgbot.Update) error {
	if !utils.IsUserSudo(u.EffectiveUser.Id) {
		return nil
	}
	return MirrorHttp(b, u, false)
}

func TarMirrorTorrentHandler(b ext.Bot, u *gotgbot.Update) error {
	if !utils.IsUserSudo(u.EffectiveUser.Id) {
		return nil
	}
	return MirrorTorrent(b, u, true)
}

func TarMirrorHttpHandler(b ext.Bot, u *gotgbot.Update) error {
	if !utils.IsUserSudo(u.EffectiveUser.Id) {
		return nil
	}
	return MirrorHttp(b, u, true)
}

func LoadMirrorHandlers(updater *gotgbot.Updater, l *zap.SugaredLogger) {
	defer l.Info("Mirror Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("torrent", MirrorTorrentHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("http", MirrorHttpHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("tartorrent", TarMirrorTorrentHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("tarhttp", TarMirrorHttpHandler))
}
