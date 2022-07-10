package engine

import (
	"MirrorBotGo/utils"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/gotd/contrib/bg"
	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/tg"
)

var gotdClient *telegram.Client = getTgClient()
var gotdDownloader *GotdDownloader = getGotdDownloader()

var accessHashesCache map[int64]int64 = make(map[int64]int64)

func GetChatIdFromPeerClass(c tg.PeerClass) int64 {
	switch m := c.(type) {
	case *tg.PeerChat:
		return m.ChatID
	case *tg.PeerChannel:
		return m.ChannelID
	case *tg.PeerUser:
		return m.UserID
	default:
		return -1
	}
}

func GetAccessHashByChatID(chatID int64) int64 {
	for i, j := range accessHashesCache {
		if i == chatID {
			return j
		}
	}
	return -1
}

func AddAccessHashCache(chatID int64, accessHash int64) {
	accessHashesCache[chatID] = accessHash
}

func getGotdDownloader() *GotdDownloader {
	return &GotdDownloader{}
}

func getTgClient() *telegram.Client {
	appIDInt, err := strconv.Atoi(utils.GetTgAppId())
	if err != nil {
		L().Fatal(err)
	}
	dispatcher := tg.NewUpdateDispatcher()

	opts := telegram.Options{
		UpdateHandler:  dispatcher,
		SessionStorage: &session.FileStorage{Path: "file.session"},
	}
	client := telegram.NewClient(appIDInt, utils.GetTgAppHash(), opts)
	dispatcher.OnNewChannelMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewChannelMessage) error {
		for _, channel := range e.Channels {
			AddAccessHashCache(channel.ID, channel.AccessHash)
		}
		for _, user := range e.Users {
			AddAccessHashCache(user.ID, user.AccessHash)
		}
		return nil
	})
	_, err = bg.Connect(client)
	if err != nil {
		L().Fatal(err)
		return client
	}
	ctx := context.Background()

	status, err := client.Auth().Status(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if !status.Authorized {
		if _, err := client.Auth().Bot(ctx, utils.GetBotToken()); err != nil {
			log.Fatal(err)
		}
	}
	return client
}

func NewGotdDownloadListener(document *tg.Document, filename string, filePath string, listener *MirrorListener, prg *GotdProgressWriter) *GotdDownloadListener {
	return &GotdDownloadListener{
		document: document,
		filename: filename,
		filePath: filePath,
		listener: listener,
		prg:      prg,
	}
}

type GotdDownloadListener struct {
	document               *tg.Document
	filename               string
	filePath               string
	listener               *MirrorListener
	prg                    *GotdProgressWriter
	speed                  int64
	isSpeedObserverRunning bool
}

func (g *GotdDownloadListener) GetCompleted() int64 {
	return g.prg.completed
}

func (g *GotdDownloadListener) GetTotal() int64 {
	return g.prg.total
}

func (g *GotdDownloadListener) GetSpeed() int64 {
	return g.speed
}

func (g *GotdDownloadListener) IsCancelled() bool {
	return g.prg.isCancelled
}

func (g *GotdDownloadListener) Cancel() {
	g.prg.Cancel()
}

func (g *GotdDownloadListener) OnDownloadStart() {
	L().Infof("[GotdDownload] %s | %d -> %s", g.filename, g.document.Size, g.filePath)
	g.StartSpeedObserver()
}

func (g *GotdDownloadListener) OnDownloadComplete() {
	g.StopSpeedObserver()
	g.listener.OnDownloadComplete()
}

func (g *GotdDownloadListener) OnDownloadStop(err error) {
	g.StartSpeedObserver()
	g.listener.OnDownloadError(err.Error())
}

func (g *GotdDownloadListener) StartSpeedObserver() {
	g.isSpeedObserverRunning = true
	go g.SpeedObserver()
}

func (g *GotdDownloadListener) StopSpeedObserver() {
	g.isSpeedObserverRunning = false
}

