package engine

import (
	"MirrorBotGo/utils"
	"errors"
	"fmt"
	"log"
	"os"
	pathlib "path"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/Arman92/go-tdlib"
)

var tgMtProtoClient *tdlib.Client = getMtProtoClient()
var tgDownloader *TgMtprotoDownloader = getTgMtprotoDownloader()

func getTgMtprotoDownloader() *TgMtprotoDownloader {
	return &TgMtprotoDownloader{}
}

func getMtProtoClient() *tdlib.Client {
	tdlib.SetLogVerbosityLevel(1)
	tdlib.SetFilePath("./errors.txt")
	client := tdlib.NewClient(tdlib.Config{
		APIID:               utils.GetTgAppId(),
		APIHash:             utils.GetTgAppHash(),
		SystemLanguageCode:  "en",
		DeviceModel:         "Go brrrr",
		SystemVersion:       "1.0.0",
		ApplicationVersion:  "1.0.0",
		UseMessageDatabase:  false,
		UseFileDatabase:     false,
		UseChatInfoDatabase: false,
		UseTestDataCenter:   false,
		DatabaseDirectory:   "./tdlib-db",
		FileDirectory:       utils.GetDownloadDir(),
		IgnoreFileNames:     false,
	})
	for {
		currentState, err := client.Authorize()
		if err != nil {
			log.Println("TG AUTH ERROR: ", err.Error())
			return client
		}
		if currentState.GetAuthorizationStateEnum() == tdlib.AuthorizationStateWaitPhoneNumberType {
			_, err := client.CheckAuthenticationBotToken(utils.GetBotToken())
			if err != nil {
				fmt.Printf("Error check bot token: %v", err)
				return nil
			}
		} else if currentState.GetAuthorizationStateEnum() == tdlib.AuthorizationStateReadyType {
			fmt.Println("MtProto Client Connected: ", utils.GetBotToken())
			break
		}
	}
	return client
}

func NewTgMtProtoListener(fileId int32, listener *MirrorListener) *TgMtProtoListener {
	now := time.Now()
	return &TgMtProtoListener{
		FileId:    fileId,
		listener:  listener,
		startTime: now,
	}
}

type TgMtProtoListener struct {
	FileId        int32
	current       int64
	total         int64
	speed         int64
	isQueued      bool
	uid           int64
	eta           time.Duration
	startTime     time.Time
	path          string
	eventReceiver *tdlib.EventReceiver
	listener      *MirrorListener
}

func (t *TgMtProtoListener) SetCurrent(current int64) {
	t.current = current
}

func (t *TgMtProtoListener) SetTotal(total int64) {
	t.total = total
}

func (t *TgMtProtoListener) OnDownloadComplete(fileId int32, path string) {
	newPath := pathlib.Join(utils.GetDownloadDir(), utils.ParseInt64ToString(t.uid))
	log.Printf("[MtprotoOnDownloadComplete]: %d | %s | %d\n", fileId, path, t.eventReceiver.ID)
	os.MkdirAll(newPath, 0755)
	newPath = pathlib.Join(newPath, utils.GetFileBaseName(path))
	os.Rename(path, newPath)
	t.path = newPath
	t.listener.OnDownloadComplete()
}

func (t *TgMtProtoListener) OnDownloadProgress(fileId int32, current int64, total int64, path string, isQueued bool) {
	t.path = path
	t.current = current
	t.total = total
	t.isQueued = isQueued
	now := time.Now()
	diff := int64(now.Sub(t.startTime).Seconds())
	if diff != 0 {
		t.speed = current / diff
	} else {
		t.speed = 0
	}
	if t.speed != 0 {
		t.eta = utils.CalculateETA(total-current, t.speed)
	} else {
		t.eta = time.Duration(0)
	}
}

type TgMtprotoDownloader struct {
	IsListenerRunning bool
}

func GetFileIdByMessageContent(content tdlib.MessageContent) (string, int32) {
	var fileId int32
	var name string = "unknown"
	switch content.GetMessageContentEnum() {
	case tdlib.MessageAudioType:
		audio := (content).(*tdlib.MessageAudio)
		name = audio.Audio.FileName
		fileId = audio.Audio.Audio.ID
	case tdlib.MessageVideoType:
		video := (content).(*tdlib.MessageVideo)
		name = video.Video.FileName
		fileId = video.Video.Video.ID
	case tdlib.MessageDocumentType:
		document := (content).(*tdlib.MessageDocument)
		name = document.Document.FileName
		fileId = document.Document.Document.ID
	}
	return name, fileId
}

