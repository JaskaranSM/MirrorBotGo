package app

import (
	"bytes"
	"context"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gotd/botapi"

	"mirrorbot/internal/mirror"
	"mirrorbot/internal/sources/httpdl"
	"mirrorbot/internal/sources/tgfile"
	"mirrorbot/internal/status"
	"mirrorbot/internal/util"
)

func (a *App) cmdStart(c *botapi.Context) error {
	if !a.authorized(c, c.Message()) {
		return nil
	}
	_, err := c.Reply("Hi I am mirror bot")
	return err
}

// prepareMirror routes a mirror request to the right source.
func (a *App) prepareMirror(c *botapi.Context, isTar, doUnArchive, sendStatus, seed bool) {
	msg := c.Message()
	uid := int64(msg.MessageID)
	chatID := msg.Chat.ID
	replyTo := msg.MessageID
	dir := filepath.Join(a.cfg.DownloadDir, strconv.FormatInt(uid, 10))

	arg := util.ParseMessageArgs(msg.Text)
	link, parentID := splitParent(arg)
	listener := mirror.NewMirrorListener(a.deps, uid, chatID, replyTo, isTar, doUnArchive, seed, parentID)

	// Reply-to-file: botapi delivers only the replied message id, so fetch the
	// document over MTProto.
	if reply := msg.ReplyToMessage; reply != nil && reply.MessageID != 0 {
		media, err := a.bot.FetchReplyMedia(a.deps.Ctx, chatID, reply.MessageID)
		if err != nil || media == nil {
			a.reply(c, "Couldn't read the replied message. Reply to a message that contains a file or photo.")
			return
		}
		if isTorrentDoc(media.MIME, media.FileName) {
			var buf bytes.Buffer
			if err := a.bot.DownloadMedia(a.deps.Ctx, media.Location, &buf); err != nil {
				a.reply(c, "Failed to read torrent file: "+err.Error())
				return
			}
			a.startTorrent(c, listener, uid, dir, sendStatus, seed, buf.Bytes(), true)
			return
		}
		src := tgfile.New(a.bot, media.Location, media.FileName, media.Size, dir, util.RandGID(16), listener)
		src.SetIndex(a.mgr.NextIndex())
		a.mgr.Set(uid, src)
		a.show(c, sendStatus)
		go src.Start(a.deps.Ctx)
		return
	}

	if link == "" {
		a.reply(c, "Provide a link to mirror, or reply to a file.")
		return
	}

	switch {
	case strings.Contains(link, "drive.google.com"):
		fileID := util.GetFileIDByGDriveLink(link)
		if fileID == "" {
			a.reply(c, "Could not extract a Google Drive file id from that link.")
			return
		}
		tr := a.drive.NewTransfer(status.Downloading, util.RandGID(16))
		tr.SetIndex(a.mgr.NextIndex())
		a.mgr.Set(uid, tr)
		a.show(c, sendStatus)
		go func() {
			if _, err := tr.Download(a.deps.Ctx, fileID, dir); err != nil {
				listener.OnDownloadError(err)
				return
			}
			listener.OnDownloadComplete()
		}()

	case util.IsMagnetLink(link):
		d, err := a.torrents.AddMagnet(link, dir, seed, listener)
		if err != nil {
			a.reply(c, "Failed to add torrent: "+err.Error())
			return
		}
		listener.SetDropper(d.Drop)
		d.SetIndex(a.mgr.NextIndex())
		a.mgr.Set(uid, d)
		a.show(c, sendStatus)
		go d.Start(a.deps.Ctx)

	case strings.HasSuffix(strings.ToLower(link), ".torrent"):
		data, err := fetchURLBytes(a.deps.Ctx, link)
		if err != nil {
			a.reply(c, "Failed to fetch torrent: "+err.Error())
			return
		}
		a.startTorrent(c, listener, uid, dir, sendStatus, seed, data, true)

	default:
		src := httpdl.New(link, dir, util.RandGID(16), listener)
		src.SetIndex(a.mgr.NextIndex())
		a.mgr.Set(uid, src)
		a.show(c, sendStatus)
		go src.Start(a.deps.Ctx)
	}
}

func (a *App) startTorrent(c *botapi.Context, listener *mirror.MirrorListener, uid int64, dir string, sendStatus, seed bool, data []byte, _ bool) {
	d, err := a.torrents.AddMetaInfo(data, dir, seed, listener)
	if err != nil {
		a.reply(c, "Failed to add torrent: "+err.Error())
		return
	}
	listener.SetDropper(d.Drop)
	d.SetIndex(a.mgr.NextIndex())
	a.mgr.Set(uid, d)
	a.show(c, sendStatus)
	go d.Start(a.deps.Ctx)
}

