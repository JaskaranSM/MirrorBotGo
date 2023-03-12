package engine

import (
	"MirrorBotGo/utils"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/liut/kedge-go"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

const (
	QueuedForChecking   = 0
	CheckingFiles       = 1 // 1
	DownloadingMetadata = 2 // 2
	Downloading         = 3 // 3
	Finished            = 4 // 4
	Seeding             = 5 // 5
	Allocating          = 6 // 6
	CheckingResumeData  = 7
)

type TorrentStatus struct {
	AddedTime             int64   `json:"added_time"`
	State                 int     `json:"state"`
	Flags                 int     `json:"flags"`
	SavePath              string  `json:"save_path"`
	Name                  string  `json:"name"`
	InfoHash              string  `json:"info_hash"`
	CurrentTracker        string  `json:"current_tracker"`
	NextAnnounce          int64   `json:"next_announce"`
	ActiveDuration        int64   `json:"active_duration"`
	IsFinished            bool    `json:"is_finished"`
	Progress              float64 `json:"progress"`
	ProgressPPM           int     `json:"progress_ppm"`
	Rates                 int64   `json:"rates"`
	TotalDone             int64   `json:"total_done"`
	TotalWanted           int64   `json:"total_wanted"`
	CompletedTime         int64   `json:"completed_time,omitempty"`
	FinishedDuration      int64   `json:"finished_duration,omitempty"`
	SeedingDuration       int64   `json:"seeding_duration,omitempty"`
	TotalDownload         int64   `json:"total_download,omitempty"`
	TotalUpload           int64   `json:"total_upload,omitempty"`
	AllTimeDownload       int64   `json:"all_time_download,omitempty"`
	AllTimeUpload         int64   `json:"all_time_upload,omitempty"`
	TotalPayloadDownload  int64   `json:"total_payload_download,omitempty"`
	TotalPayloadUpload    int64   `json:"total_payload_upload,omitempty"`
	TotalFailedBytes      int64   `json:"total_failed_bytes,omitempty"`
	TotalRedundantBytes   int64   `json:"total_redundant_bytes,omitempty"`
	Total                 int64   `json:"total,omitempty"`
	TotalWantedDone       int64   `json:"total_wanted_done,omitempty"`
	LastSeenComplete      int64   `json:"last_seen_complete,omitempty"`
	LastDownload          int64   `json:"last_download,omitempty"`
	LastUpload            int64   `json:"last_upload,omitempty"`
	DownloadRate          int64   `json:"download_rate,omitempty"`
	UploadRate            int64   `json:"upload_rate,omitempty"`
	DownloadPayloadRate   int64   `json:"download_payload_rate,omitempty"`
	UploadPayloadRate     int64   `json:"upload_payload_rate,omitempty"`
	NumSeeds              int     `json:"num_seeds,omitempty"`
	NumPeers              int     `json:"num_peers,omitempty"`
	NumComplete           int     `json:"num_complete,omitempty"`
	NumIncomplete         int     `json:"num_incomplete,omitempty"`
	ListSeeds             int     `json:"list_seeds,omitempty"`
	ListPeers             int     `json:"list_peers,omitempty"`
	ConnectCandidates     int     `json:"connect_candidates,omitempty"`
	NumPieces             int     `json:"num_pieces,omitempty"`
	DistributedFullCopies int     `json:"distributed_full_copies,omitempty"`
	DistributedFraction   int     `json:"distributed_fraction,omitempty"`
	BlockSize             int     `json:"block_size,omitempty"`
	NumUploads            int     `json:"num_uploads,omitempty"`
	NumConnections        int     `json:"num_connections,omitempty"`
	MovingStorage         bool    `json:"moving_storage,omitempty"`
	IsSeeding             bool    `json:"is_seeding,omitempty"`
	HasMetadata           bool    `json:"has_metadata,omitempty"`
	HasIncoming           bool    `json:"has_incoming,omitempty"`
	Errc                  int     `json:"errc,omitempty"`
}

func (t *TorrentStatus) Marshal() (string, error) {
	data, err := json.MarshalIndent(t, "", " ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (t *TorrentStatus) Unmarshal(data []byte) error {
	return json.Unmarshal(data, t)
}

type TorrentProps struct {
	IsMagnet bool
	Spec     *torrent.TorrentSpec
	Meta     *metainfo.MetaInfo
}

func NewKedgeDownloadListener(client kedge.ClientI, props *TorrentProps, listener *MirrorListener, statusGetter func(string) (*TorrentStatus, error), isSeed bool) *KedgeDownloadListener {
	return &KedgeDownloadListener{
		client:       client,
		props:        props,
		listener:     listener,
		statusGetter: statusGetter,
		isSeed:       isSeed,
	}
}

type KedgeDownloadListener struct {
	client            kedge.ClientI
	props             *TorrentProps
	listener          *MirrorListener
	IsListenerRunning bool
	statusGetter      func(string) (*TorrentStatus, error)
	IsQueued          bool
	haveInfo          bool
	isSeed            bool
	IsSeeding         bool
	IsComplete        bool
	SeedingSpeed      int64
	CompletedTime     time.Time
	UploadedBytes     int64
	SeedStartTime     time.Time
}

func (k *KedgeDownloadListener) OnSeedingStart() {
	L().Infof("[kedge]: OnSeedingStart: %s", k.props.Spec.InfoHash.HexString())
	k.SeedStartTime = time.Now()
	k.listener.OnSeedingStart(k.listener.GetDownload().Gid())
}

func (k *KedgeDownloadListener) OnSeedingError(ratio float32) {
	k.StopListener()
	seedTime := time.Now().Sub(k.CompletedTime)
	text := fmt.Sprintf("Cancelled by user. Ratio: %.2f, SeedTime: %s", ratio, utils.HumanizeDuration(seedTime))
	k.listener.OnSeedingError(fmt.Errorf(text))
}

func (k *KedgeDownloadListener) OnDownloadComplete() {
	k.StopListener()
	L().Info("download complete kedge")
	if k.isSeed {
		k.IsSeeding = true
		k.OnSeedingStart()
	}
	k.listener.OnDownloadComplete()
}

func (k *KedgeDownloadListener) OnMetadataDownloadComplete() {
	k.haveInfo = true
	L().Info("kedge metadata complete")
}

func (k *KedgeDownloadListener) OnDownloadStop(err error) {
	k.StopListener()
	L().Error(err)
	k.listener.OnDownloadError(err.Error())
}

func (k *KedgeDownloadListener) OnDownloadStart() {
	L().Info("download start kedge")
}

func (k *KedgeDownloadListener) StartListener() {
	k.IsListenerRunning = true
	go k.ListenForEvents()
}

func (k *KedgeDownloadListener) StopListener() {
	k.IsListenerRunning = false
}

func (k *KedgeDownloadListener) ListenForEvents() {
	k.OnDownloadStart()
	for k.IsListenerRunning {
		stats, err := k.statusGetter(k.props.Spec.InfoHash.HexString())
		if err != nil {
			k.OnDownloadStop(err)
			break
		}
		if stats.Errc != 0 {
			L().Error(stats.Marshal())
			k.OnDownloadStop(fmt.Errorf("got error code from kedge: %d", stats.Errc))
			break
		}
		if stats.HasMetadata {
			if !k.haveInfo {
				k.OnMetadataDownloadComplete()
			}
		}
		if stats.IsFinished {
			k.CompletedTime = time.Unix(stats.CompletedTime, 0)
			k.OnDownloadComplete()
		}
		time.Sleep(1 * time.Second)
	}
}

func NewKedgeDownloader(client kedge.ClientI, httpClient *http.Client, uri string) *KedgeDownloader {
	return &KedgeDownloader{
		client:     client,
		httpClient: httpClient,
		URI:        uri,
	}
}

type KedgeDownloader struct {
	client     kedge.ClientI
	httpClient *http.Client
	URI        string
}

func (k *KedgeDownloader) AddTorrent(reader io.Reader, dir string, isMagnet bool) error {
	uri := k.URI + "/torrents"
	req, err := http.NewRequest(http.MethodPost, uri, reader)
	if err != nil {
		return err
	}
	req.Header.Set("x-save-path", dir)
	if isMagnet {
		req.Header.Set("Content-Type", "text/plain")
	} else {
		req.Header.Set("Content-Type", "application/x-bittorrent")
	}
	res, err := k.httpClient.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode >= 400 {
		L().Errorf("got response code of %d", res.StatusCode)
		return fmt.Errorf("got response code of %d", res.StatusCode)
	}
	defer res.Body.Close()
	return nil
}

func (k *KedgeDownloader) GetTorrentStatus(hash string) (*TorrentStatus, error) {
	uri := k.URI + "/torrent/" + hash
	res, err := k.httpClient.Get(uri)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		L().Errorf("got response code of %d", res.StatusCode)
		return nil, fmt.Errorf("got response code of %d", res.StatusCode)
	}
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var status TorrentStatus
	err = status.Unmarshal(data)
	return &status, err
}

func (k *KedgeDownloader) PauseTorrent(hash string) error {
	uri := k.URI + "/torrent/" + hash + "/toggle"
	req, err := http.NewRequest(http.MethodPut, uri, nil)
	if err != nil {
		return err
	}
	res, err := k.httpClient.Do(req)
	if res.StatusCode >= 400 {
		L().Errorf("got response code of %d", res.StatusCode)
		return fmt.Errorf("got response code of %d", res.StatusCode)
	}
	return nil
}

func (k *KedgeDownloader) GetTorrentSpec(link string) (*TorrentProps, error) {
	var spec *torrent.TorrentSpec
	var err error
	var props *TorrentProps = &TorrentProps{IsMagnet: utils.IsMagnetLink(link)}
	if props.IsMagnet {
		spec, err = torrent.TorrentSpecFromMagnetUri(link)
		if err != nil {
			return props, err
		}
		props.Spec = spec
	} else {
		reader, err := utils.GetReaderHandleByUrl(link)
		if err != nil {
			return props, err
		}
		defer func(reader io.ReadCloser) {
			err := reader.Close()
			if err != nil {
				L().Errorf("GetTorrentSpec: reader.Close(): %s : %v", link, err)
			}
		}(reader)
		meta, err := metainfo.Load(reader)
		if err != nil {
			return props, err
		}
		props.Meta = meta
		spec, err = torrent.TorrentSpecFromMetaInfoErr(meta)
		if err != nil {
			return props, err
		}
		props.Spec = spec
	}
	return props, err
}

func (k *KedgeDownloader) TorrentExists(spec *torrent.TorrentSpec) (bool, error) {
	torrents, err := k.client.GetTorrents()
	if err != nil {
		return false, err
	}
	L().Info(torrents)
	for i := range torrents {
		if string(torrents[i].Infohash) == spec.InfoHash.HexString() {
			return true, nil
		}
	}
	return false, nil
}

func (k *KedgeDownloader) PrepDownload(gid string, link string, dir string, listener *MirrorListener, index int, isSeed bool) {
	var err error
	props, err := k.GetTorrentSpec(link)
	if err != nil {
		listener.OnDownloadError(err.Error())
		return
	}
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		L().Errorf("[kedge]: AddDownload: os.MkdirAll: %s, %v", link, err)
		listener.OnDownloadError(err.Error())
		return
	}
	exists, err := k.TorrentExists(props.Spec)
	if err != nil {
		L().Errorf("[kedge]: AddDownload: TorrentExists: %s, %v", link, err)
		listener.OnDownloadError(err.Error())
		return
	}
	if exists {
		err = os.RemoveAll(dir)
		if err != nil {
			L().Errorf("[kedge]: AddDownload: os.RemoveAll (torrent already present in client): %s, %v", link, err)
		}
		listener.OnDownloadError(fmt.Sprintf("infohash %s is already registered in the client", props.Spec.InfoHash.HexString()))
		return
	}
	if props.IsMagnet {
		err = k.AddTorrent(strings.NewReader(link), dir, true)
	} else {
		var buffer bytes.Buffer
		err = props.Meta.Write(&buffer)
		if err != nil {
			L().Errorf("[kedge]: AddDownload: trying to write metainfo to buffer: %v", err)
			listener.OnDownloadError(err.Error())
			return
		}
		err = k.AddTorrent(&buffer, dir, false)
	}
	if err != nil {
		L().Errorf("[kedge]: AddDownload: trying to write metainfo to buffer: %v", err)
		listener.OnDownloadError(err.Error())
		return
	}
	listener.isTorrent = true
	listener.isSeed = isSeed
	kedgeListener := NewKedgeDownloadListener(k.client, props, listener, k.GetTorrentStatus, isSeed)
	kedgeListener.StartListener()
	status := NewKedgeDownloadStatus(gid, listener, kedgeListener, k.GetTorrentStatus, k.client.Drop, props)
	status.Index_ = index
	AddMirrorLocal(listener.GetUid(), status)
	status.GetListener().OnDownloadStart(status.Gid())
}

