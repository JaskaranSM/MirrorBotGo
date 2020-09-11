package mirrorManager

import (
	"MirrorBotGo/utils"
	"fmt"
	"log"
	"time"

	"github.com/PaulSonOfLars/gotgbot"
	"github.com/PaulSonOfLars/gotgbot/ext"
)

type MirrorListener struct {
	Update *gotgbot.Update
	bot    ext.Bot
}

func (m *MirrorListener) GetUid() int {
	return m.Update.Message.MessageId
}

func (m *MirrorListener) GetDownload() MirrorStatus {
	return GetMirrorByUid(m.GetUid())
}

func (m *MirrorListener) OnDownloadStart(text string) {
	log.Println(text)
}

func (m *MirrorListener) OnDownloadComplete() {
	dl := m.GetDownload()
	name := dl.Name()
	size := dl.TotalLength()
	log.Printf("[DownloadComplete]: %s (%d)\n", name, size)
	RemoveMirrorLocal(m.GetUid())
	if GetAllMirrorsCount() == 0 {
		DeleteAllMessages(m.bot)
	}
	SendMessage(m.bot, fmt.Sprintf("Download Completed: %s | %s", name, utils.GetHumanBytes(size)), m.Update.Message)
}
func (m *MirrorListener) OnDownloadError(err string) {
	dl := m.GetDownload()
	name := dl.Name()
	size := dl.TotalLength()
	log.Printf("[DownloadError]: %s (%d)\n", name, size)
}

func NewMirrorListener(b ext.Bot, update *gotgbot.Update) MirrorListener {
	return MirrorListener{bot: b, Update: update}
}

type MirrorStatus interface {
	Name() string
	CompletedLength() int64
	TotalLength() int64
	Speed() int
	ETA() *time.Duration
	Gid() string
	GetStatusType() string
	GetListener() *MirrorListener
}
