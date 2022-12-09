package engine

import (
	"MirrorBotGo/utils"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
	"golang.org/x/time/rate"
)

var anacrolixClient *torrent.Client = getAnacrolixTorrentClient(utils.GetSeed())
var anacrolixDownloader *AnacrolixTorrentDownloader = getAnacrolixTorrentDownloader()

func GetAnacrolixTorrentClientStatus() bytes.Buffer {
	var buffer bytes.Buffer
	anacrolixClient.WriteStatus(&buffer)
	return buffer
}

func setupRateLimiters(config *torrent.ClientConfig) {
	uploadRate, err := utils.GetTorrentClientMaxUploadRate()
	if err != nil {
		L().Errorf("failed to get torrent client max upload rate: %v", err)
		return
	}
	L().Infof("[ALXTorrent]: setting max upload rate to: %s | %d", utils.GetHumanBytes(int64(uploadRate)), uploadRate)
	config.UploadRateLimiter = rate.NewLimiter(rate.Limit(uploadRate), 256<<10)
}

func getAnacrolixTorrentClient(seed bool) *torrent.Client {
	config := torrent.NewDefaultClientConfig()
	setupRateLimiters(config)
	config.EstablishedConnsPerTorrent = utils.GetTorrentClientEstablishedConnsPerTorrent()
	config.HTTPUserAgent = utils.GetTorrentClientHTTPUserAgent()
	config.Bep20 = utils.GetTorrentClientBep20()
	config.UpnpID = utils.GetTorrentClientUpnpID()
	config.ExtendedHandshakeClientVersion = utils.GetTorrentClientExtendedHandshakeClientVersion()
	config.Seed = seed
	config.MinDialTimeout = utils.GetTorrentClientMinDialTimeout()
	config.ListenPort = utils.GetTorrentClientListenPort()
	L().Infof("[ALXTorrent]: starting client on port: %d", config.ListenPort)
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
	isSeed            bool
	IsSeeding         bool
	IsComplete        bool
	SeedingSpeed      int64
	UploadedBytes     int64
	SeedStartTime     time.Time
	IsObserverRunning bool
}

func (a *AnacrolixTorrentDownloadListener) OnDownloadComplete() {
	L().Infof("[ALXTorrent]: OnDownloadComplete: %s", a.torrentHandle.Name())
	if !a.isSeed {
		a.torrentHandle.Drop()
		a.StopListener()
	} else {
		a.IsSeeding = true
		a.OnSeedingStart()
	}
	a.listener.OnDownloadComplete()
}

func (a *AnacrolixTorrentDownloadListener) OnMetadataDownloadComplete() {
	L().Infof("[ALXTorrent]: OnMetadataDownloadComplete: %s", a.torrentHandle.Name())
	a.torrentHandle.DownloadAll()
	a.StartSeedingSpeedObserver()
}

func (a *AnacrolixTorrentDownloadListener) OnDownloadStop(err error) {
	L().Infof("[ALXTorrent]: OnDownloadStop: %s | %v", a.torrentHandle.Name(), err)
	a.StopSeedingSpeedObserver()
	a.StopListener()
	a.listener.OnDownloadError(err.Error())
}

func (a *AnacrolixTorrentDownloadListener) OnSeedingStart() {
	L().Infof("[ALXTorrent]: OnSeedingStart: %s", a.torrentHandle.Name())
	a.SeedStartTime = time.Now()
	a.listener.OnSeedingStart(a.listener.GetDownload().Gid())
}

func (a *AnacrolixTorrentDownloadListener) OnSeedingError() {
	L().Infof("[ALXTorrent]: OnSeedingError: %s", a.torrentHandle.Name())
	a.StopSeedingSpeedObserver()
	a.StopListener()
	ratio := float64(a.UploadedBytes) / float64(a.torrentHandle.Length())
	seedTime := time.Now().Sub(a.SeedStartTime)
	a.listener.OnSeedingError(fmt.Errorf("Cancelled by user. Ratio: %.2f, SeedTime: %s", ratio, utils.HumanizeDuration(seedTime)))
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
			if a.IsSeeding {
				a.OnSeedingError()
			} else {
				a.OnDownloadStop(errors.New("cancelled by user"))
			}
		case <-a.torrentHandle.Complete.On():
			if !a.IsComplete {
				a.IsComplete = true
				a.OnDownloadComplete()
			}
		}
		time.Sleep(1 * time.Second)
	}
}

func (a *AnacrolixTorrentDownloadListener) SeedingSpeedObserver() {
	last := a.torrentHandle.Stats()
	for a.IsObserverRunning {
		stats := a.torrentHandle.Stats()
		chunk := stats.BytesWrittenData.Int64() - last.BytesWrittenData.Int64()
		a.SeedingSpeed = chunk
		a.UploadedBytes += chunk
		//L().Infof("Seeding speed: %d | Uploaded: %d", a.SeedingSpeed, a.UploadedBytes)
		last = stats
		time.Sleep(1 * time.Second)
	}
}

func (a *AnacrolixTorrentDownloadListener) StartSeedingSpeedObserver() {
	a.IsObserverRunning = true
	go a.SeedingSpeedObserver()
}

func (a *AnacrolixTorrentDownloadListener) StopSeedingSpeedObserver() {
	a.IsObserverRunning = false
}

func (a *AnacrolixTorrentDownloadListener) StartListener() {
	a.IsListenerRunning = true
	go a.ListenForEvents()
}

