package engine

import (
	"MirrorBotGo/utils"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"time"

	"google.golang.org/api/drive/v3"
)

type FileMetadataResponse struct {
	File  drive.File `json:"file"`
	Error string     `json:"error"`
}

func (u *FileMetadataResponse) Unmarshal(data []byte) error {
	return json.Unmarshal(data, u)
}

type ListFilesResponse struct {
	Files []drive.File `json:"files"`
	Error string       `json:"error"`
}

func (u *ListFilesResponse) Unmarshal(data []byte) error {
	return json.Unmarshal(data, u)
}

type TransferStatusResponse struct {
	Gid             string `json:"gid"`
	TotalLength     int64  `json:"total_length"`
	CompletedLength int64  `json:"completed_length"`
	IsCompleted     bool   `json:"is_completed"`
	IsFailed        bool   `json:"is_failed"`
	Speed           int64  `json:"speed"`
	TransferType    string `json:"transfer_type"`
	Name            string `json:"name"`
	FileID          string `json:"file_id"`
	Error           string `json:"error"`
}

func (u *TransferStatusResponse) Unmarshal(data []byte) error {
	return json.Unmarshal(data, u)
}

type GidResponse struct {
	Gid   string `json:"gid"`
	Error string `json:"error"`
}

func (u *GidResponse) Unmarshal(data []byte) error {
	return json.Unmarshal(data, u)
}

type UploadRequest struct {
	Path        string `json:"path"`
	ParentId    string `json:"parent_id"`
	Concurrency int    `json:"concurrency"`
	Size        int64  `json:"size"`
}

func (u *UploadRequest) Marshal() ([]byte, error) {
	return json.MarshalIndent(u, "", " ")
}

type DownloadRequest struct {
	FileId      string `json:"file_id"`
	LocalDir    string `json:"local_dir"`
	Size        int64  `json:"size"`
	Concurrency int    `json:"concurrency"`
}

func (u *DownloadRequest) Marshal() ([]byte, error) {
	return json.MarshalIndent(u, "", " ")
}

type ListFilesRequest struct {
	Name     string `json:"name"`
	ParentID string `json:"parent_id"`
	Count    int    `json:"count"`
}

func (u *ListFilesRequest) Marshal() ([]byte, error) {
	return json.MarshalIndent(u, "", " ")
}

type CloneRequest struct {
	FileId      string `json:"file_id"`
	DesId       string `json:"des_id"`
	Concurrency int    `json:"concurrency"`
	Size        int64  `json:"size"`
}

func (u *CloneRequest) Marshal() ([]byte, error) {
	return json.MarshalIndent(u, "", " ")
}

type CancelRequest struct {
	Gid string `json:"gid"`
}

func (u *CancelRequest) Marshal() ([]byte, error) {
	return json.MarshalIndent(u, "", " ")
}

type TransferServiceClient struct {
	ApiUrl string
	client *http.Client
}

func (t *TransferServiceClient) CancelTransfer(u *CancelRequest) (string, error) {
	data, err := u.Marshal()
	if err != nil {
		return "", fmt.Errorf("jsonMarshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, t.ApiUrl+"/cancel", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("httpNewRequest: %v", err)
	}
	req.Header.Add("Content-Type", "application/json")
	res, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("clientDoRequest: %v", err)
	}
	defer res.Body.Close()
	gidResponse := &GidResponse{}
	resData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("ioutilReadAll: %v", err)
	}
	err = gidResponse.Unmarshal(resData)
	if err != nil {
		return "", fmt.Errorf("responseUnmarshal: %v", err)
	}
	if gidResponse.Error != "" {
		return "", fmt.Errorf("serverResponse: %v", gidResponse.Error)
	}
	return gidResponse.Gid, nil
}

func (t *TransferServiceClient) AddUpload(u *UploadRequest) (string, error) {
	data, err := u.Marshal()
	if err != nil {
		return "", fmt.Errorf("jsonMarshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, t.ApiUrl+"/upload", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("httpNewRequest: %v", err)
	}
	req.Header.Add("Content-Type", "application/json")
	res, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("clientDoRequest: %v", err)
	}
	defer res.Body.Close()
	gidResponse := &GidResponse{}
	resData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("ioutilReadAll: %v", err)
	}
	err = gidResponse.Unmarshal(resData)
	if err != nil {
		return "", fmt.Errorf("responseUnmarshal: %v", err)
	}
	if gidResponse.Error != "" {
		return "", fmt.Errorf("serverResponse: %v", gidResponse.Error)
	}
	return gidResponse.Gid, nil
}