func (g *GotdDownloadListener) SpeedObserver() {
	last := g.GetCompleted()
	for range time.Tick(1 * time.Second) {
		if !g.isSpeedObserverRunning {
			return
		}
		completed := g.GetCompleted()
		chunk := completed - last
		g.speed = chunk
		// L().Infof("Download speed: %d", g.GetSpeed())
		last = completed
	}
}

type GotdDownloader struct {
}

func (g *GotdDownloader) GetMessageClassArray(messages tg.MessagesMessagesClass) (tg.MessageClassArray, error) {
	switch m := messages.(type) {
	case *tg.MessagesMessages:
		return m.Messages, nil
	case *tg.MessagesMessagesSlice:
		return m.Messages, nil
	case *tg.MessagesChannelMessages:
		return m.Messages, nil
	default:
		return nil, nil
	}
}

func (g *GotdDownloader) GetMessageDocument(m *tg.Message) (*tg.MessageMediaDocument, error) {
	media, ok := m.GetMedia()
	if !ok {
		return nil, fmt.Errorf("failed to parse media")
	}
	switch m := media.(type) {
	case *tg.MessageMediaDocument:
		return m, nil
	}
	return nil, nil
}

func (g *GotdDownloader) GetDocumentFilename(document *tg.Document) string {
	for _, attr := range document.Attributes {
		filename, ok := attr.(*tg.DocumentAttributeFilename)
		if !ok {
			continue
		}
		return filename.GetFileName()
	}
	return ""
}

func (g *GotdDownloader) Download(ctx context.Context, api *tg.Client, document *tg.Document, writer io.WriterAt) chan error {
	d := downloader.NewDownloader()
	errorChannel := make(chan error)
	go func() {
		_, err := d.Download(api, document.AsInputDocumentFileLocation()).WithThreads(8).Parallel(ctx, writer)
		if err != nil {
			errorChannel <- err
		}
		close(errorChannel)
	}()
	return errorChannel
}

func (g *GotdDownloader) GetMessageAPI(ctx context.Context, api *tg.Client, messageID int, channelID int64, isPrivate bool) (*tg.Message, error) {
	var messages tg.MessagesMessagesClass
	var err error

	inputMessageId := tg.InputMessageID{ID: messageID}
	if isPrivate {
		messages, err = api.MessagesGetMessages(ctx, []tg.InputMessageClass{&inputMessageId})
	} else {
		chatIdStr := utils.ParseInt64ToString(channelID)
		if strings.HasPrefix(chatIdStr, "-100") {
			chatIdStr = string(chatIdStr[4:])
		}
		channelID = utils.ParseStringToInt64(chatIdStr)
		accessHash := GetAccessHashByChatID(channelID)
		messages, err = api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
			Channel: &tg.InputChannel{
				ChannelID:  channelID,
				AccessHash: accessHash,
			},
			ID: []tg.InputMessageClass{&inputMessageId},
		})

	}
	if err != nil {
		return nil, err
	}
	messes, err := g.GetMessageClassArray(messages)
	if err != nil {
		return nil, err
	}
	if len(messes) == 0 {
		return nil, fmt.Errorf("Failed to fetch message")
	}
	messageClass := messes[0]
	m, ok := messageClass.(*tg.Message)
	if !ok {
		return nil, fmt.Errorf("document parsing failed")
	}
	return m, nil
}

func (g *GotdDownloader) PrepareDocumentForDownload(ctx context.Context, api *tg.Client, messageId int, chatID int64, isPrivate bool) (*tg.Document, error) {
	m, err := g.GetMessageAPI(ctx, api, messageId, chatID, isPrivate)
	if err != nil {
		return nil, err
	}

	doc, err := g.GetMessageDocument(m)
	if doc == nil {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("Not a document")
	}
	document, ok := doc.Document.AsNotEmpty()
	if !ok {
		return nil, fmt.Errorf("document parsing failed")
	}
	return document, nil
}

