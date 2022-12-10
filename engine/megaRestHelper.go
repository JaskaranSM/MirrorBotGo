package engine

import (
	"MirrorBotGo/utils"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
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
)

var megaLoggedIn bool = false

type MegaSDKRestLogin struct {
	Login   string `json:"login"`
	Message string `json:"message"`
}

func (m *MegaSDKRestLogin) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

type MegaSDKRestAddDl struct {
	Adddl   string `json:"adddl"`
	Message string `json:"message"`
	Gid     string `json:"gid"`
	Dir     string `json:"dir"`
}

func (m *MegaSDKRestAddDl) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

type MegaSDKRestDownloadInfo struct {
	Dlinfo          string `json:"dlinfo"`
	Message         string `json:"message"`
	Name            string `json:"name"`
	ErrorCode       int    `json:"error_code"`
	ErrorString     string `json:"error_string"`
	Gid             string `json:"gid"`
	Speed           int64  `json:"speed"`
	CompletedLength int64  `json:"completed_length"`
	TotalLength     int64  `json:"total_length"`
	State           int    `json:"state"`
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

func (m *MegaSDKRestClient) checkAndRaiseError(resData []byte) error {
	if strings.Contains(string(resData), "failed") {
		L().Errorf("MegaSDKRest: %s", string(resData))
		return errors.New("internal error occured")
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

func (m *MegaSDKRestClient) Login(email string, password string) (*MegaSDKRestLogin, error) {
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
	var login *MegaSDKRestLogin = &MegaSDKRestLogin{}
	err = login.Unmarshal(resData)
	if err != nil {
		return nil, err
	}
	if login.Login == "failed" {
		return login, errors.New(login.Message)
	}
	return login, nil
}

func (m *MegaSDKRestClient) AddDl(link string, dir string) (*MegaSDKRestAddDl, error) {
	m.mut.Lock()
	defer m.mut.Unlock()
	m.checkLogin()
	res, err := http.PostForm(m.apiURL+"/adddl", url.Values{
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
	var adddl *MegaSDKRestAddDl = &MegaSDKRestAddDl{}
	err = adddl.Unmarshal(resData)
	if err != nil {
		return nil, err
	}
	if adddl.Adddl == "failed" {
		return adddl, errors.New(adddl.Message)
	}
	if adddl.Gid == "" {
		return nil, errors.New("internal error occurred")
	}
	return adddl, nil
}

func (m *MegaSDKRestClient) CancelDl(gid string) error {
	m.mut.Lock()
	defer m.mut.Unlock()
	res, err := http.PostForm(m.apiURL+"/canceldl", url.Values{
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
	return m.checkAndRaiseError(resData)
}

func (m *MegaSDKRestClient) GetDownloadInfo(gid string) (*MegaSDKRestDownloadInfo, error) {
	m.mut.Lock()
	defer m.mut.Unlock()
	res, err := http.Get(m.apiURL + fmt.Sprintf("/dlinfo/%s", gid))
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
	err = m.checkAndRaiseError(resData)
	if err != nil {
		return nil, err
	}
	var downloadInfo *MegaSDKRestDownloadInfo = &MegaSDKRestDownloadInfo{}
	err = downloadInfo.Unmarshal(resData)
	if err != nil {
		return nil, err
	}
	return downloadInfo, nil
}

var megaClient *MegaSDKRestClient = NewMegaSDKRestClient("http://localhost:6090")

func PerformMegaLogin() error {
	_, err := megaClient.Login(utils.GetMegaEmail(), utils.GetMegaPassword())
	if err != nil {
		L().Errorf("MegaSDKRest: %s", err.Error())
	}
	return err
}

func init() {
	StartMegaSDKRestServer(utils.GetMegaAPIKey())
}

func NewMegaDownload(link string, listener *MirrorListener) error {
	dir := path.Join(utils.GetDownloadDir(), utils.ParseInt64ToString(listener.GetUid()))
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		L().Errorf("NewMegaDownload: os.MkdirAll: %s : %v", dir, err)
		return err
	}
	adddl, err := megaClient.AddDl(link, dir)
	if err != nil {
		return err
	}
	go func() {
		state := MegaSDKRestStateActive
		do := true
		for do {
			switch state {
			case MegaSDKRestStateFailed:
				do = false
			case MegaSDKRestStateCancelled:
				do = false
			case MegaSDKRestStateCompleted:
				do = false
			}
			dlinfo, err := megaClient.GetDownloadInfo(adddl.Gid)
			if err != nil {
				state = MegaSDKRestStateFailed
				do = false
				continue
			}
			state = dlinfo.State
			time.Sleep(500 * time.Millisecond)
		}
		if state == MegaSDKRestStateFailed || state == MegaSDKRestStateCancelled {
			dlinfo, err := megaClient.GetDownloadInfo(adddl.Gid)
			if err != nil {
				state = MegaSDKRestStateFailed
				do = false
			}
			if dlinfo == nil {
				L().Errorf("critical mega error: %v", err)
				do = false
				return
			}
			dl := GetMirrorByGid(dlinfo.Gid)
			if dl != nil {
				if state == MegaSDKRestStateCancelled {
					dl.GetListener().OnDownloadError("Canceled by user")
				} else {
					dl.GetListener().OnDownloadError(dlinfo.ErrorString)
				}
				return
			}
		}
		listener.OnDownloadComplete()
	}()
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
		L().Errorf("MegaDownloadSTatus: %s", err.Error())
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
	err := megaClient.CancelDl(m.Gid())
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

func StartMegaSDKRestServer(apiKey string) {
	cmd := exec.Command("megasdkrest", "--apikey", apiKey)
	err := cmd.Start()
	if err != nil {
		L().Errorf("MegaSDKRest: %s", err.Error())
	} else {
		L().Info("MegaSDKRest: server started")
	}
}
