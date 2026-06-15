package mirror

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"mirrorbot/internal/archive"
	"mirrorbot/internal/gdrive"
	"mirrorbot/internal/status"
	"mirrorbot/internal/tgbot"
	"mirrorbot/internal/util"
)

// Deps are the shared dependencies a listener needs to run the pipeline.
type Deps struct {
	Ctx             context.Context
	Bot             *tgbot.Bot
	Mgr             *Manager
	Status          *StatusService
	Drive           *gdrive.Client
	DownloadDir     string
	DefaultParentID string
	IndexURL        string
}

// MirrorListener owns one mirror job end-to-end and implements status.Listener.
type MirrorListener struct {
	deps    *Deps
	uid     int64 // the command message id (also the per-mirror download dir name)
	chatID  int64
	replyTo int

	isTar       bool
	isSeed      bool
	doUnArchive bool
	parentID    string
	customParent bool

	mu       sync.Mutex
	canceled bool
	dropFn   func() // optional torrent.Drop, set for torrent mirrors
}

// NewMirrorListener creates a mirror listener.
func NewMirrorListener(deps *Deps, uid, chatID int64, replyTo int, isTar, doUnArchive, isSeed bool, parentID string) *MirrorListener {
	return &MirrorListener{
		deps: deps, uid: uid, chatID: chatID, replyTo: replyTo,
		isTar: isTar, doUnArchive: doUnArchive, isSeed: isSeed, parentID: parentID,
	}
}

// UID returns the mirror's unique id (command message id).
func (m *MirrorListener) UID() int64 { return m.uid }

// SetDropper registers a cleanup function (e.g. torrent.Drop) run before files
// are deleted.
func (m *MirrorListener) SetDropper(fn func()) { m.dropFn = fn }

// OnDownloadStart is invoked by a source once it has begun.
func (m *MirrorListener) OnDownloadStart() {}

// OnDownloadComplete runs the post-download pipeline (tar -> untar -> upload) in
// a new goroutine so the calling source goroutine is not blocked.
func (m *MirrorListener) OnDownloadComplete() {
	go m.runComplete()
}

func (m *MirrorListener) runComplete() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("pipeline panic: uid=%d: %v", m.uid, r)
			m.OnUploadError(fmt.Errorf("internal error: %v", r))
		}
	}()
	ctx := m.deps.Ctx
	dl := m.deps.Mgr.Get(m.uid)
	if dl == nil {
		return
	}
	name := dl.Name()
	gid := dl.GID()
	idx := dl.Index()
	p := dl.Path()

	if m.isSeed {
		m.deps.Mgr.MoveToSeeding(m.uid)
	}

	if m.isTar {
		tar := archive.NewTarArchiver(name, gid)
		tar.SetIndex(idx)
		m.deps.Mgr.Set(m.uid, tar)
		np, err := tar.Tar(p)
		if err != nil {
			m.notify(fmt.Sprintf("Failed to archive the contents, uploading as is: %s\nERR: %s", name, err))
		} else {
			p = np
		}
	}

	if m.doUnArchive && archive.Supported(p) {
		ua := archive.NewUnArchiver(name, gid)
		ua.SetIndex(idx)
		m.deps.Mgr.Set(m.uid, ua)
		np, err := ua.Extract(p)
		if err != nil {
			m.notify(fmt.Sprintf("Failed to unarchive the contents, uploading as is: %s\nERR: %s", name, err))
		} else {
			p = np
		}
	}

	parentID := m.deps.DefaultParentID
	if m.parentID != "" {
		if _, err := m.deps.Drive.Metadata(ctx, m.parentID); err == nil {
			parentID = m.parentID
			m.customParent = true
		}
	}

	tr := m.deps.Drive.NewTransfer(status.Uploading, util.RandGID(16))
	tr.SetIndex(idx)
	m.deps.Mgr.Set(m.uid, tr)
	fileID, err := tr.Upload(ctx, p, parentID)
	if err != nil {
		log.Printf("upload failed: uid=%d name=%q: %v", m.uid, name, err)
		m.OnUploadError(err)
		return
	}
	m.OnUploadComplete(fileID)
}

// OnDownloadError handles a failed/cancelled download.
func (m *MirrorListener) OnDownloadError(err error) {
	m.mu.Lock()
	if m.canceled {
		m.mu.Unlock()
		return
	}
	m.canceled = true
	m.mu.Unlock()

	log.Printf("download stopped: uid=%d: %v", m.uid, err)
	dl := m.deps.Mgr.Get(m.uid)
	if dl != nil {
		m.clean()
	}
	m.notify(fmt.Sprintf("Your download has been stopped due to: %s", err))
	if dl != nil {
		m.cleanDownload()
	}
}

