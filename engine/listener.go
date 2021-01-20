package engine

import (
	"MirrorBotGo/utils"
	"fmt"
	"log"
	"path"
	"time"

	"github.com/PaulSonOfLars/gotgbot"
	"github.com/PaulSonOfLars/gotgbot/ext"
)

type MirrorListener struct {
	Update      *gotgbot.Update
	bot         ext.Bot
	isTar       bool
	doUnArchive bool
	isCanceled  bool
}

func (m *MirrorListener) GetUid() int {
	return m.Update.Message.MessageId
}

func (m *MirrorListener) GetDownload() MirrorStatus {
	return GetMirrorByUid(m.GetUid())
}

func (m *MirrorListener) OnDownloadStart(text string) {
	log.Println(text)
	UpdateAllMessages(m.bot)
}

func (m *MirrorListener) Clean() {
	if GetAllMirrorsCount() == 0 {
		DeleteAllMessages(m.bot)
	}
	UpdateAllMessages(m.bot)
	MoveMirrorToCancel(m.GetUid(), GetMirrorByUid(m.GetUid()))
	RemoveMirrorLocal(m.GetUid())
}

func (m *MirrorListener) OnDownloadComplete() {
	dl := m.GetDownload()
	name := dl.Name()
	size := dl.TotalLength()
	path := dl.Path()
	log.Printf("[DownloadComplete]: %s (%d)\n", name, size)
	if m.isTar {
		archiver := NewTarArchiver(NewProgress(), dl.TotalLength())
		tarStatus := NewTarStatus(dl.Gid(), dl.Name(), nil, archiver)
		tarStatus.Index_ = dl.Index()
		AddMirrorLocal(m.GetUid(), tarStatus)
		path = archiver.TarPath(path)
	}
	if m.doUnArchive {
		cntSize, err := GetArchiveContentSize(path)
		if err != nil {
			log.Printf("cannot get archive content size: %v,uploading without unarchive\n", err)
		} else {
			size = cntSize
			unarchiver := NewUnArchiver(NewProgress(), size)
			unArchiverStatus := NewUnArchiverStatus(dl.Gid(), dl.Name(), nil, unarchiver)
			unArchiverStatus.Index_ = dl.Index()
			AddMirrorLocal(m.GetUid(), unArchiverStatus)
			path = unarchiver.UnArchivePath(path)
		}
	}
	drive := NewGDriveClient(size, dl.GetListener())
	drive.Init("")
	drive.Authorize()
	driveStatus := NewGoogleDriveStatus(drive, utils.GetFileBaseName(path), dl.Gid())
	driveStatus.Index_ = dl.Index()
	AddMirrorLocal(m.GetUid(), driveStatus)
	UpdateAllMessages(m.bot)
	drive.Upload(path)
}
func (m *MirrorListener) OnDownloadError(err string) {
	if m.isCanceled {
		return
	}
	m.isCanceled = true
	fmt.Println("DownloadError: " + err)
	dl := m.GetDownload()
	name := dl.Name()
	size := dl.TotalLength()
	log.Printf("[DownloadError]: %s (%d)\n", name, size)
	m.Clean()
	msg := "Your download has been stopped due to: %s"
	SendMessage(m.bot, fmt.Sprintf(msg, err), m.Update.Message)
	utils.RemoveByPath(dl.Path())
}

func (m *MirrorListener) OnUploadError(err string) {
	dl := m.GetDownload()
	name := dl.Name()
	size := dl.TotalLength()
	log.Printf("[UploadError]: %s (%d)\n", name, size)
	m.Clean()
	msg := "Your upload has been stopped due to: %s"
	SendMessage(m.bot, fmt.Sprintf(msg, err), m.Update.Message)
	utils.RemoveByPath(path.Join(utils.GetDownloadDir(), utils.ParseIntToString(m.GetUid())))
}

func (m *MirrorListener) OnUploadComplete(link string) {
	dl := m.GetDownload()
	name := dl.Name()
	size := dl.TotalLength()
	log.Printf("[UploadComplete]: %s (%d)\n", name, size)
	msg := fmt.Sprintf("<a href='%s'>%s</a> (%s)", link, dl.Name(), utils.GetHumanBytes(dl.TotalLength()))
	in_url := utils.GetIndexUrl()
	if in_url != "" {
		in_url = in_url + "/" + name
		if utils.IsPathDir(dl.Path()) {
			in_url += "/"
		}
		msg += fmt.Sprintf("\n\n Shareable Link: <a href='%s'>here</a>", in_url)
	}
	SendMessage(m.bot, msg, m.Update.Message)
	rmpath := path.Join(utils.GetDownloadDir(), utils.ParseIntToString(m.GetUid()))
	m.Clean()
	utils.RemoveByPath(rmpath)
}

func NewMirrorListener(b ext.Bot, update *gotgbot.Update, isTar bool, doUnArchive bool) MirrorListener {
	return MirrorListener{bot: b, Update: update, isTar: isTar, doUnArchive: doUnArchive}
}

type MirrorStatus interface {
	Name() string
	CompletedLength() int64
	TotalLength() int64
	Speed() int64
	ETA() *time.Duration
	Gid() string
	Path() string
	Percentage() float32
	GetStatusType() string
	Index() int
	GetListener() *MirrorListener
	CancelMirror() bool
}
