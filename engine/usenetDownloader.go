package engine

import (
	"MirrorBotGo/utils"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"golift.io/nzbget"
)

var usenetClient *nzbget.NZBGet = getUsenetClient()
var usenetActiveDls []string
var usenetMutex sync.Mutex
var isRPCConnected bool = false

var GroupRespNotFoundErr = errors.New("(GroupResp): NZB download not found")
var HistoryRespNotFoundErr = errors.New("(HistoryResp): NZB download not found")

func isUsenetDlExist(dl string) bool {
	usenetMutex.Lock()
	defer usenetMutex.Unlock()
	for _, t := range usenetActiveDls {
		if t == dl {
			return true
		}
	}
	return false
}

func getUsenetDlIndex(dl string) int {
	usenetMutex.Lock()
	defer usenetMutex.Unlock()
	for i, t := range usenetActiveDls {
		if t == dl {
			return i
		}
	}
	return -1
}

func addUsenetActiveDl(dl string) {
	usenetMutex.Lock()
	defer usenetMutex.Unlock()
	usenetActiveDls = append(usenetActiveDls, dl)
}

func removeUsenetActiveDl(dl string) {
	index := getUsenetDlIndex(dl)
	if index == -1 {
		return
	}
	usenetMutex.Lock()
	defer usenetMutex.Unlock()
	usenetActiveDls[index] = ""
}

func getUsenetClient() *nzbget.NZBGet {
	nzb := nzbget.New(&nzbget.Config{
		URL:  utils.GetUsenetClientURL(),
		User: utils.GetUsenetClientUsername(),
		Pass: utils.GetUsenetClientPassword(),
		Client: &http.Client{
			Timeout: 20 * time.Second,
		},
	})
	events, err := nzb.Log(0, 100)
	if err != nil {
		return nil
	}
	isRPCConnected = true
	for _, event := range events {
		L().Info(event.ID, event.Kind, event.Time, event.Text)
	}
	conf, err := nzb.Config()
	if err != nil {
		return nzb
	}
	for _, c := range conf {
		L().Infof("%s = %s", c.Name, c.Value)
	}
	return nzb
}

func GetGroupRespByNZBID(nzbID int64) (*nzbget.Group, error) {
	groups, err := usenetClient.ListGroups()
	if err != nil {
		return nil, err
	}
	for _, group := range groups {
		if group.NZBID == nzbID {
			return group, nil
		}
	}
	return nil, GroupRespNotFoundErr
}

func GetHistoryRespByNZBID(nzbID int64) (*nzbget.History, error) {
	histories, err := usenetClient.History(false)
	if err != nil {
		return nil, err
	}
	for _, history := range histories {
		if history.NZBID == nzbID {
			return history, nil
		}
	}
	return nil, HistoryRespNotFoundErr
}

func NewUsenetDownloadListener(nzbID int64, listener *MirrorListener, content string) *UsenetDownloadListener {
	return &UsenetDownloadListener{
		NzbID:    nzbID,
		listener: listener,
		Content:  content,
	}
}

type UsenetDownloadListener struct {
	Content           string
	NzbID             int64
	handled           bool
	listener          *MirrorListener
	IsListenerRunning bool
	Speed             int64
	pth               string
	futurePath        string
}

func (u *UsenetDownloadListener) StartListener() {
	u.IsListenerRunning = true
	go u.ListenForEvents()
}

func (u *UsenetDownloadListener) StopListener() {
	u.IsListenerRunning = false
}

func (u *UsenetDownloadListener) OnDownloadComplete() {
	if u.handled {
		return
	}
	u.handled = true
	removeUsenetActiveDl(u.Content)
	_, err := usenetClient.EditQueue("GroupDelete", "", []int64{u.NzbID})
	if err != nil {
		L().Errorf("UsenetDownloadListener: OnDownloadComplete: EditQueue: GroupDelete: %d : %v", u.NzbID, err)
		return
	}
	u.StopListener()
	u.listener.OnDownloadComplete()
}

func (u *UsenetDownloadListener) OnDownloadError(err string) {
	if u.handled {
		return
	}
	u.handled = true
	removeUsenetActiveDl(u.Content)
	_, err2 := usenetClient.EditQueue("GroupDelete", "", []int64{u.NzbID})
	if err2 != nil {
		L().Errorf("UsenetDownloadListener: OnDownloadError: EditQueue: GroupDelete: %d : %v", u.NzbID, err2)
		return
	}
	u.StopListener()
	u.listener.OnDownloadError(err)
}

func (u *UsenetDownloadListener) ListenForEvents() {
	var last int64 = 0
	for u.IsListenerRunning {
		group, err := GetGroupRespByNZBID(u.NzbID)
		if err == GroupRespNotFoundErr {
			history, err := GetHistoryRespByNZBID(u.NzbID)
			if err != nil {
				L().Error(err)
			}
			if strings.Contains(history.Status, "SUCCESS") {
				u.pth = path.Join(u.futurePath, history.Name)
				L().Infof("[UsenetRename]: %s -> %s", history.DestDir, u.pth)
				L().Error(os.Rename(history.DestDir, u.pth))
				u.OnDownloadComplete()
				return
			} else if strings.Contains(history.Status, "FAILURE") {
				u.pth = path.Join(u.futurePath, history.Name)
				err := os.Rename(history.DestDir, u.pth)
				if err != nil {
					L().Errorf("UsenetDownloadListener: usenet download fail: rename: %s -> %s : %v", history.DestDir, u.pth, err)
					return
				}
				u.OnDownloadError("usenet download failed")
				return
			}
		}
		if group == nil {
			continue
		}
		if group.Status == nzbget.GroupPAUSED {
			u.OnDownloadError("Canceled by user.")
			return
		}
		completed := group.DownloadedSizeMB * 1024 * 1024
		chunk := completed - last
		last = group.DownloadedSizeMB * 1024 * 1024
		u.Speed = chunk
		time.Sleep(1 * time.Second)
	}
}