// show sends the status message (if requested) and starts the spinner.
func (a *App) show(c *botapi.Context, sendStatus bool) {
	msg := c.Message()
	if sendStatus {
		a.status.SendStatus(a.deps.Ctx, msg.Chat.ID, msg.MessageID, 0)
	}
	a.status.Start()
}

func (a *App) cmdClone(c *botapi.Context, sendStatus bool) {
	msg := c.Message()
	uid := int64(msg.MessageID)
	arg := util.ParseMessageArgs(msg.Text)
	link, parentID := splitParent(arg)
	if parentID == "" {
		parentID = a.cfg.GDriveParentID
	}
	if link == "" {
		a.reply(c, "Provide a Google Drive shareable link to clone.")
		return
	}
	fileID := util.GetFileIDByGDriveLink(link)
	if fileID == "" {
		a.reply(c, "FileId extraction failed, make sure the Google Drive link is correct.")
		return
	}
	cl := mirror.NewCloneListener(a.deps, uid, msg.Chat.ID, msg.MessageID, parentID)
	cl.Start(fileID)
	a.show(c, sendStatus)
}

func (a *App) cmdStatus(c *botapi.Context) error {
	if !a.authorized(c, c.Message()) {
		return nil
	}
	msg := c.Message()
	a.status.SendStatus(a.deps.Ctx, msg.Chat.ID, 0, msg.MessageID)
	a.status.Start()
	return nil
}

func (a *App) cbStatus(action string) botapi.Handler {
	return func(c *botapi.Context) error {
		_ = c.AnswerCallback()
		cq := c.Update.CallbackQuery
		if cq == nil {
			return nil
		}
		chatID := callbackChatID(cq.Data)
		if chatID == 0 {
			return nil
		}
		switch action {
		case "first":
			a.status.First(a.deps.Ctx, chatID)
		case "previous":
			a.status.Previous(a.deps.Ctx, chatID)
		case "next":
			a.status.Next(a.deps.Ctx, chatID)
		case "last":
			a.status.Last(a.deps.Ctx, chatID)
		}
		return nil
	}
}

func (a *App) cmdCancel(c *botapi.Context) error {
	if !a.authorized(c, c.Message()) {
		return nil
	}
	msg := c.Message()
	gid := util.ParseMessageArgs(msg.Text)
	var dl status.Status
	if msg.ReplyToMessage != nil {
		dl = a.mgr.Get(int64(msg.ReplyToMessage.MessageID))
	} else if gid != "" {
		dl = a.mgr.ByGID(gid)
	}
	if dl == nil {
		a.reply(c, "Reply to a mirror's command message or provide a gid to cancel it.")
		return nil
	}
	switch dl.StatusType() {
	case status.Downloading, status.Waiting, status.Failed, status.Cloning, status.Seeding, status.Uploading:
		dl.Cancel()
	default:
		a.reply(c, "Can only cancel downloads/seeds/clones/uploads.")
	}
	return nil
}

func (a *App) cmdCancelAll(c *botapi.Context) error {
	if !a.isOwner(c.Message()) {
		return nil
	}
	n := 0
	for _, s := range a.mgr.All() {
		s.Cancel()
		n++
	}
	for _, s := range a.mgr.AllSeeding() {
		s.Cancel()
		n++
	}
	a.reply(c, "Cancelled "+strconv.Itoa(n)+" mirrors.")
	return nil
}

func (a *App) cmdCancelByIndex(c *botapi.Context) error {
	if !a.isOwner(c.Message()) {
		return nil
	}
	idx, err := strconv.Atoi(util.ParseMessageArgs(c.Message().Text))
	if err != nil {
		a.reply(c, "Provide a valid index.")
		return nil
	}
	dl := a.mgr.ByIndex(idx)
	if dl == nil {
		a.reply(c, "No mirror with that index.")
		return nil
	}
	dl.Cancel()
	return nil
}

// --- helpers ---

func (a *App) reply(c *botapi.Context, text string) {
	_, _ = c.Reply(text)
}

func fetchURLBytes(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	_, err = buf.ReadFrom(resp.Body)
	return buf.Bytes(), err
}

func isTorrentDoc(mimeType, fileName string) bool {
	return strings.Contains(strings.ToLower(mimeType), "bittorrent") ||
		strings.HasSuffix(strings.ToLower(fileName), ".torrent")
}