func (t *TransferServiceClient) AddClone(u *CloneRequest) (string, error) {
	data, err := u.Marshal()
	if err != nil {
		return "", fmt.Errorf("jsonMarshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, t.ApiUrl+"/clone", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("httpNewRequest: %v", err)
	}
	req.Header.Add("Content-Type", "application/json")
	res, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("clientDoRequest: %v", err)
	}
	defer res.Body.Close()
	gidResponse := &GidResponse{}
	resData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("ioutilReadAll: %v", err)
	}
	err = gidResponse.Unmarshal(resData)
	if err != nil {
		return "", fmt.Errorf("responseUnmarshal: %v", err)
	}
	if gidResponse.Error != "" {
		return "", fmt.Errorf("serverResponse: %v", gidResponse.Error)
	}
	return gidResponse.Gid, nil
}

func (t *TransferServiceClient) ListFiles(u *ListFilesRequest) (*ListFilesResponse, error) {
	data, err := u.Marshal()
	if err != nil {
		return nil, fmt.Errorf("jsonMarshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, t.ApiUrl+"/listfiles", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("httpNewRequest: %v", err)
	}
	req.Header.Add("Content-Type", "application/json")
	res, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("clientDoRequest: %v", err)
	}
	defer res.Body.Close()
	listFilesResponse := &ListFilesResponse{}
	resData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("ioutilReadAll: %v", err)
	}
	err = listFilesResponse.Unmarshal(resData)
	if err != nil {
		return nil, fmt.Errorf("responseUnmarshal: %v", err)
	}
	if listFilesResponse.Error != "" {
		return listFilesResponse, fmt.Errorf("serverResponse: %v", listFilesResponse.Error)
	}
	return listFilesResponse, nil
}

func (t *TransferServiceClient) AddDownload(u *DownloadRequest) (string, error) {
	data, err := u.Marshal()
	if err != nil {
		return "", fmt.Errorf("jsonMarshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, t.ApiUrl+"/download", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("httpNewRequest: %v", err)
	}
	req.Header.Add("Content-Type", "application/json")
	res, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("clientDoRequest: %v", err)
	}
	defer res.Body.Close()
	gidResponse := &GidResponse{}
	resData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("ioutilReadAll: %v", err)
	}
	err = gidResponse.Unmarshal(resData)
	if err != nil {
		return "", fmt.Errorf("responseUnmarshal: %v", err)
	}
	if gidResponse.Error != "" {
		return "", fmt.Errorf("serverResponse: %v", gidResponse.Error)
	}
	return gidResponse.Gid, nil
}

func (t *TransferServiceClient) GetFileMetadata(fileId string) (*FileMetadataResponse, error) {
	req, err := http.NewRequest(http.MethodGet, t.ApiUrl+fmt.Sprintf("/filemetadata/%s", fileId), nil)
	if err != nil {
		return nil, err
	}
	res, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	fileMetadataResponse := &FileMetadataResponse{}
	resData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	err = fileMetadataResponse.Unmarshal(resData)
	if err != nil {
		return nil, err
	}
	if fileMetadataResponse.Error != "" {
		return fileMetadataResponse, errors.New(fileMetadataResponse.Error)
	}
	return fileMetadataResponse, nil
}

func (t *TransferServiceClient) GetStatusByGid(gid string) (*TransferStatusResponse, error) {
	req, err := http.NewRequest(http.MethodGet, t.ApiUrl+fmt.Sprintf("/transferstatus/%s", gid), nil)
	if err != nil {
		return nil, err
	}
	res, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	transferStatusResponse := &TransferStatusResponse{}
	resData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	err = transferStatusResponse.Unmarshal(resData)
	if err != nil {
		return nil, err
	}
	if transferStatusResponse.Error != "" {
		return transferStatusResponse, errors.New(transferStatusResponse.Error)
	}
	return transferStatusResponse, nil
}

func NewTransferServiceClient(apiUrl string, client *http.Client) *TransferServiceClient {
	return &TransferServiceClient{
		ApiUrl: apiUrl,
		client: client,
	}
}

