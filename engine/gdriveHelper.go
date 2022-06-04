package engine

import (
	"MirrorBotGo/utils"
	"encoding/json"
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

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
)

var GLOBAL_SA_INDEX int = 0
var concurrent_uploads int = 10

var upload_limit_chan chan int = make(chan int, concurrent_uploads)

const SA_DIR string = "accounts"

func GetSaCount() int {
	sas, err := ioutil.ReadDir(SA_DIR)
	if err != nil {
		L().Error(err)
	}
	return len(sas)
}

type GoogleDriveClient struct {
	RootId              string
	GDRIVE_DIR_MIMETYPE string
	TokenFile           string
	CredentialFile      string
	TotalLength         int64
	CompletedLength     int64
	LastTransferred     int64
	Speed               int64
	name                string
	MaxRetries          int
	isCancelled         bool
	isUploading         bool
	err                 error
	doNothing           bool
	path                string
	wg                  sync.WaitGroup
	prg                 *ProgressContext
	SleepTime           time.Duration
	StartTime           time.Time
	ETA                 time.Duration
	DriveSrv            *drive.Service
	Listener            *MirrorListener
	CloneListener       *CloneListener
	isCloneCancelled    bool
	SaLog               bool
}

func (G *GoogleDriveClient) Init(rootId string) {
	if rootId == "" {
		rootId = "root"
	}
	G.RootId = rootId
	G.GDRIVE_DIR_MIMETYPE = "application/vnd.google-apps.folder"
	G.TokenFile = "token.json"
	G.CredentialFile = "credentials.json"
	G.StartTime = time.Now()
	if utils.UseSa() {
		G.MaxRetries = GetSaCount()
	}
	G.SaLog = true
}

// Retrieve a token, saves the token, then returns the generated client.
func (G *GoogleDriveClient) getClient(config *oauth2.Config) *http.Client {
	tok, err := G.tokenFromFile(G.TokenFile)
	if err != nil {
		tok = G.getTokenFromWeb(config)
		G.saveToken(G.TokenFile, tok)
	}
	return config.Client(context.Background(), tok)
}

func (G *GoogleDriveClient) SwitchServiceAccount() {
	if !utils.UseSa() {
		return
	}
	if GLOBAL_SA_INDEX == GetSaCount()-1 {
		GLOBAL_SA_INDEX = 0
	}
	GLOBAL_SA_INDEX += 1
	if G.SaLog {
		L().Infof("Switching to %d service account", GLOBAL_SA_INDEX)
	}
	G.Authorize()
}

func (G *GoogleDriveClient) Authorize() {
	if utils.UseSa() {
		if G.SaLog {
			L().Infof("Authorizing with %d service account.", GLOBAL_SA_INDEX)
		}
		b, err := ioutil.ReadFile(fmt.Sprintf("%s/%d.json", SA_DIR, GLOBAL_SA_INDEX))
		config, err := google.JWTConfigFromJSON(b, drive.DriveScope)
		if err != nil {
			if G.Listener != nil {
				G.err = err
				G.Listener.OnUploadError("failed to get JWT from JSON: " + err.Error())
				return
			} else {
				L().Error(err)
			}
		}
		client := config.Client(context.Background())
		srv, err := drive.New(client)
		if err != nil {
			L().Error(err)
		}
		G.DriveSrv = srv
	} else {
		b, err := ioutil.ReadFile(G.CredentialFile)
		if err != nil {
			if G.Listener != nil {
				G.err = err
				G.Listener.OnUploadError("Unable to read client secret file: " + err.Error())
				return
			} else {
				L().Error(err)
			}
		}
		// If modifying these scopes, delete your previously saved token.json.
		config, err := google.ConfigFromJSON(b, drive.DriveScope)
		if err != nil {
			if G.Listener != nil {
				G.err = err
				G.Listener.OnUploadError("Unable to parse client secret file to config: " + err.Error())
				return
			} else {
				L().Error(err)
			}
		}
		client := G.getClient(config)

		srv, err := drive.New(client)
		if err != nil {
			if G.Listener != nil {
				G.err = err
				G.Listener.OnUploadError("Unable to retrieve Drive client: " + err.Error())
				return
			} else {
				L().Error(err)
			}
		}
		G.DriveSrv = srv
	}
}

