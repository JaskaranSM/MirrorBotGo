package engine

import (
	"MirrorBotGo/utils"
	"errors"
	"fmt"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/shirou/gopsutil/v3/cpu"
)

var StatusMessageStorage map[int64]*gotgbot.Message // chatId : message
var Spinner *ProgressSpinner = getSpinner()
var mutex sync.Mutex
var threadProfile = pprof.Lookup("threadcreate")
var FailedToSendMessageError error = errors.New("failed to send message")

var StatusMessageChunkSize int = utils.GetStatusMessagesPerPage()

func init() {
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

func DeleteMessageByChatId(chatId int64) {
	for i := range StatusMessageStorage {
		if i == chatId {
			delete(StatusMessageStorage, i)
		}
	}
}

func SendMessage(b *gotgbot.Bot, messageText string, message *gotgbot.Message) *gotgbot.Message {
	msg, err := b.SendMessage(
		message.Chat.Id, filterBotToken(messageText), &gotgbot.SendMessageOpts{
			ReplyToMessageId: message.MessageId,
			ParseMode:        "HTML",
		},
	)
	if err != nil {
		L().Errorf("SendMessage: %s | %d | %d : %v", messageText, message.Chat.Id, message.MessageId, err)
	}
	return msg
}

func SendMessageMarkup(b *gotgbot.Bot, messageText string, message *gotgbot.Message, markup gotgbot.InlineKeyboardMarkup) *gotgbot.Message {
	msg, err := b.SendMessage(
		message.Chat.Id, filterBotToken(messageText), &gotgbot.SendMessageOpts{
			ReplyToMessageId: message.MessageId,
			ParseMode:        "HTML",
			ReplyMarkup:      markup,
		},
	)
	if err != nil {
		L().Errorf("SendMessageMarkup: %s | %d | %d | %v", messageText, message.Chat.Id, message.MessageId, err)
	}
	return msg
}

func EditMessage(b *gotgbot.Bot, messageText string, message *gotgbot.Message) *gotgbot.Message {
	m, _, err := b.EditMessageText(
		filterBotToken(messageText), &gotgbot.EditMessageTextOpts{
			ChatId:    message.Chat.Id,
			MessageId: message.MessageId,
			ParseMode: "HTML",
		},
	)
	if err != nil {
		L().Errorf("EditMessage: %s | %d | %d | %v", messageText, message.Chat.Id, message.MessageId, err)
	}
	return m
}

func EditMessageMarkup(b *gotgbot.Bot, messageText string, message *gotgbot.Message, markup gotgbot.InlineKeyboardMarkup) *gotgbot.Message {
	m, _, err := b.EditMessageText(
		filterBotToken(messageText), &gotgbot.EditMessageTextOpts{
			ChatId:      message.Chat.Id,
			MessageId:   message.MessageId,
			ParseMode:   "HTML",
			ReplyMarkup: markup,
		},
	)
	if err != nil {
		L().Errorf("EditMessageMarkup: %s | %d | %d | %v", messageText, message.Chat.Id, message.MessageId, err)
	}
	return m
}

func DeleteMessage(b *gotgbot.Bot, message *gotgbot.Message) bool {
	deleted, err := b.DeleteMessage(message.Chat.Id, message.MessageId, nil)
	if err != nil {
		L().Errorf("DeleteMessage: %d | %d | %v", message.Chat.Id, message.MessageId, err)
	}
	return deleted
}

func DeleteAllMessages(b *gotgbot.Bot) {
	for _, m := range GetAllMessages() {
		DeleteMessage(b, m)
		DeleteMessageByChatId(m.Chat.Id)
	}
}

func GetCpuUsage() string {
	out := ""
	data, err := cpu.Percent(10*time.Millisecond, false)
	if err != nil {
		return "NA"
	}
	out += fmt.Sprintf("%.2f", data[0]) + "%"
	return out
}

func GetStatsString() string {
	var outStr string
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	outStr += fmt.Sprintf("Alloc: %s | ", utils.GetHumanBytes(int64(mem.Alloc)))
	outStr += fmt.Sprintf("TAlloc: %s | ", utils.GetHumanBytes(int64(mem.TotalAlloc)))
	outStr += fmt.Sprintf("GC: %d | ", mem.NumGC)
	outStr += fmt.Sprintf("GR: %d | ", runtime.NumGoroutine())
	outStr += fmt.Sprintf("TH: %d | ", threadProfile.Count())
	outStr += fmt.Sprintf("CPU: %s", GetCpuUsage())
	return outStr
}

func GetReadableProgressMessage(page int) string {
	var globalDownloadSpeed int64
	var globalUploadSpeed int64
	msg := ""
	chunks := GetAllMirrorsChunked(StatusMessageChunkSize)
	if len(chunks)-1 < page {
		page = len(chunks) - 1
	}
	dls := chunks[page]
	for i := 0; i <= len(dls)-1; i++ {
		dl := dls[i]
		msg += fmt.Sprintf("<i>%s</i> -", dl.Name())
		msg += fmt.Sprintf(" %s\n", dl.GetStatusType())
		if dl.GetStatusType() == MirrorStatusDownloading {
			globalDownloadSpeed += dl.Speed()
		}
		if dl.GetStatusType() == MirrorStatusSeeding || dl.GetStatusType() == MirrorStatusUploading {
			globalUploadSpeed += dl.Speed()
		}
		if dl.GetStatusType() == MirrorStatusCloning {
			msg += fmt.Sprintf("%s of ", utils.GetHumanBytes(dl.CompletedLength()))
			msg += fmt.Sprintf("%s at ", utils.GetHumanBytes(dl.TotalLength()))
			msg += fmt.Sprintf("%s/s, ", utils.GetHumanBytes(int64(dl.Speed())))
			msg += fmt.Sprintf("\nGID: <code>%s</code> ", dls[i].Gid())
			msg += fmt.Sprintf("I: <code>%d</code>", dls[i].Index())
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
					msg += fmt.Sprintf("ST: %s", utils.HumanizeDuration(*dl.ETA()))
				} else {
					msg += fmt.Sprintf("ETA: %s", dl.ETA())
				}
			} else {
				msg += "ETA: -"
			}
			if dl.IsTorrent() {
				msg += fmt.Sprintf(" | P: %d | S: %d | PC: %d/%d", dl.GetPeers(), dl.GetSeeders(), dl.PiecesCompleted(), dl.PiecesTotal())
			}
			msg += fmt.Sprintf("\nGID: <code>%s</code> ", dls[i].Gid())
			msg += fmt.Sprintf("I: <code>%d</code>", dls[i].Index())

		}
		msg += "\n\n"
	}
	msg += GetStatsString()
	msg += fmt.Sprintf(" | DL: %s", utils.GetHumanBytes(globalDownloadSpeed))
	msg += fmt.Sprintf(" | UP: %s", utils.GetHumanBytes(globalUploadSpeed))
	return msg
}