var transferServiceClient *TransferServiceClient = NewTransferServiceClient("http://127.0.0.1:6969/api/v1", &http.Client{})

type GoogleDriveTransferStatus struct {
	gid           string
	path          string
	listener      *MirrorListener
	cloneListener *CloneListener
	Index_        int
}

func (g *GoogleDriveTransferStatus) getStatus() *TransferStatusResponse {
	ts, err := transferServiceClient.GetStatusByGid(g.gid)
	if err != nil {
		L().Errorf("[TransferServiceStatus]: %v", err)
		if ts == nil {
			ts = &TransferStatusResponse{}
		}
	}
	return ts
}

func (g *GoogleDriveTransferStatus) Name() string {
	return g.getStatus().Name
}

func (g *GoogleDriveTransferStatus) CompletedLength() int64 {
	return g.getStatus().CompletedLength
}

func (g *GoogleDriveTransferStatus) TotalLength() int64 {
	return g.getStatus().TotalLength
}

func (g *GoogleDriveTransferStatus) Speed() int64 {
	return g.getStatus().Speed
}

func (g *GoogleDriveTransferStatus) Gid() string {
	return g.gid
}

func (g *GoogleDriveTransferStatus) ETA() *time.Duration {
	eta := time.Duration(0)
	return &eta
}

func (g *GoogleDriveTransferStatus) GetStatusType() string {
	transferType := g.getStatus().TransferType
	switch transferType {
	case "upload":
		return MirrorStatusUploading
	case "download":
		return MirrorStatusDownloading
	case "clone":
		return MirrorStatusCloning
	default:
		return MirrorStatusWaiting
	}
}

func (g *GoogleDriveTransferStatus) Path() string {
	transferType := g.getStatus().TransferType
	if transferType == "download" {
		return path.Join(g.path, g.Name())
	}
	return g.path
}

func (g *GoogleDriveTransferStatus) Percentage() float32 {
	return float32(g.CompletedLength()*100) / float32(g.TotalLength())
}

func (g *GoogleDriveTransferStatus) IsTorrent() bool {
	return false
}

func (g *GoogleDriveTransferStatus) GetPeers() int {
	return 0
}

func (g *GoogleDriveTransferStatus) GetSeeders() int {
	return 0
}

func (g *GoogleDriveTransferStatus) GetListener() *MirrorListener {
	return g.listener
}

func (g *GoogleDriveTransferStatus) GetCloneListener() *CloneListener {
	return g.cloneListener
}

func (g *GoogleDriveTransferStatus) Index() int {
	return g.Index_
}

func (g *GoogleDriveTransferStatus) CancelMirror() bool {
	_, err := transferServiceClient.CancelTransfer(&CancelRequest{
		Gid: g.Gid(),
	})
	if err != nil {
		L().Errorf("GoogleDriveTransferStatus: CancelMirror: %v", err)
		SendMessage(g.GetListener().bot, err.Error(), g.GetListener().Update.Message)
		return false
	}
	return true
}

func NewGoogleDriveTransferStatus(gid string, path string, listener *MirrorListener, cloneListener *CloneListener) *GoogleDriveTransferStatus {
	return &GoogleDriveTransferStatus{
		gid:           gid,
		path:          path,
		listener:      listener,
		cloneListener: cloneListener,
	}
}

func NewGoogleDriveTransferListener(listener *MirrorListener, cloneListener *CloneListener, isClone bool, gid string) *GoogleDriveTransferListener {
	return &GoogleDriveTransferListener{
		listener:      listener,
		cloneListener: cloneListener,
		isClone:       isClone,
		gid:           gid,
	}
}

type GoogleDriveTransferListener struct {
	listener          *MirrorListener
	cloneListener     *CloneListener
	isClone           bool
	gid               string
	isListenerRunning bool
	handled           bool
}

func (g *GoogleDriveTransferListener) haveListener() bool {
	if g.listener == nil {
		L().Error("[GoogleDriveTransferListener]: listener=nil")
		return false
	}
	return true
}

func (g *GoogleDriveTransferListener) haveCloneListener() bool {
	if g.cloneListener == nil {
		L().Error("[GoogleDriveTransferListener]: cloneListener=nil")
		return false
	}
	return true
}