func (G *GoogleDriveClient) getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		L().Fatalf("Unable to read authorization code %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		L().Fatalf("Unable to retrieve token from web %v", err)
	}
	return tok
}

func (G *GoogleDriveClient) tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func (G *GoogleDriveClient) saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		L().Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func (G *GoogleDriveClient) Sleep(d time.Duration) {
	if utils.UseSa() {
		return
	}
	L().Infof("Sleeping for %s", d)
	time.Sleep(d)
}

func (G *GoogleDriveClient) CreateDir(name string, parentId string, retry int) (*drive.File, error) {
	d := &drive.File{
		Name:     name,
		MimeType: G.GDRIVE_DIR_MIMETYPE,
		Parents:  []string{parentId},
	}
	file, err := G.DriveSrv.Files.Create(d).SupportsAllDrives(true).Do()
	if err != nil {
		if G.CheckRetry(file, err) {
			if utils.UseSa() {
				G.SwitchServiceAccount()
			}
			if retry <= G.MaxRetries {
				L().Info("Encountered: ", err.Error(), " retryin: ", retry)
				G.Sleep(G.SleepTime)
				return G.CreateDir(name, parentId, retry+1)
			} else {
				L().Info("Could not create dir (even after retryin): " + err.Error())
				return nil, err
			}
		} else {
			L().Error("Could not create dir: " + err.Error())
			return nil, err
		}
	}
	L().Info("Created G-Drive Folder: ", file.Id)
	if !utils.IsTeamDrive() {
		err = G.SetPermissions(file.Id, 1)
		if err != nil {
			L().Error(err)
		}
	}
	return file, nil
}

func (G *GoogleDriveClient) Upload(path string, parentId string) bool {
	G.path = path
	G.isUploading = true
	defer func() {
		if len(upload_limit_chan) > 0 {
			<-upload_limit_chan
		}
	}()
	var link string
	var file *drive.File
	var f os.FileInfo
	var err error
	f, err = os.Stat(path)
	if err != nil {
		G.err = err
		G.Listener.OnUploadError(err.Error())
		return false
	}
	if f.IsDir() {
		file, err = G.CreateDir(f.Name(), parentId, 1)
		if err != nil {
			G.err = err
			G.Listener.OnUploadError(err.Error())
			return false
		}
		G.UploadDirRec(path, file.Id)
		link = G.FormatLink(file.Id)
	} else {
		file, err = G.UploadFile(parentId, path, 1)
		if err != nil {
			G.err = err
			G.Listener.OnUploadError(err.Error())
			return false
		}
		link = G.FormatLink(file.Id)
	}
	if G.err == nil {
		G.Listener.OnUploadComplete(link)
	}
	return true
}

func (G *GoogleDriveClient) IsDir(file *drive.File) bool {
	return file.MimeType == G.GDRIVE_DIR_MIMETYPE
}

func (G *GoogleDriveClient) Download(fileId string, dir string) {
	G.name = "getting metadata"
	meta, err := G.GetFileMetadata(fileId, 1)
	if err != nil {
		G.Listener.OnDownloadError(err.Error())
		return
	}
	if G.IsDir(meta) {
		G.GetFolderSize(meta.Id, &G.TotalLength)
	} else {
		G.TotalLength = meta.Size
	}
	G.name = meta.Name
	L().Info(G.name)
	local := path.Join(dir, meta.Name)
	if G.IsDir(meta) {
		os.Mkdir(local, 0755)
		G.DownloadFolder(meta.Id, local)
	} else {
		err := G.DownloadFile(meta.Id, local, meta.Size, 1)
		G.Clean()
		if err != nil {
			G.Listener.OnDownloadError(err.Error())
			return
		}
	}
	if G.doNothing {
		return
	}
	if !G.isCancelled {
		G.Listener.OnDownloadComplete()
	} else {
		G.Listener.OnDownloadError("Canceled by user.")
	}
}

