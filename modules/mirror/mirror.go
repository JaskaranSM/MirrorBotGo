package mirror

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

type PrepareMirrorOptions struct {
	B                 *gotgbot.Bot
	Ctx               *ext.Context
	Message           *gotgbot.Message
	IsTar             bool
	DoUnArchive       bool
	SendStatusMessage bool
	Seed              bool
}

func HandleSendStatusMessage(opts *PrepareMirrorOptions) {
	if !opts.SendStatusMessage {
		return
	}
	err := engine.SendStatusMessage(opts.B, opts.Message)
	if err != nil {
		engine.L().Error(err)
		engine.SendMessage(opts.B, err.Error(), opts.Message)
		return
	}
}

func PrepareMirror(opts *PrepareMirrorOptions) error {
	var (
		link     string
		parentId string
	)
	var (
		isTorrent bool
	)

	result, err := prepareTgDownload(opts)
	if err != nil {
		engine.SendMessage(opts.B, err.Error(), opts.Message)
		return nil
	}
	parentId = result.ParentId
	link = result.Link
	listener := engine.NewMirrorListener(opts.B, opts.Ctx, opts.IsTar, opts.DoUnArchive, parentId)
	if result.IsUsenetDownload {
		err := engine.NewUsenetDownload(result.NzbFileName, link, &listener)
		if err != nil {
			engine.SendMessage(opts.B, err.Error(), opts.Message)
			return nil
		}
		defer func() {
			HandleSendStatusMessage(opts)
		}()
		return nil
	}

	if result.IsTgDownload {
		err := engine.NewTelegramDownload(opts.Message.ReplyToMessage, &listener)
		if err != nil {
			engine.SendMessage(opts.B, err.Error(), opts.Message)
			return nil
		}
		defer func() {
			HandleSendStatusMessage(opts)
		}()
		return nil
	}
	if result.IsTorrent {
		isTorrent = true
		opts.Message.Text = fmt.Sprintf("/mirror %s", result.Link) //TODO: find better way to handle this case
	}

	link = utils.ParseMessageArgs(opts.Message.Text)
	engine.L().Info(link)
	fileId := utils.GetFileIdByGDriveLink(opts.Message.Text)
	if fileId != "" {
		engine.NewGDriveDownloadTransferService(fileId, &listener)
		defer func() {
			HandleSendStatusMessage(opts)
		}()
		return nil
	}
	if utils.IsMegaLink(link) {
		err := engine.NewMegaDownload(link, &listener)
		if err != nil {
			engine.SendMessage(opts.B, err.Error(), opts.Message)
			return nil
		}
		defer func() {
			HandleSendStatusMessage(opts)
		}()
		return nil
	}

	if link == "" && fileId == "" {
		engine.SendMessage(opts.B, "No source provided", opts.Message)
		return nil
	}
	newLink, err := extract(link, opts.B, opts.Ctx)
	if err != nil {
		engine.L().Infof("Failed to extract ddl even: %v", err)
	} else {
		link = newLink
		fileId = utils.GetFileIdByGDriveLink(link)
	}
	isTorrent, _ = utils.IsTorrentLink(link)

	if utils.IsMagnetLink(link) || isTorrent {
		err := engine.NewKedgeDownload(link, &listener, opts.Seed)
		if err != nil {
			engine.SendMessage(opts.B, err.Error(), opts.Message)
			return nil
		}
		defer func() {
			HandleSendStatusMessage(opts)
		}()
	} else {
		err := engine.NewHTTPDownload(link, &listener)
		if err != nil {
			engine.SendMessage(opts.B, err.Error(), opts.Message)
			return nil
		}
		defer func() {
			HandleSendStatusMessage(opts)
		}()
	}
	if !engine.Spinner.IsRunning() {
		engine.Spinner.Start(opts.B)
	}
	return nil

}