func (t *TgMtprotoDownloader) AddDownload(msg *gotgbot.Message, listener *MirrorListener) error {
	log.Println("Adding Telegram Download.")
	tgMsg, err := tgMtProtoClient.GetMessage(int64(msg.Chat.Id), int64(msg.MessageId)*1048576)
	if err != nil {
		return err
	}
	content := tgMsg.Content
	name, fileId := GetFileIdByMessageContent(content)
	if fileId == 0 {
		return errors.New("Not a downloadable content")
	}
	mtprotoListener := NewTgMtProtoListener(fileId, listener)
	mtprotoListener.uid = listener.GetUid()
	reciever := tgMtProtoClient.AddEventReceiver(&tdlib.UpdateFile{}, func(msg *tdlib.TdMessage) bool {
		updateMsg := (*msg).(*tdlib.UpdateFile)
		if updateMsg.File.ID == mtprotoListener.FileId {
			return true
		}
		return false
	})
	mtprotoListener.eventReceiver = &reciever
	log.Printf("MtprotoDetails: %d | %s | %d\n", fileId, name, mtprotoListener.eventReceiver.ID)
	go func() {
		for event := range reciever.Chan {
			updateMsg := (event).(*tdlib.UpdateFile)
			var isQueued bool = false
			if updateMsg.File.Local.IsDownloadingCompleted {
				reciever.GetUpdates = false
				tgMtProtoClient.RemoveEventReceiver(reciever)
				mtprotoListener.OnDownloadComplete(updateMsg.File.ID, updateMsg.File.Local.Path)
				return
			}
			if !updateMsg.File.Local.IsDownloadingActive && !updateMsg.File.Local.IsDownloadingCompleted {
				isQueued = true
			}
			mtprotoListener.OnDownloadProgress(updateMsg.File.ID, int64(updateMsg.File.Local.DownloadedSize), int64(updateMsg.File.Size), updateMsg.File.Local.Path, isQueued)
		}
	}()
	gid := utils.RandString(16)
	status := NewTelegramDownloadStatus(gid, fileId, name, mtprotoListener)
	status.Index_ = GenerateMirrorIndex()
	AddMirrorLocal(listener.GetUid(), status)
	_, err = tgMtProtoClient.DownloadFile(fileId, 1, 0, 0, false)
	if err != nil {
		status.GetListener().OnDownloadError(err.Error())
		return nil
	}
	status.GetListener().OnDownloadStart(status.Gid())
	return nil
}

func NewTelegramDownload(msg *gotgbot.Message, listener *MirrorListener) error {
	return tgDownloader.AddDownload(msg, listener)
}

type TelegramDownloadStatus struct {
	gid        string
	fileId     int32
	mtListener *TgMtProtoListener
	name       string
	Index_     int
}

func (t *TelegramDownloadStatus) Name() string {
	return t.name
}

func (t *TelegramDownloadStatus) Gid() string {
	return t.gid
}

func (t *TelegramDownloadStatus) CompletedLength() int64 {
	return t.mtListener.current
}

func (t *TelegramDownloadStatus) TotalLength() int64 {
	return t.mtListener.total
}

func (t *TelegramDownloadStatus) Speed() int64 {
	return t.mtListener.speed
}

func (t *TelegramDownloadStatus) GetStatusType() string {
	if t.mtListener.isQueued {
		return MirrorStatusWaiting
	}
	return MirrorStatusDownloading
}

func (t *TelegramDownloadStatus) Path() string {
	return t.mtListener.path
}

func (t *TelegramDownloadStatus) ETA() *time.Duration {
	dur := t.mtListener.eta
	return &dur
}

func (t *TelegramDownloadStatus) Percentage() float32 {
	return float32(t.CompletedLength()*100) / float32(t.TotalLength())
}

func (t *TelegramDownloadStatus) GetListener() *MirrorListener {
	return t.mtListener.listener
}

func (t *TelegramDownloadStatus) GetCloneListener() *CloneListener {
	return nil
}

func (t *TelegramDownloadStatus) Index() int {
	return t.Index_
}

func (t *TelegramDownloadStatus) CancelMirror() bool {
	if t.mtListener.eventReceiver != nil {
		t.mtListener.eventReceiver.GetUpdates = false
		tgMtProtoClient.RemoveEventReceiver(*t.mtListener.eventReceiver)
	}
	_, err := tgMtProtoClient.CancelDownloadFile(t.fileId, false)
	if err != nil {
		t.GetListener().OnDownloadError(err.Error())
		return true
	}
	t.GetListener().OnDownloadError("Canceled by user.")
	return true
}

func NewTelegramDownloadStatus(gid string, fileId int32, name string, mtListener *TgMtProtoListener) *TelegramDownloadStatus {
	return &TelegramDownloadStatus{
		gid:        gid,
		fileId:     fileId,
		name:       name,
		mtListener: mtListener,
	}
}