func (G *GoogleDriveClient) GetFolderSize(folderId string, size *int64) {
	files := G.ListFilesByParentId(folderId, "", -1)
	for _, file := range files {
		if G.isCancelled {
			return
		}
		if file.MimeType == G.GDRIVE_DIR_MIMETYPE {
			G.GetFolderSize(file.Id, size)
		} else {
			*size += file.Size
		}
	}
}

func (G *GoogleDriveClient) DownloadFolder(folderId string, local string) bool {
	files := G.ListFilesByParentId(folderId, "", -1)
	for _, file := range files {
		if G.isCancelled {
			return false
		}
		current_path := path.Join(local, file.Name)
		if file.MimeType == G.GDRIVE_DIR_MIMETYPE {
			os.Mkdir(current_path, 0755)
			G.DownloadFolder(file.Id, current_path)
		} else {
			err := G.DownloadFile(file.Id, current_path, file.Size, 1)
			G.Clean()
			if err != nil {
				G.doNothing = true
				G.Listener.OnDownloadError(err.Error())
				return false
			}
		}
	}
	return true
}

func (G *GoogleDriveClient) CancelDownload() {
	G.isCancelled = true
	if G.prg != nil {
		G.prg.Cancel()
	}
}

func (G *GoogleDriveClient) DownloadFile(fileId string, local string, size int64, retry int) error {
	writer, err := os.OpenFile(local, os.O_WRONLY|os.O_CREATE, 0644)
	defer writer.Close()
	if err != nil {
		return err
	}
	request := G.DriveSrv.Files.Get(fileId).SupportsAllDrives(true)
	response, err := request.Download()
	if err != nil {
		if G.CheckRetry(nil, err) || (response != nil && response.StatusCode >= 500) {
			if utils.UseSa() {
				G.SwitchServiceAccount()
			}
			if retry <= G.MaxRetries {
				L().Warn("Encountered: ", err.Error(), " retryin: ", retry)
				G.Sleep(G.SleepTime)
				G.Clean()
				return G.DownloadFile(fileId, local, size, retry+1)
			} else {
				L().Error("Could not download drive file (even after retryin): " + err.Error())
				return err
			}
		} else {
			L().Error("Could not download drive file: " + err.Error())
			return err
		}
	}
	G.prg = &ProgressContext{drive: G, total: size, completed: 0}
	_, err = io.Copy(writer, io.TeeReader(response.Body, G.prg))
	return err
}

func (G *GoogleDriveClient) UploadDirRec(directoryPath string, parentId string) bool {
	if G.err != nil {
		return false
	}
	files, err := ioutil.ReadDir(directoryPath)
	if err != nil {
		G.err = err
		G.Listener.OnUploadError(err.Error())
		return false
	}
	for _, f := range files {
		if G.err != nil {
			return false
		}
		currentFile := path.Join(directoryPath, f.Name())
		if f.IsDir() {
			file, err := G.CreateDir(f.Name(), parentId, 1)
			if err != nil {
				G.err = err
				G.Listener.OnUploadError(err.Error())
				return false
			}
			G.UploadDirRec(currentFile, file.Id)
		} else {
			file, err := G.UploadFile(parentId, currentFile, 1)
			if err != nil {
				G.err = err
				G.Listener.OnUploadError(err.Error())
				return false
			} else {
				L().Info("Uploaded File: ", file.Id)
			}
		}
	}
	return true
}

