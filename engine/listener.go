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
	Update *gotgbot.Update
	bot    ext.Bot
	isTar  bool
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
	MoveMirrorToCancel(m.GetUid(), GetMirrorByUid(m.GetUid()))
	RemoveMirrorLocal(m.GetUid())
	if GetAllMirrorsCount() == 0 {
		DeleteAllMessages(m.bot)
	}
	UpdateAllMessages(m.bot)
}

func (m *MirrorListener) OnDownloadComplete() {
	dl := m.GetDownload()
	name := dl.Name()
	size := dl.TotalLength()
	path := dl.Path()
	log.Printf("[DownloadComplete]: %s (%d)\n", name, size)
	if m.isTar {
		tarStatus := NewTarStatus(dl.Gid(), dl.Name(), nil)
		tarStatus.Index_ = dl.Index()
		AddMirrorLocal(m.GetUid(), tarStatus)
		path = TarPath(path)
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
	utils.RemoveByPath(dl.Path())
}

func (m *MirrorListener) OnUploadComplete(link string) {
	dl := m.GetDownload()
	name := dl.Name()
	size := dl.TotalLength()
	log.Printf("[UploadComplete]: %s (%d)\n", name, size)
	msg := fmt.Sprintf("<a href='%s'>%s</a> (%s)", link, dl.Name(), utils.GetHumanBytes(dl.TotalLength()))
	m.Clean()
	SendMessage(m.bot, msg, m.Update.Message)
	utils.RemoveByPath(path.Join(utils.GetDownloadDir(), dl.Gid()))
}

func NewMirrorListener(b ext.Bot, update *gotgbot.Update, isTar bool) MirrorListener {
	return MirrorListener{bot: b, Update: update, isTar: isTar}
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