// OnUploadComplete formats the shareable link and finalizes the mirror.
func (m *MirrorListener) OnUploadComplete(fileID string) {
	dl := m.deps.Mgr.Get(m.uid)
	if dl == nil {
		return
	}
	link := strings.ReplaceAll("https://drive.google.com/open?id="+fileID, "'", "")
	msg := fmt.Sprintf("<a href='%s'>%s</a> (%s)", link, dl.Name(), util.GetHumanBytes(dl.TotalLength()))
	if m.deps.IndexURL != "" {
		if m.customParent {
			msg += "\n\nShareable Link: Mirror belongs to a custom parentId"
		} else {
			in := fmt.Sprintf("%s/%s", m.deps.IndexURL, dl.Name())
			if util.IsPathDir(dl.Path()) {
				in += "/"
			}
			msg += fmt.Sprintf("\n\nShareable Link: <a href='%s'>here</a>", in)
		}
	}

	if m.isSeed {
		m.restoreSeeding(dl.Index())
	} else {
		m.clean()
	}
	m.notify(msg)
	if !m.isSeed {
		m.cleanDownload()
	}
}

// OnUploadError handles a failed upload.
func (m *MirrorListener) OnUploadError(err error) {
	dl := m.deps.Mgr.Get(m.uid)
	if m.isSeed && dl != nil {
		m.restoreSeeding(dl.Index())
	} else {
		m.clean()
	}
	m.notify(fmt.Sprintf("Your upload has been stopped due to: %s", err))
	if !m.isSeed {
		m.cleanDownload()
	}
}

// restoreSeeding moves the parked torrent status back into the active list so it
// displays as Seeding.
func (m *MirrorListener) restoreSeeding(idx int) {
	seed := m.deps.Mgr.GetSeeding(m.uid)
	if seed == nil {
		return
	}
	seed.SetIndex(idx)
	m.deps.Mgr.Set(m.uid, seed)
	m.deps.Mgr.RemoveSeeding(m.uid)
}

func (m *MirrorListener) clean() {
	m.deps.Mgr.MoveToCancel(m.uid)
}

func (m *MirrorListener) cleanDownload() {
	if m.dropFn != nil {
		m.dropFn()
	}
	dir := filepath.Join(m.deps.DownloadDir, strconv.FormatInt(m.uid, 10))
	_ = util.RemoveByPath(dir)
}

func (m *MirrorListener) notify(text string) {
	_, _ = m.deps.Bot.SendHTML(m.deps.Ctx, m.chatID, text, m.replyTo, nil)
}

// CloneListener drives a Drive-to-Drive clone (no local download phase).
type CloneListener struct {
	deps     *Deps
	uid      int64
	chatID   int64
	replyTo  int
	parentID string
}

// NewCloneListener creates a clone listener targeting parentID.
func NewCloneListener(deps *Deps, uid, chatID int64, replyTo int, parentID string) *CloneListener {
	return &CloneListener{deps: deps, uid: uid, chatID: chatID, replyTo: replyTo, parentID: parentID}
}

// Start begins cloning srcID into the destination folder.
func (c *CloneListener) Start(srcID string) *gdrive.Transfer {
	tr := c.deps.Drive.NewTransfer(status.Cloning, util.RandGID(16))
	tr.SetIndex(c.deps.Mgr.NextIndex())
	c.deps.Mgr.Set(c.uid, tr)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("clone panic: uid=%d: %v", c.uid, r)
				c.onError(fmt.Errorf("internal error: %v", r))
			}
		}()
		newID, err := tr.Clone(c.deps.Ctx, srcID, c.parentID)
		if err != nil {
			log.Printf("clone failed: uid=%d: %v", c.uid, err)
			c.onError(err)
			return
		}
		c.onComplete(newID, tr)
	}()
	return tr
}

func (c *CloneListener) onComplete(fileID string, dl *gdrive.Transfer) {
	link := strings.ReplaceAll("https://drive.google.com/open?id="+fileID, "'", "")
	msg := fmt.Sprintf("<a href='%s'>%s</a> (%s)", link, dl.Name(), util.GetHumanBytes(dl.TotalLength()))
	c.deps.Mgr.MoveToCancel(c.uid)
	_, _ = c.deps.Bot.SendHTML(c.deps.Ctx, c.chatID, msg, c.replyTo, nil)
}

func (c *CloneListener) onError(err error) {
	c.deps.Mgr.MoveToCancel(c.uid)
	_, _ = c.deps.Bot.SendHTML(c.deps.Ctx, c.chatID, fmt.Sprintf("Your clone has been stopped due to: %s", err), c.replyTo, nil)
}
