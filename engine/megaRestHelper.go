package engine

import (
	"MirrorBotGo/utils"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
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

type MegaSDKRestReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Link     string `json:"link"`
	Dir      string `json:"dir"`
	Gid      string `json:"gid"`
}

func (m *MegaSDKRestReq) Marshal() ([]byte, error) {
	return json.MarshalIndent(m, "", " ")
}

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

func NewMegaSDKRestClient(apiURL string, client *http.Client) *MegaSDKRestClient {
	return &MegaSDKRestClient{
		apiURL: apiURL,
		client: client,
	}
}

type MegaSDKRestClient struct {
	apiURL string
	mut    sync.Mutex
	client *http.Client
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
	loginReq := &MegaSDKRestReq{
		Email:    email,
		Password: password,
	}
	data, err := loginReq.Marshal()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, m.apiURL+"/login", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := m.client.Do(req)
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
	addDownloadReq := &MegaSDKRestReq{
		Link: link,
		Dir:  dir,
	}
	data, err := addDownloadReq.Marshal()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, m.apiURL+"/adddownload", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := m.client.Do(req)
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
	cancelDownloadReq := &MegaSDKRestReq{
		Gid: gid,
	}
	data, err := cancelDownloadReq.Marshal()
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, m.apiURL+"/canceldownload", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := m.client.Do(req)
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
	getStatusReq := &MegaSDKRestReq{
		Gid: gid,
	}
	data, err := getStatusReq.Marshal()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, m.apiURL+"/getstatus", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Close = true
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Keep-Alive", "timeout=20")
	res, err := m.client.Do(req)
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

var megaClient *MegaSDKRestClient = NewMegaSDKRestClient("http://localhost:5000", http.DefaultClient)

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
	dlinfo            *MegaSDKRestDownloadInfo
}

func (m *MegaDownloadListener) GetDownloadInfo() *MegaSDKRestDownloadInfo {
	if m.dlinfo == nil {
		return &MegaSDKRestDownloadInfo{
			Name:  "unknown",
			State: MegaSDKRestStateQueued,
		}
	}
	return m.dlinfo
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
		if err != nil && status == nil {
			m.dlinfo = &MegaSDKRestDownloadInfo{Name: "unknown"}
			m.OnDownloadError(err)
			return
		}
		m.dlinfo = status

		if status.IsCancelled {
			m.OnDownloadError(fmt.Errorf("cancelled by user"))
			return
		} else if status.IsFailed {
			m.OnDownloadError(fmt.Errorf("%s", status.ErrorString))
			return
		} else if status.IsCompleted {
			m.OnDownloadComplete()
			return
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
	adddl, err := megaClient.AddDownload(link, utils.GetDownloadDir())
	if err != nil {
		return err
	}
	if adddl.Gid == "" {
		return fmt.Errorf("MegaSDKRestpp: internal error occured")
	}
	megaDownloadListener := NewMegaDownloadListener(adddl.Gid, listener)
	megaDownloadListener.StartListener()
	status := NewMegaDownloadStatus(adddl.Gid, listener, megaDownloadListener)
	status.Index_ = GenerateMirrorIndex()
	AddMirrorLocal(listener.GetUid(), status)
	status.GetListener().OnDownloadStart(status.Gid())
	return nil
}

type MegaDownloadStatus struct {
	gid                  string
	listener             *MirrorListener
	megaDownloadListener *MegaDownloadListener
	Index_               int
	lastStatsRefresh     time.Time
	dlinfo               *MegaSDKRestDownloadInfo
}

func (m *MegaDownloadStatus) GetStats() *MegaSDKRestDownloadInfo {
	return m.megaDownloadListener.GetDownloadInfo()
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

func NewMegaDownloadStatus(gid string, listener *MirrorListener, megaDownloadListener *MegaDownloadListener) *MegaDownloadStatus {
	return &MegaDownloadStatus{
		gid:                  gid,
		listener:             listener,
		megaDownloadListener: megaDownloadListener,
	}
}