func (g *GoogleDriveTransferListener) OnDownloadComplete() {
	if !g.haveListener() || g.handled {
		return
	}
	g.StopListener()
	g.handled = true
	g.listener.OnDownloadComplete()
}

func (g *GoogleDriveTransferListener) OnDownloadError(err string) {
	if !g.haveListener() || g.handled {
		return
	}
	g.StopListener()
	g.handled = true
	g.listener.OnDownloadError(err)
}

func (g *GoogleDriveTransferListener) OnUploadComplete(fileId string) {
	if !g.haveListener() || g.handled {
		return
	}
	g.StopListener()
	g.handled = true
	g.listener.OnUploadComplete(fmt.Sprintf("https://drive.google.con/open?id=%s", fileId))
}

func (g *GoogleDriveTransferListener) OnUploadError(err string) {
	if !g.haveListener() || g.handled {
		return
	}
	g.StopListener()
	g.handled = true
	g.listener.OnUploadError(err)
}

func (g *GoogleDriveTransferListener) OnCloneError(err string) {
	if !g.haveCloneListener() || g.handled {
		return
	}
	g.StopListener()
	g.handled = true
	g.cloneListener.OnCloneError(err)
}

func (g *GoogleDriveTransferListener) OnCloneComplete(fileId string) {
	if !g.haveCloneListener() || g.handled {
		return
	}
	g.StopListener()
	g.handled = true
	g.cloneListener.OnCloneComplete(fmt.Sprintf("https://drive.google.con/open?id=%s", fileId))
}

func (g *GoogleDriveTransferListener) ListenForEvents() {
	for g.isListenerRunning {
		status, err := transferServiceClient.GetStatusByGid(g.gid)
		if err != nil {
			L().Errorf("GoogleDriveTransferListener: %v", err)
			if status == nil {
				status = &TransferStatusResponse{}
			}
		}
		switch status.TransferType {
		case "download":
			if status.IsCompleted {
				g.OnDownloadComplete()
			}
			if status.IsFailed {
				g.OnDownloadError(status.Error)
			}
		case "upload":
			if status.IsCompleted {
				g.OnUploadComplete(status.FileID)
			}
			if status.IsFailed {
				g.OnUploadError(status.Error)
			}
		case "clone":
			if status.IsCompleted {
				g.OnCloneComplete(status.FileID)
			}
			if status.IsFailed {
				g.OnCloneError(status.Error)
			}
		}
		time.Sleep(1 * time.Second)
	}
}

func (g *GoogleDriveTransferListener) StartListener() {
	g.isListenerRunning = true
	go g.ListenForEvents()
}

func (g *GoogleDriveTransferListener) StopListener() {
	g.isListenerRunning = false
}

func NewGDriveCloneTransferService(fileId string, parentId string, listener *CloneListener) {
	trGid, err := transferServiceClient.AddClone(&CloneRequest{
		FileId:      fileId,
		DesId:       parentId,
		Concurrency: 10,
	})
	if err != nil {
		listener.OnCloneError(err.Error())
		return
	}
	trListener := NewGoogleDriveTransferListener(nil, listener, true, trGid)
	trListener.StartListener()
	status := NewGoogleDriveTransferStatus(trGid, "", nil, listener)
	status.Index_ = GenerateMirrorIndex()
	AddMirrorLocal(listener.GetUid(), status)
}

func NewGDriveDownloadTransferService(fileId string, listener *MirrorListener) {
	dir := path.Join(utils.GetDownloadDir(), utils.ParseInt64ToString(listener.GetUid()))
	os.MkdirAll(dir, 0755)
	trGid, err := transferServiceClient.AddDownload(&DownloadRequest{
		FileId:      fileId,
		LocalDir:    dir,
		Concurrency: 10,
	})
	if err != nil {
		listener.OnDownloadError(err.Error())
		return
	}
	trListener := NewGoogleDriveTransferListener(listener, nil, true, trGid)
	trListener.StartListener()
	status := NewGoogleDriveTransferStatus(trGid, dir, listener, nil)
	status.Index_ = GenerateMirrorIndex()
	AddMirrorLocal(listener.GetUid(), status)
}

func FormatGDriveLink(fileId string) string {
	return fmt.Sprintf("https://drive.google.com/open?id=%s", fileId)
}

func IsGDriveFolder(mimeType string) bool {
	return mimeType == "application/vnd.google-apps.folder"
}
