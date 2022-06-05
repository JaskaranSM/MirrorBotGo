package engine

import (
	"MirrorBotGo/utils"
	"errors"
	"os"
	"path"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
)

var anacrolixClient *torrent.Client = getAnacrolixTorrentClient()
var anacrolixDownloader *AnacrolixTorrentDownloader = getAnacrolixTorrentDownloader()

func getAnacrolixTorrentClient() *torrent.Client {
	config := torrent.NewDefaultClientConfig()
	config.EstablishedConnsPerTorrent = 100
	config.HTTPUserAgent = "qBittorrent/4.3.8"
	config.Bep20 = "-qB4380-"
	config.UpnpID = "qBittorrent/4.3.8"
	client, err := torrent.NewClient(config)
	if err != nil {
		L().Fatal(err)
	}
	return client
}

func getAnacrolixTorrentDownloader() *AnacrolixTorrentDownloader {
	return &AnacrolixTorrentDownloader{}
}

type AnacrolixTorrentDownloadListener struct {
	torrentHandle     *torrent.Torrent
	listener          *MirrorListener
	IsListenerRunning bool
	IsQueued          bool
	haveInfo          bool
}

func (a *AnacrolixTorrentDownloadListener) OnDownloadComplete() {
	L().Infof("[ALXTorrent]: OnDownloadComplete: %s", a.torrentHandle.Name())
	a.StopListener()
	a.torrentHandle.Drop()
	a.listener.OnDownloadComplete()
}

func (a *AnacrolixTorrentDownloadListener) OnMetadataDownloadComplete() {
	L().Infof("[ALXTorrent]: OnMetadataDownloadComplete: %s", a.torrentHandle.Name())
	a.torrentHandle.DownloadAll()
}

func (a *AnacrolixTorrentDownloadListener) OnDownloadStop() {
	L().Infof("[AnacrolixTorrent]: OnDownloadStop: %s", a.torrentHandle.Name())
	a.StopListener()
	a.listener.OnDownloadError("Canceled by user.")
}

