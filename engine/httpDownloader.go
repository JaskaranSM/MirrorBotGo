package engine

import (
	"MirrorBotGo/utils"
	"fmt"
	"net/http"
	"path"
	"time"

	"github.com/jaskaranSM/go-httpdl"
)

type HTTPDownloadStatus struct {
	dl       *httpdl.HTTPDownload
	listener *MirrorListener
	Index_   int
}

func (h *HTTPDownloadStatus) Name() string {
	return h.dl.Name()
}

func (h *HTTPDownloadStatus) CompletedLength() int64 {
	return h.dl.CompletedLength()
}

func (h *HTTPDownloadStatus) TotalLength() int64 {
	return h.dl.TotalLength()
}

func (h *HTTPDownloadStatus) Speed() int64 {
	return h.dl.Speed()
}

func (h *HTTPDownloadStatus) Gid() string {
	return h.dl.Gid()
}

func (h *HTTPDownloadStatus) ETA() *time.Duration {
	if h.Speed() != 0 {
		dur := utils.CalculateETA(h.TotalLength()-h.CompletedLength(), h.Speed())
		return &dur
	}
	dur := time.Duration(0)
	return &dur
}

func (h *HTTPDownloadStatus) GetStatusType() string {
	return MirrorStatusDownloading
}

func (h *HTTPDownloadStatus) Path() string {
	return path.Join(utils.GetDownloadDir(), utils.ParseInt64ToString(h.GetListener().GetUid()), h.Name())
}

func (h *HTTPDownloadStatus) Percentage() float32 {
	return float32(h.CompletedLength()*100) / float32(h.TotalLength())
}

func (h *HTTPDownloadStatus) IsTorrent() bool {
	return false
}

func (h *HTTPDownloadStatus) GetPeers() int {
	return 0
}

func (h *HTTPDownloadStatus) GetSeeders() int {
	return 0
}

func (h *HTTPDownloadStatus) GetListener() *MirrorListener {
	return h.listener
}

func (h *HTTPDownloadStatus) GetCloneListener() *CloneListener {
	return nil
}

func (h *HTTPDownloadStatus) CancelMirror() bool {
	h.dl.CancelDownload()
	return true
}

func (h *HTTPDownloadStatus) Index() int {
	return h.Index_
}

func NewHTTPDownloadStatus(listener *MirrorListener, dl *httpdl.HTTPDownload) *HTTPDownloadStatus {
	return &HTTPDownloadStatus{
		listener: listener,
		dl:       dl,
	}
}

func NewHTTPDownloadListener(listener *MirrorListener) *HTTPDownloadListener {
	return &HTTPDownloadListener{
		listener: listener,
	}
}

type HTTPDownloadListener struct {
	listener *MirrorListener
}

func (h *HTTPDownloadListener) OnDownloadStart(dl *httpdl.HTTPDownload) {
	h.listener.OnDownloadStart(dl.Gid())
}

func (h *HTTPDownloadListener) OnDownloadStop(dl *httpdl.HTTPDownload) {
	err := dl.GetFailureError()
	if err == nil {
		//I have no idea why did I do it this way. guess we find out when time comes
		err = fmt.Errorf("unknown error, debug wen: %v", err)
	}
	h.listener.OnDownloadError(err.Error())
}

func (h *HTTPDownloadListener) OnDownloadComplete(dl *httpdl.HTTPDownload) {
	h.listener.OnDownloadComplete()
}

func NewHTTPDownload(link string, listener *MirrorListener) error {
	httpDownloader := httpdl.NewHTTPDownloader(&http.Client{})
	httpListener := NewHTTPDownloadListener(listener)
	httpDownloader.AddListener(httpListener)
	dir := path.Join(utils.GetDownloadDir(), utils.ParseInt64ToString(listener.GetUid()))
	dl, err := httpDownloader.AddDownload(link, &httpdl.AddDownloadOpts{
		Connections: 10,
		Dir:         dir,
	})
	if err != nil {
		return err
	}
	status := NewHTTPDownloadStatus(listener, dl)
	status.Index_ = GenerateMirrorIndex()
	AddMirrorLocal(listener.GetUid(), status)
	return nil
}