func (g *GotdDownloader) AddDownload(msg *gotgbot.Message, listener *MirrorListener) error {
	api := tg.NewClient(gotdClient)
	ctx := context.Background()
	document, err := g.PrepareDocumentForDownload(ctx, api, int(msg.MessageId), msg.Chat.Id, msg.Chat.Type == "private")
	if err != nil {
		return err
	}
	gid := utils.RandString(16)
	filename := g.GetDocumentFilename(document)
	if filename == "" {
		filename = gid
	}
	dir := path.Join(utils.GetDownloadDir(), utils.ParseInt64ToString(listener.GetUid()))
	os.MkdirAll(dir, 0755)
	filePath := path.Join(dir, filename)
	writer, err := os.Create(filePath)
	if err != nil {
		return err
	}
	prg := NewGotdProgressWriter(writer, int64(document.Size))
	gotdListener := NewGotdDownloadListener(document, filename, filePath, listener, prg)

	status := NewGotdDownloadStatus(gotdListener, gid)
	status.Index_ = GenerateMirrorIndex()
	AddMirrorLocal(listener.GetUid(), status)

	errChannel := g.Download(ctx, api, document, prg)
	status.GetListener().OnDownloadStart(status.Gid())
	go func() {
		for err := range errChannel {
			if err != nil {
				gotdListener.OnDownloadStop(err)
				return
			}
		}
		gotdListener.OnDownloadComplete()
	}()
	gotdListener.OnDownloadStart()
	return nil
}

func NewTelegramDownload(msg *gotgbot.Message, listener *MirrorListener) error {
	return gotdDownloader.AddDownload(msg, listener)
}

type GotdDownloadStatus struct {
	gid          string
	gotdListener *GotdDownloadListener
	Index_       int
}

func (g *GotdDownloadStatus) Name() string {
	return g.gotdListener.filename
}

func (g *GotdDownloadStatus) Gid() string {
	return g.gid
}

func (g *GotdDownloadStatus) CompletedLength() int64 {
	return g.gotdListener.GetCompleted()
}

func (g *GotdDownloadStatus) TotalLength() int64 {
	return g.gotdListener.GetTotal()
}

func (g *GotdDownloadStatus) Speed() int64 {
	return g.gotdListener.GetSpeed()
}

func (g *GotdDownloadStatus) GetStatusType() string {
	return MirrorStatusDownloading
}

func (g *GotdDownloadStatus) Path() string {
	return g.gotdListener.filePath
}

func (g *GotdDownloadStatus) ETA() *time.Duration {
	speed := g.Speed()
	var dur time.Duration
	if speed != 0 {
		dur = utils.CalculateETA(g.TotalLength()-g.CompletedLength(), speed)
	} else {
		dur = time.Duration(0)
	}
	return &dur
}

func (g *GotdDownloadStatus) Percentage() float32 {
	return float32(g.CompletedLength()*100) / float32(g.TotalLength())
}

func (g *GotdDownloadStatus) IsTorrent() bool {
	return false
}

func (g *GotdDownloadStatus) GetPeers() int {
	return 0
}

func (g *GotdDownloadStatus) GetSeeders() int {
	return 0
}

func (g *GotdDownloadStatus) GetListener() *MirrorListener {
	return g.gotdListener.listener
}

func (g *GotdDownloadStatus) GetCloneListener() *CloneListener {
	return nil
}

func (g *GotdDownloadStatus) Index() int {
	return g.Index_
}

func (g *GotdDownloadStatus) CancelMirror() bool {
	g.gotdListener.Cancel()
	return true
}

func NewGotdDownloadStatus(gotdListener *GotdDownloadListener, gid string) *GotdDownloadStatus {
	return &GotdDownloadStatus{
		gotdListener: gotdListener,
		gid:          gid,
	}
}

type GotdProgressWriter struct {
	writer      io.WriterAt
	completed   int64
	total       int64
	isCancelled bool
}

func (p *GotdProgressWriter) WriteAt(b []byte, off int64) (int, error) {
	if p.isCancelled {
		return 0, errors.New("Canceled by user.")
	}
	n := len(b)
	p.completed += int64(n)
	p.writer.WriteAt(b, off)
	return n, nil
}

func (p *GotdProgressWriter) Cancel() {
	p.isCancelled = true
}

func NewGotdProgressWriter(writer io.WriterAt, size int64) *GotdProgressWriter {
	return &GotdProgressWriter{writer: writer, total: size, completed: 0}
}
