package engine

import (
	"MirrorBotGo/utils"
	"context"
	"log"
	"path"
	"time"

	"github.com/zyxar/argo/rpc"
)

var client rpc.Client = getSession()
var ariaDownloader *AriaDownloader = getAriaDownloader()

type Aria2Listener struct {
	dl string
}

func (a Aria2Listener) OnDownloadStart(events []rpc.Event) {
	log.Println("[OnDownloadStart]")
}

func (a Aria2Listener) OnDownloadStop(events []rpc.Event) {
	log.Println("[OnDownloadStop]")
	for _, event := range events {
		dl := GetMirrorByGid(event.Gid)
		go dl.GetListener().OnDownloadError("Canceled by Stop event.")
	}
}

func (a Aria2Listener) OnDownloadError(events []rpc.Event) {
	log.Println("[OnDownloadError]")
	for _, event := range events {
		ariaDl, _ := client.TellStatus(event.Gid)
		dl := GetMirrorByGid(event.Gid)
		go dl.GetListener().OnDownloadError(ariaDl.ErrorMessage)
	}
}

func (a Aria2Listener) OnDownloadPause(events []rpc.Event) {
	log.Println("[OnDownloadPause]")
}

func (a Aria2Listener) OnDownloadComplete(events []rpc.Event) {
	log.Println("[OnDownloadComplete]")
	for _, event := range events {
		ariaDl, _ := client.TellStatus(event.Gid)
		dl := GetMirrorByGid(event.Gid)
		listener := dl.GetListener()
		if len(ariaDl.FollowedBy) != 0 {
			status := NewAriaDownloadStatus(dl.Name(), ariaDl.FollowedBy[0], listener)
			status.Index_ = dl.Index()
			AddMirrorLocal(listener.GetUid(), status)
			go listener.OnDownloadStart(status.Gid())
		} else {
			go listener.OnDownloadComplete()
		}
	}
}

func (a Aria2Listener) OnBtDownloadComplete(events []rpc.Event) {
	log.Println("[OnBtDownloadComplete]")
	log.Println(events)
}

func getSession() rpc.Client {
	RPC_URL := "http://localhost:6800/jsonrpc"
	RPC_TOKEN := ""
	notifier := Aria2Listener{}
	client, err := rpc.New(context.Background(), RPC_URL, RPC_TOKEN, 20*time.Second, notifier)
	if err != nil {
		log.Fatalf("Unable to start Aria2 Client: %v", err)
	}
	return client
}

func getAriaDownloader() *AriaDownloader {
	td := &AriaDownloader{}
	return td
}

type AriaDownloadStatus struct {
	ariaGid  string
	listener *MirrorListener
	Index_   int
	name     string
}

func (t *AriaDownloadStatus) GetStats() rpc.StatusInfo {
	stats, _ := client.TellStatus(t.ariaGid)
	return stats
}

func (t *AriaDownloadStatus) Name() string {
	stats := t.GetStats()
	if stats.BitTorrent.Info.Name != "" {
		return stats.BitTorrent.Info.Name
	}
	if len(stats.Files) != 0 {
		t.name = utils.GetFileBaseName(stats.Files[0].Path)
	}
	return t.name
}

func (t *AriaDownloadStatus) CompletedLength() int64 {
	stats := t.GetStats()
	return utils.ParseStringToInt64(stats.CompletedLength)
}

func (t *AriaDownloadStatus) TotalLength() int64 {
	stats := t.GetStats()
	return utils.ParseStringToInt64(stats.TotalLength)
}

func (t *AriaDownloadStatus) Speed() int64 {
	stats := t.GetStats()
	return utils.ParseStringToInt64(stats.DownloadSpeed)
}

func (t *AriaDownloadStatus) ETA() *time.Duration {
	dur := utils.CalculateETA(t.TotalLength()-t.CompletedLength(), t.Speed())
	return &dur
}

func (t *AriaDownloadStatus) Gid() string {
	return t.ariaGid
}

func (t *AriaDownloadStatus) Percentage() float32 {
	if t.CompletedLength() == 0 {
		return float32(0.00)
	}
	return float32(t.CompletedLength()*100) / float32(t.TotalLength())
}

func (t *AriaDownloadStatus) GetStatusType() string {
	return MirrorStatusDownloading
}

func (t *AriaDownloadStatus) Path() string {
	return path.Join(utils.GetDownloadDir(), utils.ParseIntToString(t.GetListener().GetUid()), t.Name())
}

func (t *AriaDownloadStatus) GetListener() *MirrorListener {
	return t.listener
}

func (t *AriaDownloadStatus) Index() int {
	return t.Index_
}

func (t *AriaDownloadStatus) CancelMirror() bool {
	_, err := client.Pause(t.Gid())
	if err != nil {
		log.Println(err)
	}
	t.GetListener().OnDownloadError("Canceled by user.")
	return true
}

func NewAriaDownloadStatus(name string, ariaGid string, listener *MirrorListener) *AriaDownloadStatus {
	return &AriaDownloadStatus{name: name, ariaGid: ariaGid, listener: listener}
}

type AriaDownloader struct {
	IsListenerRunning bool
}

func (t *AriaDownloader) AddDownload(link string, listener *MirrorListener) error {
	pth := path.Join(utils.GetDownloadDir(), utils.ParseIntToString(listener.GetUid()))
	opt := make(map[string]string)
	opt["dir"] = pth
	ariaGid, err := client.AddURI(link, opt)
	if err != nil {
		return err
	}
	status := NewAriaDownloadStatus(utils.GetFileBaseName(link), ariaGid, listener)
	status.Index_ = GlobalMirrorIndex + 1
	AddMirrorLocal(listener.GetUid(), status)
	status.GetListener().OnDownloadStart(status.Gid())
	return nil
}

func NewAriaDownload(link string, listener *MirrorListener) error {
	return ariaDownloader.AddDownload(link, listener)
}
