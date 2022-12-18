package engine

import (
	"MirrorBotGo/utils"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"sync"
	"time"
)

const (
	MegaSDKRestStateNone       = 0
	MegaSDKRestStateQueued     = 1
	MegaSDKRestStateActive     = 2
	MegaSDKRestStatePaused     = 3
	MegaSDKRestStateRetrying   = 4
	MegaSDKRestStateCompleting = 5
	MegaSDKRestStateCompleted  = 6
	MegaSDKRestStateCancelled  = 7
	MegaSDKRestStateFailed     = 8

	MegaNoError         = 0
	MegaInfoNotProvided = 401
	MegaNotFound        = 404
)

var megaLoggedIn bool = false

type MegaSDKRestResp struct {
	Gid         string `json:"gid"`
	ErrorCode   int    `json:"error_code"`
	ErrorString string `json:"error_string"`
}

func (m *MegaSDKRestResp) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

type MegaSDKRestDownloadInfo struct {
	ErrorCode       int    `json:"error_code"`
	ErrorString     string `json:"error_string"`
	Name            string `json:"name"`
	Speed           int64  `json:"speed"`
	CompletedLength int64  `json:"completed_length"`
	TotalLength     int64  `json:"total_length"`
	State           int    `json:"state"`
	IsCompleted     bool   `json:"is_completed"`
	IsFailed        bool   `json:"is_failed"`
	IsCancelled     bool   `json:"is_cancelled"`
}

func (m *MegaSDKRestDownloadInfo) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

func NewMegaSDKRestClient(apiURL string) *MegaSDKRestClient {
	return &MegaSDKRestClient{
		apiURL: apiURL,
	}
}

type MegaSDKRestClient struct {
	apiURL string
	mut    sync.Mutex
}

func (m *MegaSDKRestClient) checkAndRaiseError(errorCode int, errorString string) error {
	if errorCode == MegaNoError {
		return nil
	}
	if errorString != "" {
		return fmt.Errorf("MegaSDKRestpp: %s : %d", errorString, errorCode)
	}
	if errorCode == MegaInfoNotProvided {
		return fmt.Errorf("MegaSdkRestpp: info not provided properly: %s %d", errorString, errorCode)
	}
	if errorCode == MegaNotFound {
		return fmt.Errorf("MegaSdkRestpp: not found: %s %d", errorString, errorCode)
	}
	return nil
}

func (m *MegaSDKRestClient) checkLogin() {
	if megaLoggedIn {
		return
	}
	_, err := m.Login(utils.GetMegaEmail(), utils.GetMegaPassword())
	if err != nil {
		L().Errorf("MegaSDKRest Login: %s", err.Error())
	}
	megaLoggedIn = true
}

func (m *MegaSDKRestClient) Login(email string, password string) (*MegaSDKRestResp, error) {
	res, err := http.PostForm(m.apiURL+"/login", url.Values{
		"email":    []string{email},
		"password": []string{password},
	})
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			L().Errorf(" MegaSDKRestClient: Login: failed to close response body: %v", err)
		}
	}(res.Body)
	resData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var login *MegaSDKRestResp = &MegaSDKRestResp{}
	err = login.Unmarshal(resData)
	if err != nil {
		return nil, err
	}
	return login, m.checkAndRaiseError(login.ErrorCode, login.ErrorString)
}

func (m *MegaSDKRestClient) AddDownload(link string, dir string) (*MegaSDKRestResp, error) {
	m.mut.Lock()
	defer m.mut.Unlock()
	m.checkLogin()
	res, err := http.PostForm(m.apiURL+"/adddownload", url.Values{
		"link": []string{link},
		"dir":  []string{dir},
	})
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			L().Errorf(" MegaSDKRestClient: AddDl: failed to close response body: %v", err)
		}
	}(res.Body)
	resData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var adddl *MegaSDKRestResp = &MegaSDKRestResp{}
	err = adddl.Unmarshal(resData)
	if err != nil {
		return nil, err
	}
	return adddl, m.checkAndRaiseError(adddl.ErrorCode, adddl.ErrorString)
}

func (m *MegaSDKRestClient) CancelDownload(gid string) error {
	m.mut.Lock()
	defer m.mut.Unlock()
	res, err := http.PostForm(m.apiURL+"/canceldownload", url.Values{
		"gid": []string{gid},
	})
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			L().Errorf(" MegaSDKRestClient: CancelDl: failed to close response body: %v", err)
		}
	}(res.Body)
	resData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	var cancel *MegaSDKRestResp = &MegaSDKRestResp{}
	err = cancel.Unmarshal(resData)
	if err != nil {
		return err
	}
	return m.checkAndRaiseError(cancel.ErrorCode, cancel.ErrorString)
}

