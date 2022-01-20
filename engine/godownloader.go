package engine

import (
	"MirrorBotGo/utils"
	"log"
	"os"
	"path"
	"time"

	"github.com/dustin/go-humanize"
	godownloader "github.com/jaskaranSM/go-downloader"
)

var dlEngine *godownloader.DownloadEngine

func init() {
	dlEngine = godownloader.NewDownloadEngine()
	dlEngine.AddEventListener(&DlListener{})
}

type DlListener struct {
}

func (dl *DlListener) OnDownloadStart(gid string, dlinfo *godownloader.DownloadInfo) {
	log.Printf("[GoDown-OnDownloadStart]: %s\n", gid)
	download := GetMirrorByGid(gid)
	if download != nil {
		go download.GetListener().OnDownloadStart(gid)
	}
}

func (dl *DlListener) OnDownloadStop(gid string, dlinfo *godownloader.DownloadInfo) {
	log.Printf("[GoDown-OnDownloadStop]: %s\n", gid)
	log.Printf("Error: %s\n", dlinfo.Error.Error())
	download := GetMirrorByGid(gid)
	if download != nil {
		go download.GetListener().OnDownloadError(dlinfo.Error.Error())
	}
}

func (dl *DlListener) OnDownloadComplete(gid string, dlinfo *godownloader.DownloadInfo) {
	log.Printf("[GoDown-OnDownloadComplete]: %s\n", gid)
	download := GetMirrorByGid(gid)
	if download != nil {
		go download.GetListener().OnDownloadComplete()
	}
}

func (dl *DlListener) OnDownloadProgress(gid string, dlinfo *godownloader.DownloadInfo) {
	log.Printf("%s, Speed: %s, Downloaded: %s, Total: %s, Type: %s ETA: %s\n",
		dlinfo.Name,
		humanize.Bytes(uint64(dlinfo.Speed)),
		humanize.Bytes(uint64(dlinfo.CompletedLength)),
		humanize.Bytes(uint64(dlinfo.TotalLength)),
		dlinfo.Type, dlinfo.ETA)
	time.Sleep(1 * time.Second)
}

func NewGoDownloadStatus(gid string, index int, listener *MirrorListener, dlinfo *godownloader.DownloadInfo) *GoDownloadStatus {
	return &GoDownloadStatus{
		gid:      gid,
		Index_:   index,
		listener: listener,
		dlinfo:   dlinfo,
	}
}

type GoDownloadStatus struct {
	gid       string
	Index_    int
	listener  *MirrorListener
	isTorrent bool
	dlinfo    *godownloader.DownloadInfo
}

func (g *GoDownloadStatus) Name() string {
	return g.dlinfo.Name
}

func (g *GoDownloadStatus) CompletedLength() int64 {
	return g.dlinfo.CompletedLength
}

func (g *GoDownloadStatus) TotalLength() int64 {
	return g.dlinfo.TotalLength
}

func (g *GoDownloadStatus) Speed() int64 {
	return g.dlinfo.Speed
}

func (g *GoDownloadStatus) ETA() *time.Duration {
	eta := g.dlinfo.ETA
	return &eta
}

func (g *GoDownloadStatus) Gid() string {
	return g.gid
}

func (g *GoDownloadStatus) Percentage() float32 {
	if g.CompletedLength() == 0 {
		return float32(0.00)
	}
	return float32(g.CompletedLength()*100) / float32(g.TotalLength())
}

func (g *GoDownloadStatus) GetStatusType() string {
	if g.dlinfo.IsCancelled {
		return MirrorStatusCanceled
	}
	if g.dlinfo.IsFailed {
		return MirrorStatusFailed
	}
	return MirrorStatusDownloading
}

func (g *GoDownloadStatus) Path() string {
	return path.Join(utils.GetDownloadDir(), utils.ParseIntToString(g.GetListener().GetUid()), g.Name())
}

func (g *GoDownloadStatus) GetListener() *MirrorListener {
	return g.listener
}

func (g *GoDownloadStatus) Index() int {
	return g.Index_
}

func (g *GoDownloadStatus) CancelMirror() bool {
	dlEngine.CancelDownloadByGid(g.Gid())
	return true
}

func NewGoDownload(link string, listener *MirrorListener) error {
	pth := path.Join(utils.GetDownloadDir(), utils.ParseIntToString(listener.GetUid()))
	os.MkdirAll(pth, 0755)
	opt := make(map[string]string)
	opt["dir"] = pth
	opt["connections"] = "16"
	log.Println("Adding download: ", link)
	gid := dlEngine.AddURL(link, opt)
	dlinfo := dlEngine.GetDownloadInfoByGid(gid)
	status := NewGoDownloadStatus(gid, GenerateMirrorIndex(), listener, dlinfo)
	AddMirrorLocal(listener.GetUid(), status)
	return nil
}