func NewKeyboardButtonText(text string, callbackData string) gotgbot.InlineKeyboardButton {
	return gotgbot.InlineKeyboardButton{
		Text:         text,
		CallbackData: callbackData,
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
	var newMsg *gotgbot.Message
	var progress string
	msg := GetMessageByChatId(message.Chat.Id)
	if msg != nil {
		DeleteMessageByChatId(message.Chat.Id)
	}
	go func() {
		if msg != nil {
			DeleteMessage(b, msg)
		}
	}()
	if GetAllMirrorsCount()+GetAllSeedingMirrorsCount() == 0 {
		progress = "No active mirrors"
	} else {
		progress = GetReadableProgressMessage(0)
	}
	if GetAllMirrorsCount() > StatusMessageChunkSize {
		newMsg = SendMessageMarkup(b, progress, message, GetPaginationMarkup(false, true, "0", utils.ParseIntToString(len(GetAllMirrorsChunked(StatusMessageChunkSize))-1)))
		if newMsg == nil {
			return FailedToSendMessageError
		}
	} else {
		newMsg = SendMessage(b, progress, message)
		if newMsg == nil {
			return FailedToSendMessageError
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
			if m != nil {
				DeleteMessage(b, m)
			}
		}
	}()
}

func UpdateAllMessages(b *gotgbot.Bot) {
	mirrorsCount := GetAllMirrorsCount()
	for _, msg := range GetAllMessages() {
		var previous bool
		var next bool
		var progress string
		if GetAllMirrorsCount()+GetAllSeedingMirrorsCount() == 0 {
			progress = "No active mirrors"
			EditMessage(b, progress, msg)
			continue
		} else {
			progress = GetReadableProgressMessage(int(msg.Date))
		}
		if msg.Text != progress {
			chunks := GetAllMirrorsChunked(StatusMessageChunkSize)
			if mirrorsCount > StatusMessageChunkSize {
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
		mutex.Lock()
		UpdateAllMessages(b)
		mutex.Unlock()
		time.Sleep(p.UpdateInterval)
		if GetAllMirrorsCount()+GetAllSeedingMirrorsCount() == 0 {
			DeleteAllMessages(b)
			p.Stop()
			break
		}
	}
}

func (p *ProgressSpinner) Start(b *gotgbot.Bot) {
	if p.isRunning {
		return
	}
	p.isRunning = true
	go p.SpinProgress(b)
}

func (p *ProgressSpinner) Stop() {
	p.isRunning = false
}

func filterBotToken(text string) string {
	return strings.ReplaceAll(text, utils.GetBotToken(), "REDACTED")
}
