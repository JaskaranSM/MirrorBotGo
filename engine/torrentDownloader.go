package engine

import (
	"MirrorBotGo/utils"
	"log"
	"path"
	"time"

	"github.com/cenkalti/rain/torrent"
)

var client *torrent.Session = getSession()
var torrentDownloader *TorrentDownloader = getTorrentDownloader()

func getSession() *torrent.Session {
	torrent.DefaultConfig.RPCEnabled = false
	torrent.DefaultConfig.DataDir = utils.GetDownloadDir()
	torrent.DefaultConfig.ResumeOnStartup = false
	torrent.DefaultConfig.MaxOpenFiles = 1020
	torrent.DefaultConfig.SpeedLimitUpload = 1
	client, err := torrent.NewSession(torrent.DefaultConfig)
	if err != nil {
		log.Fatal(err)
	}
	return client
}

func getTorrentDownloader() *TorrentDownloader {
	td := &TorrentDownloader{}
	return td
}

type TorrentDownloadStatus struct {
	tor      *torrent.Torrent
	listener *MirrorListener
	Index_   int
}

func (t *TorrentDownloadStatus) Update() {
	t.tor = client.GetTorrent(t.tor.ID())
}

func (t *TorrentDownloadStatus) GetStats() torrent.Stats {
	return t.tor.Stats()
}

func (t *TorrentDownloadStatus) Name() string {
	stats := t.GetStats()
	return stats.Name
}

func (t *TorrentDownloadStatus) CompletedLength() int64 {
	stats := t.GetStats()
	return stats.Bytes.Completed
}

func (t *TorrentDownloadStatus) TotalLength() int64 {
	stats := t.GetStats()
	return stats.Bytes.Total
}

func (t *TorrentDownloadStatus) Speed() int64 {
	stats := t.GetStats()
	return int64(stats.Speed.Download)
}

func (t *TorrentDownloadStatus) ETA() *time.Duration {
	stats := t.GetStats()
	return stats.ETA
}

func (t *TorrentDownloadStatus) Gid() string {
	return t.tor.ID()
}

func (t *TorrentDownloadStatus) Percentage() float32 {
	if t.CompletedLength() == 0 {
		return float32(0.00)
	}
	return float32(t.CompletedLength()*100) / float32(t.TotalLength())
}

func (t *TorrentDownloadStatus) GetStatusType() string {
	return MirrorStatusDownloading
}

func (t *TorrentDownloadStatus) Path() string {
	return path.Join(utils.GetDownloadDir(), t.Gid(), t.Name())
}

func (t *TorrentDownloadStatus) GetListener() *MirrorListener {
	return t.listener
}

func (t *TorrentDownloadStatus) Index() int {
	return t.Index_
}

func (t *TorrentDownloadStatus) CancelMirror() bool {
	t.tor.Stop()
	listener := t.GetListener()
	listener.OnDownloadError("Canceled by user.")
	return true
}

func NewTorrentDownloadStatus(tor *torrent.Torrent, listener *MirrorListener) *TorrentDownloadStatus {
	return &TorrentDownloadStatus{tor: tor, listener: listener}
}

type TorrentDownloader struct {
	IsListenerRunning bool
}

func (t *TorrentDownloader) Listen(tor *torrent.Torrent) {
	tor.Start()
	go func() {
		for _ = range tor.NotifyComplete() {
			log.Println("Listening Complete")
		}
		dl := GetMirrorByGid(tor.ID())
		if dl != nil {
			listener := dl.GetListener()
			listener.OnDownloadComplete()
		}
	}()
	go func() {
		for _ = range tor.NotifyStop() {
			log.Println("Listening Stop")
		}
	}()
}

func (t *TorrentDownloader) AddDownload(link string, listener *MirrorListener) error {
	tor, err := client.AddURI(link, &torrent.AddTorrentOptions{StopAfterDownload: true})
	if err != nil {
		return err
	}
	t.Listen(tor)
	status := NewTorrentDownloadStatus(tor, listener)
	status.Index_ = GlobalMirrorIndex + 1
	AddMirrorLocal(listener.GetUid(), status)
	status.GetListener().OnDownloadStart(tor.ID())
	return nil
}

func NewTorrentDownload(link string, listener *MirrorListener) error {
	return torrentDownloader.AddDownload(link, listener)
}

func Clean() {
	for _, t := range client.ListTorrents() {
		client.RemoveTorrent(t.ID())
	}
}