func (m *MegaSDKRestClient) GetDownloadInfo(gid string) (*MegaSDKRestDownloadInfo, error) {
	m.mut.Lock()
	defer m.mut.Unlock()
	res, err := http.PostForm(m.apiURL+"/getstatus", url.Values{
		"gid": []string{gid},
	})
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			L().Errorf(" MegaSDKRestClient: GetDownloadInfo: failed to close response body: %v", err)
		}
	}(res.Body)
	resData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var downloadInfo *MegaSDKRestDownloadInfo = &MegaSDKRestDownloadInfo{}
	err = downloadInfo.Unmarshal(resData)
	if err != nil {
		return nil, err
	}
	return downloadInfo, m.checkAndRaiseError(downloadInfo.ErrorCode, downloadInfo.ErrorString)
}

var megaClient *MegaSDKRestClient = NewMegaSDKRestClient("http://localhost:8069")

func PerformMegaLogin() error {
	_, err := megaClient.Login(utils.GetMegaEmail(), utils.GetMegaPassword())
	if err != nil {
		L().Errorf("MegaSDKRest: %s", err.Error())
	}
	return err
}

func NewMegaDownloadListener(gid string, listener *MirrorListener) *MegaDownloadListener {
	return &MegaDownloadListener{
		gid:               gid,
		listener:          listener,
		isListenerRunning: false,
	}
}

type MegaDownloadListener struct {
	gid               string
	listener          *MirrorListener
	isListenerRunning bool
}

func (m *MegaDownloadListener) StartListener() {
	m.isListenerRunning = true
	go m.ListenForEvents()
}

func (m *MegaDownloadListener) StopListener() {
	m.isListenerRunning = false
}

func (m *MegaDownloadListener) OnDownloadError(err error) {
	m.StopListener()
	m.listener.OnDownloadError(err.Error())
}

func (m *MegaDownloadListener) OnDownloadComplete() {
	m.StopListener()
	m.listener.OnDownloadComplete()
}

func (m *MegaDownloadListener) ListenForEvents() {
	for m.isListenerRunning {
		status, err := megaClient.GetDownloadInfo(m.gid)
		if err != nil {
			m.OnDownloadError(err)
			continue
		}
		if status.IsFailed {
			m.OnDownloadError(fmt.Errorf("%s", status.ErrorString))
			continue
		} else if status.IsCancelled {
			m.OnDownloadError(fmt.Errorf("cancelled by user"))
			continue
		} else if status.IsCompleted {
			m.OnDownloadComplete()
			continue
		}
		time.Sleep(1 * time.Second)
	}
}

func NewMegaDownload(link string, listener *MirrorListener) error {
	dir := path.Join(utils.GetDownloadDir(), utils.ParseInt64ToString(listener.GetUid()))
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		L().Errorf("NewMegaDownload: os.MkdirAll: %s : %v", dir, err)
		return err
	}
	adddl, err := megaClient.AddDownload(link, dir)
	if err != nil {
		return err
	}
	if adddl.Gid == "" {
		return fmt.Errorf("MegaSDKRestpp: internal error occured")
	}
	megaDownloadListener := NewMegaDownloadListener(adddl.Gid, listener)
	megaDownloadListener.StartListener()
	status := NewMegaDownloadStatus(adddl.Gid, listener)
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

func (m *MegaDownloadStatus) GetStats() *MegaSDKRestDownloadInfo {
	dlinfo, err := megaClient.GetDownloadInfo(m.Gid())
	if err != nil {
		L().Errorf("MegaDownloadStatus: %s", err.Error())
	}
	if dlinfo == nil {
		dlinfo = &MegaSDKRestDownloadInfo{}
	}
	return dlinfo
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
	switch stats.State {
	case MegaSDKRestStateFailed:
		return MirrorStatusFailed
	case MegaSDKRestStateCancelled:
		return MirrorStatusCanceled
	case MegaSDKRestStateQueued:
		return MirrorStatusWaiting
	default:
		return MirrorStatusDownloading
	}
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
	err := megaClient.CancelDownload(m.Gid())
	if err != nil {
		L().Errorf("MegaDownloadStatus: CancelMirror: %s: %s: %v", m.Name(), m.Gid(), err)
		return false
	}
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
