//go:build !disable_mega
// +build !disable_mega

package engine

import (
	"MirrorBotGo/utils"
	"os"
	"path"
	"sync"
	"time"

	"github.com/jaskaranSM/megasdkgo"
)

var megaMutex sync.Mutex
var megaClient *megasdkgo.MegaClient = NewMegaClient()

func NewMegaClient() *megasdkgo.MegaClient {
	client := megasdkgo.NewMegaClient(utils.GetMegaAPIKey())
	megaMutex.Lock()
	defer megaMutex.Unlock()
	err := client.Login(utils.GetMegaEmail(), utils.GetMegaPasssword())
	if err != nil {
		L().Errorf("mega login failed: %s", err.Error())
	}
	return client
}

func NewMegaDownload(link string, listener *MirrorListener) error {
	dir := path.Join(utils.GetDownloadDir(), utils.ParseInt64ToString(listener.GetUid()))
	os.MkdirAll(dir, 0755)
	megaMutex.Lock()
	defer megaMutex.Unlock()
	gid, err := megaClient.AddDownload(link, dir)
	if err != nil {
		return err
	}
	go func() {
		state := megasdkgo.StateActive
		do := true
		for do {
			switch state {
			case megasdkgo.StateFailed:
				do = false
			case megasdkgo.StateCancelled:
				do = false
			case megasdkgo.StateCompleted:
				do = false
			}
			megaMutex.Lock()
			stats := megaClient.GetDownloadInfo(gid)
			if stats.Gid == "" {
				state = megasdkgo.StateFailed
				do = false
				megaMutex.Unlock()
				continue
			}
			megaMutex.Unlock()
			state = stats.State
			time.Sleep(500 * time.Millisecond)
		}
		if state == megasdkgo.StateFailed || state == megasdkgo.StateCancelled {
			megaMutex.Lock()
			stats := megaClient.GetDownloadInfo(gid)
			megaMutex.Unlock()
			dl := GetMirrorByGid(gid)
			if dl != nil {
				if state == megasdkgo.StateCancelled {
					dl.GetListener().OnDownloadError("Canceled by user")
				} else {
					dl.GetListener().OnDownloadError(stats.ErrorString)
				}
				return
			}
		}
		listener.OnDownloadComplete()
	}()
	status := NewMegaDownloadStatus(gid, listener)
	status.Index_ = GenerateMirrorIndex()
	AddMirrorLocal(listener.GetUid(), status)
	status.GetListener().OnDownloadStart(status.Gid())
	return nil
}

type MegaDownloadStatus struct {
	gid      string
	listener *MirrorListener
	Index_   int
}

func (m *MegaDownloadStatus) GetStats() *megasdkgo.DownloadInfo {
	return megaClient.GetDownloadInfo(m.Gid())
}

func (m *MegaDownloadStatus) Name() string {
	stats := m.GetStats()
	return stats.Name
}

func (m *MegaDownloadStatus) CompletedLength() int64 {
	stats := m.GetStats()
	return stats.CompletedLength
}

func (m *MegaDownloadStatus) TotalLength() int64 {
	stats := m.GetStats()
	return stats.TotalLength
}

func (m *MegaDownloadStatus) Speed() int64 {
	stats := m.GetStats()
	return stats.Speed
}

func (m *MegaDownloadStatus) Gid() string {
	return m.gid
}

func (m *MegaDownloadStatus) ETA() *time.Duration {
	if m.Speed() != 0 {
		dur := utils.CalculateETA(m.TotalLength()-m.CompletedLength(), m.Speed())
		return &dur
	}
	dur := time.Duration(0)
	return &dur
}

func (m *MegaDownloadStatus) GetStatusType() string {
	stats := m.GetStats()
	if stats.State == megasdkgo.StateFailed {
		return MirrorStatusFailed
	} else if stats.State == megasdkgo.StateCancelled {
		return MirrorStatusCanceled
	} else if stats.State == megasdkgo.StateQueued {
		return MirrorStatusWaiting
	}
	return MirrorStatusDownloading
}

func (m *MegaDownloadStatus) Path() string {
	return path.Join(utils.GetDownloadDir(), utils.ParseInt64ToString(m.GetListener().GetUid()), m.Name())
}

func (m *MegaDownloadStatus) Percentage() float32 {
	return float32(m.CompletedLength()*100) / float32(m.TotalLength())
}

func (m *MegaDownloadStatus) IsTorrent() bool {
	return false
}

func (m *MegaDownloadStatus) GetPeers() int {
	return 0
}

func (m *MegaDownloadStatus) GetSeeders() int {
	return 0
}

func (m *MegaDownloadStatus) GetListener() *MirrorListener {
	return m.listener
}

func (m *MegaDownloadStatus) GetCloneListener() *CloneListener {
	return nil
}

func (m *MegaDownloadStatus) CancelMirror() bool {
	megaClient.CancelDownload(m.Gid())
	return true
}

func (m *MegaDownloadStatus) Index() int {
	return m.Index_
}

func NewMegaDownloadStatus(gid string, listener *MirrorListener) *MegaDownloadStatus {
	return &MegaDownloadStatus{
		gid:      gid,
		listener: listener,
	}
}
