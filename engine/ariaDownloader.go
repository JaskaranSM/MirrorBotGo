package engine

import (
	"MirrorBotGo/utils"
	"fmt"
	"log"
	"path"
	"sync"
	"time"

	"github.com/coolerfall/aria2go"
)

//AriaStatusCodeToString get status as string
func AriaStatusCodeToString(code int) string {
	if code == -1 {
		return "Unknown"
	}
	switch code {
	case 0:
		return MirrorStatusDownloading
	case 1:
		return MirrorStatusWaiting
	case 2:
		return MirrorStatusCanceled
	case 3:
		return MirrorStatusDownloading
	case 4:
		return MirrorStatusFailed
	case 5:
		return MirrorStatusCanceled
	}
	return "Unknown"
}

var client *aria2go.Aria2 = getSession()
var ariaDownloader *AriaDownloader = getAriaDownloader()
var ariaMutex sync.Mutex

type Aria2Listener struct {
	dl string
}

func (a Aria2Listener) OnStart(gid string) {
	log.Println("[OnDownloadStart]")
}

func (a Aria2Listener) OnStop(gid string) {
	log.Println("[OnDownloadStop]")
	dl := GetMirrorByGid(gid)
	if dl != nil {
		go dl.GetListener().OnDownloadError("Canceled by Stop event.")
	}
}

func (a Aria2Listener) OnError(gid string) {
	log.Println("[OnDownloadError]")
	dl := GetMirrorByGid(gid)
	ariaMutex.Lock()
	dlinfo := client.GetDownloadInfo(gid)
	ariaMutex.Unlock()
	if dl != nil {
		go dl.GetListener().OnDownloadError(utils.ParseIntToString(dlinfo.ErrorCode))
	}
}

func (a Aria2Listener) OnPause(gid string) {
	log.Println("[OnDownloadPause]")
}

func (a Aria2Listener) OnComplete(gid string) {
	log.Println("[OnDownloadComplete]: ", gid)
	ariaMutex.Lock()
	dinfo := client.GetDownloadInfo(gid)
	ariaMutex.Unlock()
	dl := GetMirrorByGid(gid)
	if dl != nil {
		listener := dl.GetListener()
		if dinfo.FollowedByGid != "0" {
			status := NewAriaDownloadStatus(dl.Name(), dinfo.FollowedByGid, listener)
			status.Index_ = dl.Index()
			AddMirrorLocal(listener.GetUid(), status)
			go listener.OnDownloadStart(status.Gid())
		} else {
			go listener.OnDownloadComplete()
		}
	}
}

func getSession() *aria2go.Aria2 {
	notifier := Aria2Listener{}
	a := aria2go.NewAria2(aria2go.Config{
		Options: aria2go.Options{
			"seed-time":                 "0.01",
			"max-overall-upload-limit":  "1K",
			"max-concurrent-downloads":  "10",
			"min-split-size":            "10M",
			"split":                     "10",
			"save-session":              "ses.session",
			"max-connection-per-server": "10",
			"follow-torrent":            "mem",
			"allow-overwrite":           "true",
		},
	})
	a.SetNotifier(notifier)
	go a.Run()
	return a
}

func getAriaDownloader() *AriaDownloader {
	td := &AriaDownloader{}
	return td
}

type AriaDownloadStatus struct {
	ariaGid   string
	listener  *MirrorListener
	Index_    int
	isTorrent bool
	name      string
}

func (t *AriaDownloadStatus) GetStats() aria2go.DownloadInfo {
	ariaMutex.Lock()
	stats := client.GetDownloadInfo(t.ariaGid)
	ariaMutex.Unlock()
	return stats
}

func (t *AriaDownloadStatus) Name() string {
	stats := t.GetStats()
	if stats.MetaInfo.Name != "" {
		t.name = stats.MetaInfo.Name
		t.isTorrent = true
	}
	if !t.isTorrent && len(stats.Files) != 0 {
		pth := utils.GetFileBaseName(stats.Files[0].Name)
		if pth != "" {
			t.name = pth
		}
	}
	return t.name
}

func (t *AriaDownloadStatus) CompletedLength() int64 {
	stats := t.GetStats()
	return stats.BytesCompleted
}

func (t *AriaDownloadStatus) TotalLength() int64 {
	stats := t.GetStats()
	return stats.TotalLength
}

func (t *AriaDownloadStatus) Speed() int64 {
	stats := t.GetStats()
	return int64(stats.DownloadSpeed)
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
	stats := t.GetStats()
	return AriaStatusCodeToString(stats.Status)
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
	client.Remove(t.Gid())
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
	fmt.Println("Adding download: ", link)
	ariaMutex.Lock() //libaria2 is not thread safe
	ariaGid, err := client.AddUri(link, opt)
	ariaMutex.Unlock()
	if err != nil {
		return err
	}
	status := NewAriaDownloadStatus(utils.GetFileBaseName(link), ariaGid, listener)
	status.Index_ = GenerateMirrorIndex()
	AddMirrorLocal(listener.GetUid(), status)
	status.GetListener().OnDownloadStart(status.Gid())
	return nil
}

func NewAriaDownload(link string, listener *MirrorListener) error {
	return ariaDownloader.AddDownload(link, listener)
}