func (a *AnacrolixTorrentDownloadListener) StopListener() {
	a.IsListenerRunning = false
}

func NewAnacrolixTorrentDownloadListener(t *torrent.Torrent, listener *MirrorListener, isSeed bool) *AnacrolixTorrentDownloadListener {
	return &AnacrolixTorrentDownloadListener{
		torrentHandle:     t,
		listener:          listener,
		IsListenerRunning: false,
		isSeed:            isSeed,
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
		reader, err := utils.GetReaderHandleByUrl(link)
		if err != nil {
			return spec, err
		}
		defer func(reader io.ReadCloser) {
			err := reader.Close()
			if err != nil {
				L().Errorf("GetTorrentSpec: reader.Close(): %s : %v", link, err)
			}
		}(reader)
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

func (a *AnacrolixTorrentDownloader) AddDownload(link string, listener *MirrorListener, isSeed bool) error {
	dir := path.Join(utils.GetDownloadDir(), utils.ParseInt64ToString(listener.GetUid()))
	spec, err := a.GetTorrentSpec(link)
	if err != nil {
		return err
	}
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		L().Errorf("[ALXTorrent]: AddDownload: os.MkdirAll: %s, %v", link, err)
		return err
	}
	spec.Storage = storage.NewMMap(dir)
	for _, tor := range anacrolixClient.Torrents() {
		if tor.InfoHash().HexString() == spec.InfoHash.HexString() {
			err = os.RemoveAll(dir)
			if err != nil {
				L().Errorf("[ALXTorrent]: AddDownload: os.RemoveAll (torrent already present in client): %s, %v", link, err)
			}
			return fmt.Errorf("infohash %s is already registered in the client", tor.InfoHash().HexString())
		}
	}
	t, _, err := anacrolixClient.AddTorrentSpec(spec)
	if err != nil {
		func() {
			err := os.RemoveAll(dir)
			if err != nil {
				L().Errorf("[ALXTorrent]: AddDownload: os.RemoveAll (failed to add torrent spec): %s, %v", link, err)
			} //we do not want this error to be sent to the user.
		}()
		return err
	}
	listener.isTorrent = true
	listener.isSeed = isSeed
	anacrolixListener := NewAnacrolixTorrentDownloadListener(t, listener, isSeed)
	t.SetOnWriteChunkError(func(err error) {
		t.Drop()
		L().Errorf("[ALXTorrent]: OnWriteChunkError: %v", err)
		anacrolixListener.OnDownloadStop(err)
	})
	anacrolixListener.StartListener()
	gid := utils.RandString(16)
	status := NewAnacrolixTorrentDownloadStatus(gid, listener, anacrolixListener, t)
	status.Index_ = GenerateMirrorIndex()
	AddMirrorLocal(listener.GetUid(), status)
	status.GetListener().OnDownloadStart(status.Gid())
	return nil
}

func NewAnacrolixTorrentDownload(link string, listener *MirrorListener, isSeed bool) error {
	return anacrolixDownloader.AddDownload(link, listener, isSeed)
}

type AnacrolixTorrentDownloadStatus struct {
	gid               string
	listener          *MirrorListener
	anacrolixListener *AnacrolixTorrentDownloadListener
	Index_            int
	isCancelled       bool
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
	if a.anacrolixListener.IsSeeding {
		return a.anacrolixListener.UploadedBytes
	}
	return a.torrentHandle.BytesCompleted()
}

func (a *AnacrolixTorrentDownloadStatus) GetListener() *MirrorListener {
	return a.listener
}

func (a *AnacrolixTorrentDownloadStatus) Speed() int64 {
	if a.anacrolixListener.IsSeeding {
		return a.anacrolixListener.SeedingSpeed
	}
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
	if a.anacrolixListener.IsSeeding {
		if a.CompletedLength() >= a.TotalLength() {
			dur := time.Now().Sub(a.anacrolixListener.SeedStartTime)
			return &dur
		}
	}
	dur := utils.CalculateETA(a.TotalLength()-a.CompletedLength(), a.Speed())
	return &dur
}

func (a *AnacrolixTorrentDownloadStatus) GetStatusType() string {
	if a.isCancelled {
		return MirrorStatusCanceled
	}
	if a.anacrolixListener.IsQueued {
		return MirrorStatusWaiting
	}
	if a.anacrolixListener.IsSeeding {
		return MirrorStatusSeeding
	}
	return MirrorStatusDownloading
}

func (a *AnacrolixTorrentDownloadStatus) Path() string {
	return path.Join(utils.GetDownloadDir(), utils.ParseInt64ToString(a.GetListener().GetUid()), a.Name())
}

func (a *AnacrolixTorrentDownloadStatus) IsTorrent() bool {
	return true
}

func (a *AnacrolixTorrentDownloadStatus) GetPeers() int {
	return a.torrentHandle.Stats().ActivePeers
}

func (a *AnacrolixTorrentDownloadStatus) GetSeeders() int {
	return a.torrentHandle.Stats().ConnectedSeeders
}

func (a *AnacrolixTorrentDownloadStatus) GetCloneListener() *CloneListener {
	return nil
}

func (a *AnacrolixTorrentDownloadStatus) Index() int {
	return a.Index_
}

func (a *AnacrolixTorrentDownloadStatus) CancelMirror() bool {
	a.isCancelled = true
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
