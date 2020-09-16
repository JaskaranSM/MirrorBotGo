package engine

import (
	"MirrorBotGo/utils"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/ext"
)

var StatusMessageStorage map[int]*ext.Message // chatId : message
var Spinner *ProgressSpinner = getSpinner()
var mutex sync.Mutex

func Init() {
	StatusMessageStorage = make(map[int]*ext.Message)
}

func getSpinner() *ProgressSpinner {
	return &ProgressSpinner{isRunning: false, UpdateInterval: utils.GetStatusUpdateInterval()}
}

func AddStatusMessage(message *ext.Message) {
	StatusMessageStorage[message.Chat.Id] = message
}

func GetAllMessages() []*ext.Message {
	var msgs []*ext.Message
	for _, m := range StatusMessageStorage {
		msgs = append(msgs, m)
	}
	return msgs
}

func GetMessageByChatId(chatId int) *ext.Message {
	for i, m := range StatusMessageStorage {
		if i == chatId {
			return m
		}
	}
	return nil
}

func GetAllMessagesCount() int {
	return len(GetAllMessages())
}

func DeleteMessageByChatId(chatId int) {
	for i, _ := range StatusMessageStorage {
		if i == chatId {
			delete(StatusMessageStorage, i)
		}
	}
}

func SendMessage(b ext.Bot, messageText string, message *ext.Message) (*ext.Message, error) {
	return b.ReplyHTML(message.Chat.Id, messageText, message.MessageId)
}

func EditMessage(b ext.Bot, messageText string, message *ext.Message) (*ext.Message, error) {
	return b.EditMessageHTML(message.Chat.Id, message.MessageId, messageText)
}

func DeleteMessage(b ext.Bot, message *ext.Message) (bool, error) {
	return b.DeleteMessage(message.Chat.Id, message.MessageId)
}

func DeleteAllMessages(b ext.Bot) {
	for _, m := range GetAllMessages() {
		DeleteMessage(b, m)
		DeleteMessageByChatId(m.Chat.Id)
	}
}

func GetReadableProgressMessage() string {
	msg := ""
	dls := GetAllMirrors()
	for i := 0; i <= len(dls)-1; i++ {
		msg += fmt.Sprintf("<i>%s</i> -", dls[i].Name())
		msg += fmt.Sprintf(" %s\n", dls[i].GetStatusType())
		if dls[i].GetStatusType() != MirrorStatusArchiving {
			msg += fmt.Sprintf("<code>%s %.2f%%</code>", utils.GetProgressBarString(int(dls[i].CompletedLength()), int(dls[i].TotalLength())), dls[i].Percentage())
			msg += fmt.Sprintf(" , %s of ", utils.GetHumanBytes(dls[i].CompletedLength()))
			msg += fmt.Sprintf("%s at ", utils.GetHumanBytes(dls[i].TotalLength()))
			msg += fmt.Sprintf("%s/s, ", utils.GetHumanBytes(int64(dls[i].Speed())))
			if dls[i].ETA() != nil {
				msg += fmt.Sprintf("ETA: %s", utils.HumanizeDuration(*dls[i].ETA()))
			} else {
				msg += "ETA: -"
			}
			msg += fmt.Sprintf("\nGID: <code>%s</code>", dls[i].Gid())
		}
		msg += "\n\n"
	}
	return msg
}

func SendStatusMessage(b ext.Bot, message *ext.Message) error {
	mutex.Lock()
	defer mutex.Unlock()
	progress := GetReadableProgressMessage()
	msg := GetMessageByChatId(message.Chat.Id)
	if msg != nil {
		DeleteMessage(b, msg)
		DeleteMessageByChatId(message.Chat.Id)
	}
	newMsg, err := SendMessage(b, progress, message)
	if err != nil {
		log.Println(err)
		return err
	}
	AddStatusMessage(newMsg)
	return nil
}

func AutoDeleteMessages(b ext.Bot, timeout time.Duration, messages ...*ext.Message) {
	go func() {
		time.Sleep(timeout)
		for _, m := range messages {
			DeleteMessage(b, m)
		}
	}()
}

func UpdateAllMessages(b ext.Bot) {
	progress := GetReadableProgressMessage()
	for _, msg := range GetAllMessages() {
		if msg.Text != progress {
			EditMessage(b, progress, msg)
			msg.Text = progress
		}
	}
}

type ProgressSpinner struct {
	isRunning      bool
	UpdateInterval time.Duration
}

func (p *ProgressSpinner) IsRunning() bool {
	return p.isRunning
}

func (p *ProgressSpinner) SpinProgress(b ext.Bot) {
	for p.IsRunning() {
		if GetAllMirrorsCount() == 0 {
			DeleteAllMessages(b)
			p.Stop()
			break
		}
		UpdateAllMessages(b)
		time.Sleep(p.UpdateInterval)
	}
}

func (p *ProgressSpinner) Start(b ext.Bot) {
	p.isRunning = true
	go p.SpinProgress(b)
}

func (p *ProgressSpinner) Stop() {
	p.isRunning = false
}