func (k *KedgeDownloader) AddDownload(link string, listener *MirrorListener, isSeed bool) error {
	dir := path.Join(utils.GetDownloadDir(), utils.ParseInt64ToString(listener.GetUid()))
	gid := utils.RandString(16)
	initializingStatus := NewInitializingStatus(utils.TrimString(link), gid, dir, listener)
	initializingStatus.Index_ = GenerateMirrorIndex()
	AddMirrorLocal(listener.GetUid(), initializingStatus)
	go k.PrepDownload(gid, link, dir, listener, initializingStatus.Index(), isSeed)
	return nil
}

func NewKedgeDownload(link string, listener *MirrorListener, isSeed bool) error {
	kedgeDownloader := NewKedgeDownloader(kedge.New(), &http.Client{}, utils.GetKedgeURL())
	return kedgeDownloader.AddDownload(link, listener, isSeed)
}

func NewKedgeDownloadStatus(gid string, listener *MirrorListener, kedgeLister *KedgeDownloadListener, statusGetter func(string) (*TorrentStatus, error), pauseTorrent func(string) error, props *TorrentProps) *KedgeDownloadStatus {
	return &KedgeDownloadStatus{
		gid:           gid,
		listener:      listener,
		kedgeListener: kedgeLister,
		statusGetter:  statusGetter,
		pauseTorrent:  pauseTorrent,
		props:         props,
	}
}