type TgDownloadDetectionResult struct {
	Link             string
	ParentId         string
	NzbFileName      string
	IsUsenetDownload bool
	IsTgDownload     bool
	IsTorrent        bool
}

func prepareTgDownload(opts *PrepareMirrorOptions) (TgDownloadDetectionResult, error) {
	var result TgDownloadDetectionResult
	if opts.Message.ReplyToMessage != nil {
		if opts.Message.ReplyToMessage.Document != nil {
			document := opts.Message.ReplyToMessage.Document
			if document.MimeType == "application/x-nzb" {
				result.IsUsenetDownload = true
				result.NzbFileName = document.FileName
			}
			if document.MimeType == "application/x-bittorrent" {
				result.IsTorrent = true
			}
			if document.MimeType == "application/x-bittorrent" || document.MimeType == "application/x-nzb" {
				file, err := opts.B.GetFile(document.FileId, nil)
				if err != nil {
					engine.L().Error(err)
					return result, err
				}
				result.Link = utils.FormatTGFileLink(file.FilePath, opts.B.GetToken())
			} else {
				result.IsTgDownload = true
			}
		}
		if opts.Message.ReplyToMessage.Audio != nil || opts.Message.ReplyToMessage.Video != nil {
			result.IsTgDownload = true
		}
	}

	if strings.Contains(opts.Message.Text, "|") {
		data := strings.SplitN(opts.Message.Text, "|", 2)
		if len(data) > 1 {
			result.ParentId = utils.GetFileIdByGDriveLink(strings.TrimSpace(data[1]))
		}
	}

	return result, nil
}

func MirrorHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	return PrepareMirror(&PrepareMirrorOptions{
		B:                 b,
		Ctx:               ctx,
		SendStatusMessage: true,
		Message:           ctx.Message,
	})
}

func SilentMirrorhandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	return PrepareMirror(&PrepareMirrorOptions{
		B:       b,
		Ctx:     ctx,
		Message: ctx.Message,
	})
}

func TarMirrorHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	return PrepareMirror(&PrepareMirrorOptions{
		B:                 b,
		Ctx:               ctx,
		SendStatusMessage: true,
		IsTar:             true,
		Message:           ctx.Message,
	})
}

func SilentTarMirrorHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	return PrepareMirror(&PrepareMirrorOptions{
		B:       b,
		Ctx:     ctx,
		IsTar:   true,
		Message: ctx.Message,
	})
}

func UnArchMirrorHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	return PrepareMirror(&PrepareMirrorOptions{
		B:                 b,
		Ctx:               ctx,
		SendStatusMessage: true,
		DoUnArchive:       true,
		Message:           ctx.Message,
	})
}

func SilentUnArchMirrorHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	return PrepareMirror(&PrepareMirrorOptions{
		B:           b,
		Ctx:         ctx,
		DoUnArchive: true,
		Message:     ctx.Message,
	})
}

func SeedTorrentHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	return PrepareMirror(&PrepareMirrorOptions{
		B:                 b,
		Ctx:               ctx,
		SendStatusMessage: true,
		Seed:              utils.GetSeed(),
		Message:           ctx.Message,
	})
}

func SilentSeedTorrentHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	return PrepareMirror(&PrepareMirrorOptions{
		B:       b,
		Ctx:     ctx,
		Seed:    utils.GetSeed(),
		Message: ctx.Message,
	})
}

func LoadMirrorHandlers(updater *ext.Updater, l *zap.SugaredLogger) {
	defer l.Info("Mirror Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("mirror", MirrorHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("tarmirror", TarMirrorHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("unarchmirror", UnArchMirrorHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("mirrors", SilentMirrorhandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("tarmirrors", SilentTarMirrorHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("unarchmirrors", SilentUnArchMirrorHandler))

	updater.Dispatcher.AddHandler(handlers.NewCommand("seedtorrent", SeedTorrentHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("seedtorrents", SilentSeedTorrentHandler))

}