func (G *GoogleDriveClient) OnTransferUpdate(current, total int64) {
	chunkSize := current - G.LastTransferred
	G.CompletedLength += chunkSize
	G.LastTransferred = current
	now := time.Now()
	diff := int64(now.Sub(G.StartTime).Seconds())
	if diff != 0 {
		G.Speed = G.CompletedLength / diff
	} else {
		G.Speed = 0
	}
	if G.Speed != 0 {
		G.ETA = utils.CalculateETA(G.TotalLength-G.CompletedLength, G.Speed)
	} else {
		G.ETA = time.Duration(0)
	}
}

func (G *GoogleDriveClient) Clean() {
	G.LastTransferred = 0
}

func (G *GoogleDriveClient) SetPermissions(fileId string, retry int) error {
	permission := &drive.Permission{
		AllowFileDiscovery: false,
		Role:               "reader",
		Type:               "anyone",
	}
	file, err := G.DriveSrv.Permissions.Create(fileId, permission).Fields("").SupportsAllDrives(true).SupportsTeamDrives(true).Do()
	if err != nil {
		var tmp *drive.File
		if file != nil {
			tmp = &drive.File{ServerResponse: file.ServerResponse} //just a hack
		}
		if G.CheckRetry(tmp, err) {
			if utils.UseSa() {
				G.SwitchServiceAccount()
			}
			if retry <= G.MaxRetries {
				L().Warn("Encountered: ", err.Error(), " retryin: ", retry)
				G.Sleep(G.SleepTime)
				return G.SetPermissions(fileId, retry+1)
			} else {
				L().Error("Could not set file permissions (even after retryin): " + err.Error())
				return err
			}
		} else {
			L().Error("Could not set file permissions: " + err.Error())
			return err
		}
	}
	return err
}

func (G *GoogleDriveClient) FormatLink(fileId string) string {
	return fmt.Sprintf("https://drive.google.com/open?id=%s", fileId)
}

