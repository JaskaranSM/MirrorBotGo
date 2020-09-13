package engine

import (
	"MirrorBotGo/utils"
	"log"
	"os"
	"path"
	"time"

	"github.com/cavaliercoder/grab"
)

type HttpDownloader struct {
	client   *grab.Client
	BasePath string
	Gid      string
}

func (h *HttpDownloader) Listen(resp *grab.Response) {
	go func() {
		for _ = range resp.Done {
			log.Println("HTTP Download Complete")
		}
		dl := GetMirrorByGid(h.Gid)
		if dl != nil {
			listener := dl.GetListener()
			listener.OnDownloadComplete()
		}
	}()
}
func (h *HttpDownloader) AddDownload(link string, listener *MirrorListener) error {
	req, err := grab.NewRequest(h.BasePath, link)
	if err != nil {
		return err
	}
	resp := h.client.Do(req)
	h.Listen(resp)
	status := NewHttpDownloadStatus(h.Gid, h.BasePath, resp, listener)
	AddMirrorLocal(listener.GetUid(), status)
	status.GetListener().OnDownloadStart(status.Gid())
	return nil
}

func NewHttpDownload(link string, listener *MirrorListener) error {
	shortId := utils.GetShortId()
	pth := path.Join(utils.GetDownloadDir(), shortId)
	os.MkdirAll(pth, 0755)
	httpDl := &HttpDownloader{client: grab.NewClient(), Gid: shortId, BasePath: pth}
	return httpDl.AddDownload(link, listener)
}

type HttpDownloadStatus struct {
	resp     *grab.Response
	listener *MirrorListener
	gid      string
	BasePath string
}

func (h *HttpDownloadStatus) Name() string {
	return utils.GetFileBaseName(h.resp.Filename)
}

func (h *HttpDownloadStatus) CompletedLength() int64 {
	return h.resp.BytesComplete()
}

func (h *HttpDownloadStatus) TotalLength() int64 {
	return h.resp.Size()
}
func (h *HttpDownloadStatus) Speed() int64 {
	return int64(h.resp.BytesPerSecond())
}

func (h *HttpDownloadStatus) ETA() *time.Duration {
	if h.CompletedLength() == 0 {
		d := time.Duration(0)
		return &d
	}
	eta := h.resp.ETA()
	dur := time.Until(eta)
	return &dur
}

func (h *HttpDownloadStatus) Gid() string {
	return h.gid
}

func (h *HttpDownloadStatus) GetStatusType() string {
	return MirrorStatusDownloading
}

func (h *HttpDownloadStatus) Path() string {
	return path.Join(h.BasePath, h.Name())
}

func (h *HttpDownloadStatus) Percentage() float32 {
	if h.CompletedLength() == 0 {
		return float32(0.00)
	}
	return float32(h.CompletedLength()*100) / float32(h.TotalLength())
}

func (h *HttpDownloadStatus) GetListener() *MirrorListener {
	return h.listener
}

func (h *HttpDownloadStatus) CancelMirror() bool {
	log.Println("Cancelling http download")
	go h.resp.Cancel()
	listener := h.GetListener()
	listener.OnDownloadError("Canceled by user.")
	return true
}

func NewHttpDownloadStatus(gid string, pth string, resp *grab.Response, listener *MirrorListener) *HttpDownloadStatus {
	return &HttpDownloadStatus{gid: gid, BasePath: pth, resp: resp, listener: listener}
}
