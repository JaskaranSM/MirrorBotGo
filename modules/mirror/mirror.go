package mirror

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

func Mirror(b *gotgbot.Bot, ctx *ext.Context, isTar bool, doUnArchive bool, sendStatusMessage bool, isSeed bool) error {
	message := ctx.EffectiveMessage
	var link string
	var isTgDownload bool = false
	var isUsenetDownload bool = false
	var nzbFileName string
	var isTorrent bool = false
	var parentId string
	if message.ReplyToMessage != nil && message.ReplyToMessage.Document != nil {
		doc := message.ReplyToMessage.Document
		if doc.MimeType == "application/x-bittorrent" {
			file, err := b.GetFile(doc.FileId, nil)
			if err != nil {
				engine.L().Error(err)
			}
			if strings.Contains(message.Text, "|") {
				data := strings.SplitN(message.Text, "|", 2)
				if len(data) > 1 {
					parentId = utils.GetFileIdByGDriveLink(strings.TrimSpace(data[1]))
				}
			}
			link = utils.FormatTGFileLink(file.FilePath, b.GetToken())
		} else if doc.MimeType == "application/x-nzb" {
			file, err := b.GetFile(doc.FileId, nil)
			if err != nil {
				engine.L().Error(err)
			}
			if strings.Contains(message.Text, "|") {
				data := strings.SplitN(message.Text, "|", 2)
				if len(data) > 1 {
					parentId = utils.GetFileIdByGDriveLink(strings.TrimSpace(data[1]))
				}
			}
			link = utils.FormatTGFileLink(file.FilePath, b.GetToken())
			nzbFileName = doc.FileName
			isUsenetDownload = true
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
	engine.L().Info("ALT: ", parentId)
	fileId := utils.GetFileIdByGDriveLink(link)
	listener := engine.NewMirrorListener(b, ctx, isTar, doUnArchive, parentId)
	if link != "" {
		isTorrent, _ = utils.IsTorrentLink(link)
	}
	if isTgDownload {
		err := engine.NewTelegramDownload(message.ReplyToMessage, &listener)
		//err := fmt.Errorf("Tg download not supported in this build")
		if err != nil {
			engine.SendMessage(b, err.Error(), message)
			return nil
		}
	} else if isUsenetDownload {
		err := engine.NewUsenetDownload(nzbFileName, link, &listener)
		if err != nil {
			engine.SendMessage(b, err.Error(), message)
			return nil
		}
	} else if fileId != "" {
		engine.NewGDriveDownloadTransferService(fileId, &listener)
	} else if utils.IsMegaLink(link) {
		//err := fmt.Errorf("mega isn't supported in this build")
		err := engine.NewMegaDownload(link, &listener)
		if err != nil {
			engine.SendMessage(b, err.Error(), message)
			return nil
		}
	} else if utils.IsMagnetLink(link) || isTorrent {
		err := engine.NewAnacrolixTorrentDownload(link, &listener, isSeed)
		if err != nil {
			engine.SendMessage(b, err.Error(), message)
			return nil
		}
	} else {
		err := engine.NewHTTPDownload(link, &listener)
		if err != nil {
			engine.SendMessage(b, err.Error(), message)
			return nil
		}
	}
	if !isTgDownload && fileId == "" && link == "" {
		engine.SendMessage(b, "No Source Provided.", message)
		return nil
	}
	if sendStatusMessage {
		err := engine.SendStatusMessage(b, message)
		if err != nil {
			engine.SendMessage(b, err.Error(), message)
			return err
		}
	}
	if !engine.Spinner.IsRunning() {
		engine.Spinner.Start(b)
	}
	return nil
}

func MirrorHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	return Mirror(b, ctx, false, false, true, false)
}

func SilentMirrorhandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	return Mirror(b, ctx, false, false, false, false)
}

func TarMirrorHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	return Mirror(b, ctx, true, false, true, false)
}

func SilentTarMirrorHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	return Mirror(b, ctx, true, false, false, false)
}

func UnArchMirrorHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	return Mirror(b, ctx, false, true, true, false)
}

func SilentUnArchMirrorHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	return Mirror(b, ctx, false, true, false, false)
}

func TorrentHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	return Mirror(b, ctx, false, false, true, false)
}

func SilentTorrenthandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	return Mirror(b, ctx, false, false, false, false)
}

func TarTorrentHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	return Mirror(b, ctx, true, false, true, false)
}

func SilentTarTorrentHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	return Mirror(b, ctx, true, false, false, false)
}

func UnArchTorrentHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	return Mirror(b, ctx, false, true, true, false)
}

func SilentUnArchTorrentHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	return Mirror(b, ctx, false, true, false, false)
}

func SeedTorrentHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	return Mirror(b, ctx, false, false, true, true)
}

func SilentSeedTorrentHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	return Mirror(b, ctx, false, false, false, true)
}

func LoadMirrorHandlers(updater *ext.Updater, l *zap.SugaredLogger) {
	defer l.Info("Mirror Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("mirror", MirrorHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("tarmirror", TarMirrorHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("unarchmirror", UnArchMirrorHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("mirrors", SilentMirrorhandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("tarmirrors", SilentTarMirrorHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("unarchmirrors", SilentUnArchMirrorHandler))

	updater.Dispatcher.AddHandler(handlers.NewCommand("torrent", TorrentHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("tartorrent", TarTorrentHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("unarchtorrent", UnArchTorrentHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("torrents", SilentTorrenthandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("tartorrents", SilentTarTorrentHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("unarchtorrents", SilentUnArchTorrentHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("seedtorrent", SeedTorrentHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("seedtorrents", SilentSeedTorrentHandler))
}
