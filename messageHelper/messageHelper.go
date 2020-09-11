package messageHelper

import (
	"MirrorBotGo/mirrorManager"
	"MirrorBotGo/utils"
	"fmt"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/ext"
)

var mutex sync.Mutex
var StatusMessageStorage map[int]*ext.Message // chatId : message
var Spinner *ProgressSpinner = getSpinner()

func Init() {
	StatusMessageStorage = make(map[int]*ext.Message)
}

func getSpinner() *ProgressSpinner {
	return &ProgressSpinner{}
}

func AddStatusMessage(message *ext.Message) {
	mutex.Lock()
	StatusMessageStorage[message.Chat.Id] = message
	mutex.Unlock()
}

func GetAllMessages() []*ext.Message {
	var msgs []*ext.Message
	mutex.Lock()
	for _, m := range StatusMessageStorage {
		msgs = append(msgs, m)
	}
	mutex.Unlock()
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
	mutex.Lock()
	for i, _ := range StatusMessageStorage {
		if i == chatId {
			delete(StatusMessageStorage, i)
		}
	}
	mutex.Unlock()
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
	}
}

func GetReadableProgressMessage() string {
	msg := ""
	for _, dl := range mirrorManager.GetAllMirrors() {
		msg += fmt.Sprintf("<i>%s</i> -", dl.Name())
		msg += fmt.Sprintf(" %s\n", dl.GetStatusType())
		msg += fmt.Sprintf("%s of ", utils.GetHumanBytes(dl.CompletedLength()))
		msg += fmt.Sprintf("%s at ", utils.GetHumanBytes(dl.TotalLength()))
		msg += fmt.Sprintf("%s/s, ", utils.GetHumanBytes(int64(dl.Speed())))
		if dl.ETA() != nil {
			msg += fmt.Sprintf("ETA - %s", dl.ETA())
		} else {
			msg += "ETA: -"
		}
		msg += "\n\n"
	}
	return msg
}

func SendStatusMessage(b ext.Bot, message *ext.Message) error {
	progress := GetReadableProgressMessage()
	msg := GetMessageByChatId(message.Chat.Id)
	if msg != nil {
		DeleteMessage(b, msg)
		DeleteMessageByChatId(message.Chat.Id)
	}
	newMsg, err := SendMessage(b, progress, message)
	if err != nil {
		return err
	}
	AddStatusMessage(newMsg)
	return nil
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
	isRunning bool
}

func (p *ProgressSpinner) IsRunning() bool {
	return p.isRunning
}

func (p *ProgressSpinner) SpinProgress(b ext.Bot) {
	for p.IsRunning() {
		if mirrorManager.GetAllMirrorsCount() == 0 {
			DeleteAllMessages(b)
			p.Stop()
			break
		}
		UpdateAllMessages(b)
		time.Sleep(utils.GetSleepTime())
	}
}

func (p *ProgressSpinner) Start(b ext.Bot) {
	p.isRunning = true
	go p.SpinProgress(b)
}

func (p *ProgressSpinner) Stop() {
	p.isRunning = false
}