type KedgeDownloadStatus struct {
	gid           string
	listener      *MirrorListener
	kedgeListener *KedgeDownloadListener
	statusGetter  func(string) (*TorrentStatus, error)
	pauseTorrent  func(string) error
	props         *TorrentProps
	Index_        int
	isCanceled    bool
	lastStats     *TorrentStatus
}

func (k *KedgeDownloadStatus) pullStatus() *TorrentStatus {
	var torrentStatus TorrentStatus
	stats, err := k.statusGetter(k.props.Spec.InfoHash.HexString())
	if err != nil {
		L().Error(err)
	} else {
		torrentStatus = *stats
	}
	if torrentStatus.Name == "" {
		torrentStatus.Name = fmt.Sprintf("infohash:%s", torrentStatus.InfoHash)
	}
	return &torrentStatus
}

func (k *KedgeDownloadStatus) Name() string {
	if k.lastStats != nil {
		return k.lastStats.Name
	}
	stats := k.pullStatus()
	if stats.Name == "" {
		return "N.A"
	}
	return stats.Name
}

func (k *KedgeDownloadStatus) TotalLength() int64 {
	if !k.kedgeListener.haveInfo {
		return 0
	}
	if k.lastStats != nil {
		return k.lastStats.TotalWanted
	}
	return k.pullStatus().TotalWanted
}