//count = -1 for disabling limit
func (G *GoogleDriveClient) ListFilesByParentId(parentId string, name string, count int) []*drive.File {
	var files []*drive.File
	pageToken := ""
	query := fmt.Sprintf("'%s' in parents", parentId)
	if name != "" {
		query += fmt.Sprintf(" and name contains '%s'", name)
	}
	for {
		request := G.DriveSrv.Files.List().Q(query).OrderBy("modifiedTime desc").SupportsAllDrives(true).IncludeTeamDriveItems(true).PageSize(1000).
			Fields("nextPageToken,files(id, name,size, mimeType)")
		if pageToken != "" {
			request = request.PageToken(pageToken)
		}
		res, err := request.Do()
		if err != nil {
			L().Errorf("Error : %v", err)
			return files
		}
		for _, f := range res.Files {
			if count != -1 && len(files) == count {
				return files
			}
			files = append(files, f)
		}
		pageToken = res.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return files
}

func (G *GoogleDriveClient) GetFileMetadata(fileId string, retry int) (*drive.File, error) {
	file, err := G.DriveSrv.Files.Get(fileId).Fields("name,mimeType,size,id,md5Checksum").SupportsAllDrives(true).Do()
	if err != nil {
		if G.CheckRetry(file, err) {
			if utils.UseSa() {
				G.SwitchServiceAccount()
			}
			if retry <= G.MaxRetries {
				L().Warn("Encountered: ", err.Error(), " retryin: ", retry)
				G.Sleep(G.SleepTime)
				return G.GetFileMetadata(fileId, retry+1)
			} else {
				L().Error("[GetFileMetadata] Could not get file metadata (even after retryin): " + err.Error())
				return nil, err
			}
		} else {
			L().Error("[GetFileMetadata] Could not get file metdata: " + err.Error())
			return nil, err
		}
	}
	return file, err
}

func (G *GoogleDriveClient) CheckRetry(file *drive.File, err error) bool {
	if strings.Contains(strings.ToLower(err.Error()), "rate") || strings.Contains(strings.ToLower(err.Error()), "500") {
		return true
	}
	if file != nil {
		if file.ServerResponse.HTTPStatusCode >= 500 {
			return true
		}
	}
	return false
}

func (G *GoogleDriveClient) UploadFileNonResumable(parentId string, file_path string, retry int) (*drive.File, error) {
	L().Infof("Uploading File with 0 bytes: %s", file_path)
	content, err := os.Open(file_path)
	if err != nil {
		L().Error("Error while opening file for upload: ", err.Error())
		return nil, err
	}
	contentType := "application/octet-stream"
	arr := strings.Split(file_path, "/")
	name := arr[len(arr)-1]
	f := &drive.File{
		MimeType: contentType,
		Name:     name,
		Parents:  []string{parentId},
	}
	L().Infof("Uploading %s with mimeType: %s", f.Name, f.MimeType)
	file, err := G.DriveSrv.Files.Create(f).Media(content).SupportsAllDrives(true).Do()
	if err != nil {
		if G.CheckRetry(file, err) {
			if utils.UseSa() {
				G.SwitchServiceAccount()
			}
			if retry <= G.MaxRetries {
				L().Warn("Encountered: ", err.Error(), " retryin: ", retry)
				G.Sleep(G.SleepTime)
				return G.UploadFileNonResumable(parentId, file_path, retry+1)
			} else {
				L().Error("[NonResumable] Could not create file (even after retryin): " + err.Error())
				return nil, err
			}
		} else {
			L().Error("[NonResumable] Could not create file: " + err.Error())
			return nil, err
		}
	}
	if !utils.IsTeamDrive() {
		err = G.SetPermissions(file.Id, 1)
		if err != nil {
			L().Error(err)
		}
	}
	return file, nil
}

func (G *GoogleDriveClient) UploadFile(parentId string, file_path string, retry int) (*drive.File, error) {
	defer G.Clean()
	content, err := os.Open(file_path)
	if err != nil {
		L().Error("Error while opening file for upload: ", err.Error())
		return nil, err
	}
	stat, err := content.Stat()
	if err != nil {
		L().Error("Error while doing content.Stat()", err.Error())
		return nil, err
	}
	if stat.Size() == 0 {
		content.Close()
		return G.UploadFileNonResumable(parentId, file_path, retry)
	}
	contentType, err := utils.GetFileContentType(content)
	if err != nil {
		L().Error("Error while sniffing content type: ", err.Error())
		return nil, err
	}
	arr := strings.Split(file_path, "/")
	name := arr[len(arr)-1]
	f := &drive.File{
		MimeType: contentType,
		Name:     name,
		Parents:  []string{parentId},
	}
	ctx := context.Background()
	L().Infof("Uploading %s with mimeType: %s", f.Name, f.MimeType)
	file, err := G.DriveSrv.Files.Create(f).ResumableMedia(ctx, content, stat.Size(), contentType).ProgressUpdater(G.OnTransferUpdate).SupportsAllDrives(true).Do()
	if err != nil {
		if G.CheckRetry(file, err) {
			if utils.UseSa() {
				G.SwitchServiceAccount()
			}
			if retry <= G.MaxRetries {
				L().Warn("Encountered: ", err.Error(), " retryin: ", retry)
				G.Sleep(G.SleepTime)
				return G.UploadFile(parentId, file_path, retry+1)
			} else {
				L().Error("Could not create file (even after retryin): " + err.Error())
				return nil, err
			}
		} else {
			L().Error("Could not create file: " + err.Error())
			return nil, err
		}
	}
	if !utils.IsTeamDrive() {
		err = G.SetPermissions(file.Id, 1)
		if err != nil {
			L().Error(err)
		}
	}
	return file, nil
}

func (G *GoogleDriveClient) Clone(fileId string, parentId string) {
	_, err := G.GetFileMetadata(parentId, 1)
	if err != nil {
		L().Error("Clone error while checking for user supplied parentId: " + err.Error())
		parentId = utils.GetGDriveParentId()
	}
	var link string
	meta, err := G.GetFileMetadata(fileId, 1)
	if err != nil {
		G.CloneListener.OnCloneError(err.Error())
		return
	}
	G.name = meta.Name
	L().Info("Cloning: " + meta.Name)
	G.CloneListener.OnCloneStart(meta.Name)
	if meta.MimeType == G.GDRIVE_DIR_MIMETYPE {
		new_dir, err := G.CreateDir(meta.Name, parentId, 1)
		if err != nil {
			L().Error("GDriveCreateDir: " + err.Error())
			G.CloneListener.OnCloneError(err.Error())
			return
		} else {
			if utils.UseSa() {
				G.SwitchServiceAccount()
				G.wg.Add(1)
				go G.CopyFolder(meta.Id, new_dir.Id, true)
				G.wg.Wait()
			} else {
				G.CopyFolder(meta.Id, new_dir.Id, false)
			}
		}
		link = G.FormatLink(new_dir.Id)
	} else {
		file, err := G.CopyFile(meta.Id, parentId, 1, false, meta.Size)
		if err != nil {
			G.CloneListener.OnCloneError(err.Error())
			return

		}
		link = G.FormatLink(file.Id)
		G.TotalLength += meta.Size
	}
	if G.isCloneCancelled {
		G.CloneListener.OnCloneError("Cancelled by user.")
		return
	}
	L().Info("CloneDone: " + meta.Name)
	G.CloneListener.OnCloneComplete(link, meta.MimeType == G.GDRIVE_DIR_MIMETYPE)
}

func (G *GoogleDriveClient) CopyFolder(folderId, parentId string, is_thread bool) {
	if is_thread {
		defer G.wg.Done()
	}
	files := G.ListFilesByParentId(folderId, "", -1)
	for _, file := range files {
		if file.MimeType == G.GDRIVE_DIR_MIMETYPE {
			continue
		}
		G.TotalLength += file.Size
	}
	for _, file := range files {
		if G.isCloneCancelled {
			return
		}
		if file.MimeType == G.GDRIVE_DIR_MIMETYPE {
			if utils.UseSa() {
				G.SwitchServiceAccount()
			}
			new_dir, err := G.CreateDir(file.Name, parentId, 1)
			if err != nil {
				L().Error("GDriveCreateDir: " + err.Error())
			} else {
				if is_thread {
					if utils.UseSa() {
						G.SwitchServiceAccount()
					}
					G.wg.Add(1)
					go G.CopyFolder(file.Id, new_dir.Id, true)
				} else {
					G.CopyFolder(file.Id, new_dir.Id, false)
				}

			}
		} else {
			if is_thread {
				if utils.UseSa() {
					G.SwitchServiceAccount()
				}
				G.wg.Add(1)
				go G.CopyFile(file.Id, parentId, 1, true, file.Size)
			} else {
				G.CopyFile(file.Id, parentId, 1, false, file.Size)
			}
		}
	}

}

func (G *GoogleDriveClient) CopyFile(fileId, parentId string, retry int, is_thread bool, size int64) (*drive.File, error) {
	if is_thread {
		defer G.wg.Done()
	}
	if G.isCloneCancelled {
		return nil, fmt.Errorf("Cancelled by user")
	}
	f := &drive.File{
		Parents: []string{parentId},
	}
	file, err := G.DriveSrv.Files.Copy(fileId, f).SupportsAllDrives(true).SupportsTeamDrives(true).Do()
	if err != nil {
		if G.CheckRetry(file, err) {
			if utils.UseSa() {
				G.SwitchServiceAccount()
			}
			if retry <= G.MaxRetries {
				L().Warn("Encountered: ", err.Error(), " retryin: ", retry)
				G.Sleep(G.SleepTime * time.Duration(retry))
				return G.CopyFile(fileId, parentId, retry+1, false, size)
			} else {
				L().Error("Could not copy file (even after retryin): " + err.Error())
				return nil, err
			}
		} else {
			L().Error("Could not copy file: " + err.Error())
			return nil, err
		}
	}
	G.CompletedLength += size
	if !utils.IsTeamDrive() {
		err = G.SetPermissions(file.Id, 1)
		if err != nil {
			L().Error(err)
		}
	}
	return file, err
}

func NewGDriveClient(size int64, listener *MirrorListener) *GoogleDriveClient {
	return &GoogleDriveClient{TotalLength: size, Listener: listener, MaxRetries: 5, SleepTime: 5 * time.Second}
}

func NewGDriveDownload(fileId string, listener *MirrorListener) {
	client := NewGDriveClient(0, listener)
	client.Init("")
	client.Authorize()
	dir := path.Join(utils.GetDownloadDir(), utils.ParseInt64ToString(listener.GetUid()))
	os.MkdirAll(dir, 0755)
	go client.Download(fileId, dir)
	gid := utils.RandString(16)
	status := NewGoogleDriveDownloadStatus(client, gid)
	status.Index_ = GenerateMirrorIndex()
	AddMirrorLocal(listener.GetUid(), status)
	status.GetListener().OnDownloadStart(status.Gid())
}

func NewGDriveClone(fileId string, parentId string, listener *CloneListener) {
	client := NewGDriveClient(0, nil)
	client.Init("")
	client.Authorize()
	client.name = "getting metadata"
	client.CloneListener = listener
	gid := utils.RandString(16)
	status := NewGoogleDriveCloneStatus(client, gid)
	status.Index_ = GenerateMirrorIndex()
	AddMirrorLocal(listener.GetUid(), status)
	go client.Clone(fileId, parentId)
}

type GoogleDriveDownloadStatus struct {
	DriveObj *GoogleDriveClient
	gid      string
	Index_   int
}

func (g *GoogleDriveDownloadStatus) Name() string {
	return g.DriveObj.name
}

func (g *GoogleDriveDownloadStatus) CompletedLength() int64 {
	return g.DriveObj.CompletedLength
}

func (g *GoogleDriveDownloadStatus) TotalLength() int64 {
	return g.DriveObj.TotalLength
}

func (g *GoogleDriveDownloadStatus) Speed() int64 {
	return g.DriveObj.Speed
}

func (g *GoogleDriveDownloadStatus) Gid() string {
	return g.gid
}

func (g *GoogleDriveDownloadStatus) ETA() *time.Duration {
	eta := g.DriveObj.ETA
	return &eta
}

func (g *GoogleDriveDownloadStatus) GetStatusType() string {
	return MirrorStatusDownloading
}

func (g *GoogleDriveDownloadStatus) Path() string {
	return path.Join(utils.GetDownloadDir(), utils.ParseInt64ToString(g.GetListener().GetUid()), g.Name())
}

func (g *GoogleDriveDownloadStatus) Percentage() float32 {
	return float32(g.CompletedLength()*100) / float32(g.TotalLength())
}

func (g *GoogleDriveDownloadStatus) GetListener() *MirrorListener {
	return g.DriveObj.Listener
}

func (g *GoogleDriveDownloadStatus) GetCloneListener() *CloneListener {
	return nil
}

func (g *GoogleDriveDownloadStatus) Index() int {
	return g.Index_
}

func (g *GoogleDriveDownloadStatus) CancelMirror() bool {
	g.DriveObj.CancelDownload()
	return true
}

func NewGoogleDriveDownloadStatus(driveobj *GoogleDriveClient, gid string) *GoogleDriveDownloadStatus {
	return &GoogleDriveDownloadStatus{DriveObj: driveobj, gid: gid}
}

type GoogleDriveStatus struct {
	DriveObj *GoogleDriveClient
	name     string
	gid      string
	Index_   int
}

func (g *GoogleDriveStatus) Name() string {
	return g.name
}

func (g *GoogleDriveStatus) CompletedLength() int64 {
	return g.DriveObj.CompletedLength
}

func (g *GoogleDriveStatus) TotalLength() int64 {
	return g.DriveObj.TotalLength
}

func (g *GoogleDriveStatus) Speed() int64 {
	return g.DriveObj.Speed
}

func (g *GoogleDriveStatus) Gid() string {
	return g.gid
}

func (g *GoogleDriveStatus) ETA() *time.Duration {
	eta := g.DriveObj.ETA
	return &eta
}

func (g *GoogleDriveStatus) GetStatusType() string {
	if g.DriveObj.isUploading {
		return MirrorStatusUploading
	}
	return MirrorStatusUploadQueued
}

func (g *GoogleDriveStatus) Path() string {
	return g.DriveObj.path
}

func (g *GoogleDriveStatus) Percentage() float32 {
	return float32(g.CompletedLength()*100) / float32(g.TotalLength())
}

func (g *GoogleDriveStatus) GetListener() *MirrorListener {
	return g.DriveObj.Listener
}

func (g *GoogleDriveStatus) GetCloneListener() *CloneListener {
	return nil
}

func (g *GoogleDriveStatus) Index() int {
	return g.Index_
}

func (g *GoogleDriveStatus) CancelMirror() bool {
	return true
}

func NewGoogleDriveStatus(driveobj *GoogleDriveClient, name string, gid string) *GoogleDriveStatus {
	return &GoogleDriveStatus{DriveObj: driveobj, name: name, gid: gid}
}

type GoogleDriveCloneStatus struct {
	DriveObj  *GoogleDriveClient
	gid       string
	startTime time.Time
	Index_    int
}

func (g *GoogleDriveCloneStatus) Name() string {
	return g.DriveObj.name
}

func (g *GoogleDriveCloneStatus) CompletedLength() int64 {
	return g.DriveObj.CompletedLength
}

func (g *GoogleDriveCloneStatus) TotalLength() int64 {
	return g.DriveObj.TotalLength
}

func (g *GoogleDriveCloneStatus) Speed() int64 {
	now := time.Now()
	diff := int64(now.Sub(g.startTime).Seconds())
	if diff != 0 {
		return g.CompletedLength() / diff
	}
	return 0
}

func (g *GoogleDriveCloneStatus) Gid() string {
	return g.gid
}

func (g *GoogleDriveCloneStatus) ETA() *time.Duration {
	if g.Speed() != 0 {
		dur := utils.CalculateETA(g.TotalLength()-g.CompletedLength(), g.Speed())
		return &dur
	} else {
		dur := time.Duration(0)
		return &dur
	}
}

func (g *GoogleDriveCloneStatus) GetStatusType() string {
	return MirrorStatusCloning
}

func (g *GoogleDriveCloneStatus) Path() string {
	return g.DriveObj.path
}

func (g *GoogleDriveCloneStatus) Percentage() float32 {
	return float32(g.CompletedLength()*100) / float32(g.TotalLength())
}

func (g *GoogleDriveCloneStatus) GetListener() *MirrorListener {
	return g.DriveObj.Listener
}

func (g *GoogleDriveCloneStatus) GetCloneListener() *CloneListener {
	return g.DriveObj.CloneListener
}

func (g *GoogleDriveCloneStatus) Index() int {
	return g.Index_
}

func (g *GoogleDriveCloneStatus) CancelMirror() bool {
	g.DriveObj.isCloneCancelled = true
	return true
}

func NewGoogleDriveCloneStatus(driveobj *GoogleDriveClient, gid string) *GoogleDriveCloneStatus {
	return &GoogleDriveCloneStatus{DriveObj: driveobj, gid: gid}
}

type ProgressContext struct {
	isCancelled bool
	completed   int64
	total       int64
	drive       TransferListener
}

func (p *ProgressContext) Write(b []byte) (int, error) {
	if p.isCancelled {
		return 0, errors.New("Canceled by user.")
	}
	n := len(b)
	p.completed += int64(n)
	p.drive.OnTransferUpdate(p.completed, p.total)
	return n, nil
}

func (p *ProgressContext) Cancel() {
	p.isCancelled = true
}

func NewProgressContext(size int64, t TransferListener) *ProgressContext {
	return &ProgressContext{drive: t, total: size, completed: 0}
}
