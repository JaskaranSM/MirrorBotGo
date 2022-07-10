package engine

import (
	"MirrorBotGo/utils"
	"fmt"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

var StatusMessageStorage map[int64]*gotgbot.Message // chatId : message
var Spinner *ProgressSpinner = getSpinner()
var mutex sync.Mutex

var STATUS_MESSAGE_CHUNKSIZE int = utils.GetStatusMessagesPerPage()

func Init() {
	StatusMessageStorage = make(map[int64]*gotgbot.Message)
}

func getSpinner() *ProgressSpinner {
	return &ProgressSpinner{isRunning: false, UpdateInterval: utils.GetStatusUpdateInterval()}
}

func AddStatusMessage(message *gotgbot.Message) {
	StatusMessageStorage[message.Chat.Id] = message
}

func GetAllMessages() []*gotgbot.Message {
	var msgs []*gotgbot.Message
	for _, m := range StatusMessageStorage {
		msgs = append(msgs, m)
	}
	return msgs
}

func GetMessageByChatId(chatId int64) *gotgbot.Message {
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

func DeleteMessageByChatId(chatId int64) {
	for i := range StatusMessageStorage {
		if i == chatId {
			delete(StatusMessageStorage, i)
		}
	}
}

func SendMessage(b *gotgbot.Bot, messageText string, message *gotgbot.Message) (*gotgbot.Message, error) {
	return b.SendMessage(
		message.Chat.Id, messageText, &gotgbot.SendMessageOpts{
			ReplyToMessageId: message.MessageId,
			ParseMode:        "HTML",
		},
	)
}

func SendMessageMarkup(b *gotgbot.Bot, messageText string, message *gotgbot.Message, markup gotgbot.InlineKeyboardMarkup) (*gotgbot.Message, error) {
	return b.SendMessage(
		message.Chat.Id, messageText, &gotgbot.SendMessageOpts{
			ReplyToMessageId: message.MessageId,
			ParseMode:        "HTML",
			ReplyMarkup:      markup,
		},
	)
}

func EditMessage(b *gotgbot.Bot, messageText string, message *gotgbot.Message) (*gotgbot.Message, error) {
	m, _, err := b.EditMessageText(
		messageText, &gotgbot.EditMessageTextOpts{
			ChatId:    message.Chat.Id,
			MessageId: message.MessageId,
			ParseMode: "HTML",
		},
	)
	return m, err
}

func EditMessageMarkup(b *gotgbot.Bot, messageText string, message *gotgbot.Message, markup gotgbot.InlineKeyboardMarkup) (*gotgbot.Message, error) {
	m, _, err := b.EditMessageText(
		messageText, &gotgbot.EditMessageTextOpts{
			ChatId:      message.Chat.Id,
			MessageId:   message.MessageId,
			ParseMode:   "HTML",
			ReplyMarkup: markup,
		},
	)
	return m, err
}

func DeleteMessage(b *gotgbot.Bot, message *gotgbot.Message) (bool, error) {
	return b.DeleteMessage(message.Chat.Id, message.MessageId)
}

func DeleteAllMessages(b *gotgbot.Bot) {
	for _, m := range GetAllMessages() {
		DeleteMessage(b, m)
		DeleteMessageByChatId(m.Chat.Id)
	}
}

func GetReadableProgressMessage(page int) string {
	msg := ""
	chunks := GetAllMirrorsChunked(STATUS_MESSAGE_CHUNKSIZE)
	if len(chunks)-1 < page {
		page = len(chunks) - 1
	}
	dls := chunks[page]
	for i := 0; i <= len(dls)-1; i++ {
		dl := dls[i]
		msg += fmt.Sprintf("<i>%s</i> -", dl.Name())
		msg += fmt.Sprintf(" %s\n", dl.GetStatusType())
		if dl.GetStatusType() == MirrorStatusCloning {
			msg += fmt.Sprintf("%s of ", utils.GetHumanBytes(dl.CompletedLength()))
			msg += fmt.Sprintf("%s at ", utils.GetHumanBytes(dl.TotalLength()))
			msg += fmt.Sprintf("%s/s, ", utils.GetHumanBytes(int64(dl.Speed())))
			msg += fmt.Sprintf("\nGID: <code>%s</code>", dls[i].Gid())
			msg += "\n\n"
			continue
		}
		if dl.GetStatusType() != "bruh" {
			msg += fmt.Sprintf("<code>%s %.2f%% </code>", utils.GetProgressBarString(int(dl.CompletedLength()), int(dl.TotalLength())), dl.Percentage())
			msg += fmt.Sprintf(", %s of ", utils.GetHumanBytes(dl.CompletedLength()))
			msg += fmt.Sprintf("%s at ", utils.GetHumanBytes(dl.TotalLength()))
			msg += fmt.Sprintf("%s/s, ", utils.GetHumanBytes(int64(dl.Speed())))
			if dl.ETA() != nil {
				if dl.GetStatusType() == MirrorStatusSeeding && dl.CompletedLength() > dl.TotalLength() {
					msg += fmt.Sprintf("ST: %s", dl.ETA())
				} else {
					msg += fmt.Sprintf("ETA: %s", dl.ETA())
				}
			} else {
				msg += "ETA: -"
			}
			if dl.IsTorrent() {
				msg += fmt.Sprintf(" | P: %d | S: %d", dl.GetPeers(), dl.GetSeeders())
			}
			msg += fmt.Sprintf("\nGID: <code>%s</code>", dls[i].Gid())

		}
		msg += "\n\n"
	}
	return msg
}

func NewKeyboardButtonText(text string, callback_data string) gotgbot.InlineKeyboardButton {
	return gotgbot.InlineKeyboardButton{
		Text:         text,
		CallbackData: callback_data,
	}
}

func GetPaginationMarkup(previous bool, next bool, prString string, nxString string) gotgbot.InlineKeyboardMarkup {
	var markup gotgbot.InlineKeyboardMarkup
	var modulesMatrix [][]gotgbot.InlineKeyboardButton
	var modules []gotgbot.InlineKeyboardButton
	if previous {
		modules = append(modules, NewKeyboardButtonText(fmt.Sprint("First", ""), "first"))
		modules = append(modules, NewKeyboardButtonText(fmt.Sprintf("<=(%s)", prString), "previous"))
	}
	if next {
		modules = append(modules, NewKeyboardButtonText(fmt.Sprintf("=>(%s)", nxString), "next"))
		modules = append(modules, NewKeyboardButtonText(fmt.Sprint("Last", ""), "last"))
	}
	modulesMatrix = append(modulesMatrix, modules)
	markup.InlineKeyboard = modulesMatrix
	return markup
}

func SendStatusMessage(b *gotgbot.Bot, message *gotgbot.Message) error {
	mutex.Lock()
	defer mutex.Unlock()
	var err error
	var newMsg *gotgbot.Message
	var progress string
	msg := GetMessageByChatId(message.Chat.Id)
	if msg != nil {
		DeleteMessage(b, msg)
		DeleteMessageByChatId(message.Chat.Id)
	}

	progress = GetReadableProgressMessage(0)
	if GetAllMirrorsCount() > STATUS_MESSAGE_CHUNKSIZE {
		newMsg, err = SendMessageMarkup(b, progress, message, GetPaginationMarkup(false, true, "0", utils.ParseIntToString(len(GetAllMirrorsChunked(STATUS_MESSAGE_CHUNKSIZE))-1)))
		if err != nil {
			L().Error(err)
			return err
		}
	} else {
		newMsg, err = SendMessage(b, progress, message)
		if err != nil {
			L().Error(err)
			return err
		}
	}
	newMsg.Date = 0
	AddStatusMessage(newMsg)
	return nil
}

func AutoDeleteMessages(b *gotgbot.Bot, timeout time.Duration, messages ...*gotgbot.Message) {
	go func() {
		time.Sleep(timeout)
		for _, m := range messages {
			DeleteMessage(b, m)
		}
	}()
}

func UpdateAllMessages(b *gotgbot.Bot) {
	mirrorsCount := GetAllMirrorsCount()
	for _, msg := range GetAllMessages() {
		var previous bool
		var next bool
		progress := GetReadableProgressMessage(int(msg.Date))
		chunks := GetAllMirrorsChunked(STATUS_MESSAGE_CHUNKSIZE)
		if msg.Text != progress {
			if mirrorsCount > STATUS_MESSAGE_CHUNKSIZE {
				if msg.Date > int64(len(chunks)) {
					msg.Date = int64(len(chunks)) - 1
				}
				if msg.Date > 0 {
					previous = true
				}
				if len(chunks) > int(msg.Date)+1 {
					next = true
				}
				EditMessageMarkup(b, progress, msg, GetPaginationMarkup(previous, next, utils.ParseInt64ToString(msg.Date), utils.ParseIntToString(len(chunks)-int(msg.Date)-1)))
			} else {
				EditMessage(b, progress, msg)
			}
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

func (p *ProgressSpinner) SpinProgress(b *gotgbot.Bot) {
	for p.IsRunning() {
		if GetAllMirrorsCount()+GetAllSeedingMirrorsCount() == 0 {
			DeleteAllMessages(b)
			p.Stop()
			break
		}
		UpdateAllMessages(b)
		time.Sleep(p.UpdateInterval)
	}
}

func (p *ProgressSpinner) Start(b *gotgbot.Bot) {
	p.isRunning = true
	go p.SpinProgress(b)
}

func (p *ProgressSpinner) Stop() {
	p.isRunning = false
}