type UsenetDownloadStatusStruct struct {
	Name      string
	Completed int64
	Total     int64
	Path      string
	IsQueued  bool
}

func NewUsenetDownloadStatus(gid string, usenetListener *UsenetDownloadListener, nzbID int64) *UsenetDownloadStatus {
	return &UsenetDownloadStatus{
		gid:                    gid,
		usenetDownloadListener: usenetListener,
		nzbID:                  nzbID,
	}
}

type UsenetDownloadStatus struct {
	gid                    string
	usenetDownloadListener *UsenetDownloadListener
	nzbID                  int64
	isCancelled            bool
	Index_                 int
}

func (u *UsenetDownloadStatus) GetStatus() UsenetDownloadStatusStruct {
	var status UsenetDownloadStatusStruct
	group, err := GetGroupRespByNZBID(u.nzbID)
	if err == GroupRespNotFoundErr {
		history, err := GetHistoryRespByNZBID(u.nzbID)
		if err != nil {
			L().Error(err)
			return status
		}
		status.Name = history.Name
		status.Completed = history.DownloadedSizeMB * 1024 * 1024
		status.Total = history.FileSizeMB * 1024 * 1024
		status.Path = history.DestDir
		return status
	}
	status.Name = group.NZBName
	status.Completed = group.DownloadedSizeMB * 1024 * 1024
	status.Total = group.FileSizeMB * 1024 * 1024
	status.Path = group.DestDir
	if group.Status == nzbget.GroupPPQUEUED || group.Status == nzbget.GroupQUEUED {
		status.IsQueued = true
	}
	return status
}

func (u *UsenetDownloadStatus) Name() string {
	return u.GetStatus().Name
}

func (u *UsenetDownloadStatus) TotalLength() int64 {
	return u.GetStatus().Total
}

func (u *UsenetDownloadStatus) CompletedLength() int64 {
	return u.GetStatus().Completed
}

func (u *UsenetDownloadStatus) GetListener() *MirrorListener {
	return u.usenetDownloadListener.listener
}

func (u *UsenetDownloadStatus) Speed() int64 {
	return u.usenetDownloadListener.Speed
}

func (u *UsenetDownloadStatus) Gid() string {
	return u.gid
}

func (u *UsenetDownloadStatus) Percentage() float32 {
	if u.CompletedLength() == 0 {
		return float32(0.00)
	}
	return float32(u.CompletedLength()*100) / float32(u.TotalLength())
}

func (u *UsenetDownloadStatus) ETA() *time.Duration {
	dur := utils.CalculateETA(u.TotalLength()-u.CompletedLength(), u.Speed())
	return &dur
}

func (u *UsenetDownloadStatus) GetStatusType() string {
	if u.isCancelled {
		return MirrorStatusCanceled
	}
	if u.GetStatus().IsQueued {
		return MirrorStatusWaiting
	}
	return MirrorStatusDownloading
}

func (u *UsenetDownloadStatus) Path() string {
	if u.usenetDownloadListener.pth != "" {
		return u.usenetDownloadListener.pth
	}
	return u.GetStatus().Path
}

func (u *UsenetDownloadStatus) IsTorrent() bool {
	return false
}

func (u *UsenetDownloadStatus) GetPeers() int {
	return 0
}

func (u *UsenetDownloadStatus) GetSeeders() int {
	return 0
}

func (u *UsenetDownloadStatus) GetCloneListener() *CloneListener {
	return nil
}

func (u *UsenetDownloadStatus) Index() int {
	return u.Index_
}

func (u *UsenetDownloadStatus) CancelMirror() bool {
	u.isCancelled = true
	dun, err := usenetClient.EditQueue("GroupPause", "", []int64{u.nzbID})
	if err != nil {
		L().Error(err)
	}
	return dun
}

func NewUsenetDownload(filename string, link string, listener *MirrorListener) error {
	if !isRPCConnected {
		return fmt.Errorf("NZBGet RPC isnt connected atm.")
	}
	L().Info(link)
	reader, err := utils.GetReaderHandleByUrl(link)
	if err != nil {
		return err
	}
	defer func(reader io.ReadCloser) {
		err := reader.Close()
		if err != nil {
			L().Errorf("NewUsenetDownload: failed to close reader handle: %s: %v", link, err)
		}
	}(reader)
	content, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}
	base64Encoded := base64.StdEncoding.EncodeToString(content)
	if isUsenetDlExist(base64Encoded) {
		return errors.New("this usenet download is already in queue, let the mirror finish and then add again")
	}
	dir := path.Join(utils.GetDownloadDir(), utils.ParseInt64ToString(listener.GetUid()))
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		L().Errorf("NewUsenetDownload: os.MkdirAll: %s : %v", dir, err)
		return err
	}
	nzbID, err := usenetClient.Append(&nzbget.AppendInput{
		Filename: filename,
		Content:  base64Encoded,
		DupeMode: "force",
	})
	if err != nil {
		return err
	}
	usenetListener := NewUsenetDownloadListener(nzbID, listener, base64Encoded)
	usenetListener.futurePath = dir
	usenetListener.StartListener()
	gid := utils.RandString(16)
	status := NewUsenetDownloadStatus(gid, usenetListener, nzbID)
	status.Index_ = GenerateMirrorIndex()
	AddMirrorLocal(listener.GetUid(), status)
	addUsenetActiveDl(base64Encoded)
	status.GetListener().OnDownloadStart(status.Gid())
	return err
}