func (k *KedgeDownloadStatus) CompletedLength() int64 {
	if k.lastStats != nil {
		return k.lastStats.TotalDone
	}
	return k.pullStatus().TotalDone
}

func (k *KedgeDownloadStatus) GetListener() *MirrorListener {
	return k.listener
}

func (k *KedgeDownloadStatus) Speed() int64 {
	if k.kedgeListener.IsSeeding {
		return k.pullStatus().UploadRate
	}
	return k.pullStatus().DownloadRate
}

func (k *KedgeDownloadStatus) Gid() string {
	return k.gid
}

func (k *KedgeDownloadStatus) Percentage() float32 {
	if k.CompletedLength() == 0 {
		return float32(0.00)
	}
	return float32(k.CompletedLength()*100) / float32(k.TotalLength())
}

func (k *KedgeDownloadStatus) ETA() *time.Duration {
	dur := utils.CalculateETA(k.TotalLength()-k.CompletedLength(), k.Speed())
	return &dur
}

func (k *KedgeDownloadStatus) GetStatusType() string {
	if k.isCanceled {
		return MirrorStatusCanceled
	}
	if k.kedgeListener.IsQueued {
		return MirrorStatusWaiting
	}
	if k.kedgeListener.IsSeeding {
		return MirrorStatusSeeding
	}
	return MirrorStatusDownloading
}

func (k *KedgeDownloadStatus) Path() string {
	return path.Join(utils.GetDownloadDir(), utils.ParseInt64ToString(k.GetListener().GetUid()), k.Name())
}

func (k *KedgeDownloadStatus) IsTorrent() bool {
	return true
}

func (k *KedgeDownloadStatus) PiecesCompleted() int {
	stats := k.pullStatus()
	return stats.NumPieces
}

func (k *KedgeDownloadStatus) PiecesTotal() int {
	//TODO: find a way to do it properly from kedge API
	//because we need at least one completed piece to determine the piece size
	completedPieces := k.PiecesCompleted()
	if completedPieces == 0 {
		return 0
	}
	completedLength := k.CompletedLength()
	pieceSize := completedLength / int64(completedPieces)
	return int(k.TotalLength() / pieceSize)
}

func (k *KedgeDownloadStatus) GetPeers() int {
	return k.pullStatus().NumPeers
}

func (k *KedgeDownloadStatus) GetSeeders() int {
	return k.pullStatus().NumSeeds
}

func (k *KedgeDownloadStatus) GetCloneListener() *CloneListener {
	return nil
}

func (k *KedgeDownloadStatus) Index() int {
	return k.Index_
}

func (k *KedgeDownloadStatus) CancelMirror() bool {
	if k.isCanceled {
		return true
	}
	stats := k.pullStatus()
	k.lastStats = stats //cache last stats because we are removing torrent from kedge at cancellation
	k.isCanceled = true
	k.kedgeListener.StopListener()
	err := k.pauseTorrent(k.props.Spec.InfoHash.HexString())
	if err != nil {
		L().Errorf("kedge cancelMirror: %v", err)
	}
	if k.kedgeListener.IsSeeding {
		ratio := float32(stats.TotalUpload) / float32(stats.TotalWanted)
		k.kedgeListener.OnSeedingError(ratio)
	} else {
		k.kedgeListener.OnDownloadStop(fmt.Errorf("canceled by user"))
	}
	return true
}