func (a *AnacrolixTorrentDownloadListener) ListenForEvents() {
	for a.IsListenerRunning {
		select {
		case <-a.torrentHandle.GotInfo():
			if !a.haveInfo {
				a.haveInfo = true
				a.OnMetadataDownloadComplete()
			}
		case <-a.torrentHandle.Closed():
			a.OnDownloadStop()
		case <-a.torrentHandle.Complete.On():
			a.OnDownloadComplete()
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (a *AnacrolixTorrentDownloadListener) StartListener() {
	a.IsListenerRunning = true
	go a.ListenForEvents()
}

func (a *AnacrolixTorrentDownloadListener) StopListener() {
	a.IsListenerRunning = false
}

func NewAnacrolixTorrentDownloadListener(t *torrent.Torrent, listener *MirrorListener) *AnacrolixTorrentDownloadListener {
	return &AnacrolixTorrentDownloadListener{
		torrentHandle:     t,
		listener:          listener,
		IsListenerRunning: false,
	}
}

type AnacrolixTorrentDownloader struct {
}

func (a *AnacrolixTorrentDownloader) GetTorrentSpec(link string) (*torrent.TorrentSpec, error) {
	var spec *torrent.TorrentSpec
	var err error
	if utils.IsMagnetLink(link) {
		spec, err = torrent.TorrentSpecFromMagnetUri(link)
		if err != nil {
			return spec, err
		}
	} else {
		isTorrent, err := utils.IsTorrentLink(link)
		if err != nil {
			return spec, err
		}
		if !isTorrent {
			return spec, errors.New("Not a torrent/magnet link")
		}
		reader, err := utils.GetReaderHandleByUrl(link)
		if err != nil {
			return spec, err
		}
		meta, err := metainfo.Load(reader)
		if err != nil {
			return spec, err
		}
		spec, err = torrent.TorrentSpecFromMetaInfoErr(meta)
		if err != nil {
			return spec, err
		}
	}
	return spec, err
}

func (a *AnacrolixTorrentDownloader) AddDownload(link string, listener *MirrorListener) error {
	dir := path.Join(utils.GetDownloadDir(), utils.ParseInt64ToString(listener.GetUid()))
	spec, err := a.GetTorrentSpec(link)
	if err != nil {
		return err
	}
	os.MkdirAll(dir, 0755)
	spec.Storage = storage.NewFile(dir)
	t, _, err := anacrolixClient.AddTorrentSpec(spec)
	if err != nil {
		return err
	}
	anacrolixListener := NewAnacrolixTorrentDownloadListener(t, listener)
	anacrolixListener.StartListener()
	gid := utils.RandString(16)
	status := NewAnacrolixTorrentDownloadStatus(gid, listener, anacrolixListener, t)
	status.Index_ = GenerateMirrorIndex()
	AddMirrorLocal(listener.GetUid(), status)
	status.GetListener().OnDownloadStart(status.Gid())
	return nil
}

func NewAnacrolixTorrentDownload(link string, listener *MirrorListener) error {
	return anacrolixDownloader.AddDownload(link, listener)
}

type AnacrolixTorrentDownloadStatus struct {
	gid               string
	listener          *MirrorListener
	anacrolixListener *AnacrolixTorrentDownloadListener
	Index_            int
	torrentHandle     *torrent.Torrent
}

func (a *AnacrolixTorrentDownloadStatus) Name() string {
	return a.torrentHandle.Name()
}

func (a *AnacrolixTorrentDownloadStatus) TotalLength() int64 {
	if !a.anacrolixListener.haveInfo {
		return 0
	}
	return a.torrentHandle.Length()
}

func (a *AnacrolixTorrentDownloadStatus) CompletedLength() int64 {
	return a.torrentHandle.BytesCompleted()
}

func (a *AnacrolixTorrentDownloadStatus) GetListener() *MirrorListener {
	return a.listener
}

func (a *AnacrolixTorrentDownloadStatus) Speed() int64 {
	var speed float64
	for _, peer := range a.torrentHandle.PeerConns() {
		speed += peer.DownloadRate()
	}
	return int64(speed)
}

func (a *AnacrolixTorrentDownloadStatus) Gid() string {
	return a.gid
}

func (a *AnacrolixTorrentDownloadStatus) Percentage() float32 {
	if a.CompletedLength() == 0 {
		return float32(0.00)
	}
	return float32(a.CompletedLength()*100) / float32(a.TotalLength())
}

func (a *AnacrolixTorrentDownloadStatus) ETA() *time.Duration {
	dur := utils.CalculateETA(a.TotalLength()-a.CompletedLength(), a.Speed())
	return &dur
}

func (a *AnacrolixTorrentDownloadStatus) GetStatusType() string {
	if a.anacrolixListener.IsQueued {
		return MirrorStatusWaiting
	}
	return MirrorStatusDownloading
}

func (a *AnacrolixTorrentDownloadStatus) Path() string {
	return path.Join(utils.GetDownloadDir(), utils.ParseInt64ToString(a.GetListener().GetUid()), a.Name())
}

func (a *AnacrolixTorrentDownloadStatus) GetCloneListener() *CloneListener {
	return nil
}

func (a *AnacrolixTorrentDownloadStatus) Index() int {
	return a.Index_
}

func (a *AnacrolixTorrentDownloadStatus) CancelMirror() bool {
	a.torrentHandle.Drop()
	return true
}

func NewAnacrolixTorrentDownloadStatus(gid string, listener *MirrorListener, anacrolixListener *AnacrolixTorrentDownloadListener, t *torrent.Torrent) *AnacrolixTorrentDownloadStatus {
	return &AnacrolixTorrentDownloadStatus{
		gid:               gid,
		listener:          listener,
		anacrolixListener: anacrolixListener,
		torrentHandle:     t,
	}
}
