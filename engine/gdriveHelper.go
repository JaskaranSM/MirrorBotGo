package engine

import (
	"MirrorBotGo/utils"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
)

type GoogleDriveClient struct {
	RootId              string
	GDRIVE_DIR_MIMETYPE string
	TokenFile           string
	CredentialFile      string
	TotalLength         int64
	CompletedLength     int64
	LastTransferred     int64
	Speed               int64
	MaxRetries          int
	SleepTime           time.Duration
	StartTime           time.Time
	ETA                 time.Duration
	DriveSrv            *drive.Service
	Listener            *MirrorListener
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

func (G *GoogleDriveClient) Authorize() {
	b, err := ioutil.ReadFile(G.CredentialFile)
	if err != nil {
		G.Listener.OnUploadError("Unable to read client secret file: " + err.Error())
		return
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, drive.DriveScope)
	if err != nil {
		G.Listener.OnUploadError("Unable to parse client secret file to config: " + err.Error())
		return
	}
	client := G.getClient(config)

	srv, err := drive.New(client)
	if err != nil {
		G.Listener.OnUploadError("Unable to retrieve Drive client: " + err.Error())
		return
	}
	G.DriveSrv = srv
}

func (G *GoogleDriveClient) getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web %v", err)
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
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func (G *GoogleDriveClient) CreateDir(name string, parentId string, retry int) (*drive.File, error) {
	d := &drive.File{
		Name:     name,
		MimeType: G.GDRIVE_DIR_MIMETYPE,
		Parents:  []string{parentId},
	}
	file, err := G.DriveSrv.Files.Create(d).SupportsAllDrives(true).Do()
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "rate") {
			if retry <= G.MaxRetries {
				log.Println("Encountered: ", err.Error(), " retryin: ", retry)
				time.Sleep(G.SleepTime)
				return G.CreateDir(name, parentId, retry+1)
			} else {
				log.Println("Could not create dir (even after retryin): " + err.Error())
				return nil, err
			}
		} else {
			log.Println("Could not create dir: " + err.Error())
			return nil, err
		}
	}
	fmt.Println("Created G-Drive Folder: ", file.Id)
	if !utils.IsTeamDrive() {
		err = G.SetPermissions(file.Id, 1)
		if err != nil {
			log.Println(err)
		}
	}
	return file, nil
}

func (G *GoogleDriveClient) Upload(path string) bool {
	var link string
	var file *drive.File
	var f os.FileInfo
	var err error
	f, err = os.Stat(path)
	if err != nil {
		G.Listener.OnUploadError(err.Error())
		return false
	}
	if f.IsDir() {
		file, err = G.CreateDir(f.Name(), utils.GetGDriveParentId(), 1)
		if err != nil {
			G.Listener.OnUploadError(err.Error())
			return false
		}
		G.UploadDirRec(path, file.Id)
		link = G.FormatLink(file.Id)
	} else {
		file, err = G.UploadFile(utils.GetGDriveParentId(), path, 1)
		if err != nil {
			G.Listener.OnUploadError(err.Error())
			return false
		}
		link = G.FormatLink(file.Id)
	}
	G.Listener.OnUploadComplete(link)
	return true
}

func (G *GoogleDriveClient) UploadDirRec(directoryPath string, parentId string) bool {
	files, err := ioutil.ReadDir(directoryPath)
	if err != nil {
		G.Listener.OnUploadError(err.Error())
		return false
	}
	for _, f := range files {
		currentFile := path.Join(directoryPath, f.Name())
		if f.IsDir() {
			file, err := G.CreateDir(f.Name(), parentId, 1)
			if err != nil {
				G.Listener.OnUploadError(err.Error())
				return false
			}
			G.UploadDirRec(currentFile, file.Id)
		} else {
			file, err := G.UploadFile(parentId, currentFile, 1)
			if err != nil {
				G.Listener.OnUploadError(err.Error())
				return false
			} else {
				log.Println("Uploaded File: ", file.Id)
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
	_, err := G.DriveSrv.Permissions.Create(fileId, permission).Fields("").SupportsAllDrives(true).SupportsTeamDrives(true).Do()
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "rate") {
			if retry <= G.MaxRetries {
				log.Println("Encountered: ", err.Error(), " retryin: ", retry)
				time.Sleep(G.SleepTime)
				return G.SetPermissions(fileId, retry+1)
			} else {
				log.Println("Could not set file permissions (even after retryin): " + err.Error())
				return err
			}
		} else {
			log.Println("Could not set file permissions: " + err.Error())
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
			log.Printf("Error : %v\n", err)
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

func (G *GoogleDriveClient) GetFileMetadata(fileId string) (*drive.File, error) {
	return G.DriveSrv.Files.Get(fileId).Fields("name,mimeType,size,id,md5Checksum").SupportsAllDrives(true).Do()
}

func (G *GoogleDriveClient) UploadFile(parentId string, file_path string, retry int) (*drive.File, error) {
	defer G.Clean()
	content, err := os.Open(file_path)
	if err != nil {
		fmt.Println(err)
	}
	contentType, err := utils.GetFileContentType(content)
	if err != nil {
		fmt.Println(err)
	}
	arr := strings.Split(file_path, "/")
	name := arr[len(arr)-1]
	f := &drive.File{
		MimeType: contentType,
		Name:     name,
		Parents:  []string{parentId},
	}
	ctx := context.Background()
	stat, err := content.Stat()
	if err != nil {
		log.Println(err)
		return nil, err
	}
	log.Printf("Uploading %s with mimeType: %s", f.Name, f.MimeType)
	file, err := G.DriveSrv.Files.Create(f).ResumableMedia(ctx, content, stat.Size(), contentType).ProgressUpdater(G.OnTransferUpdate).SupportsAllDrives(true).Do()

	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "rate") {
			if retry <= G.MaxRetries {
				log.Println("Encountered: ", err.Error(), " retryin: ", retry)
				time.Sleep(G.SleepTime)
				return G.UploadFile(parentId, file_path, retry+1)
			} else {
				log.Println("Could not create file (even after retryin): " + err.Error())
				return nil, err
			}
		} else {
			log.Println("Could not create file: " + err.Error())
			return nil, err
		}

	}
	if !utils.IsTeamDrive() {
		err = G.SetPermissions(file.Id, 1)
		if err != nil {
			log.Println(err)
		}
	}
	return file, nil
}

func (G *GoogleDriveClient) Clone(fileId string) (string, error) {
	var link string
	meta, err := G.GetFileMetadata(fileId)
	if err != nil {
		return link, err
	}
	log.Println("Cloning: " + meta.Name)
	if meta.MimeType == G.GDRIVE_DIR_MIMETYPE {
		new_dir, err := G.CreateDir(meta.Name, utils.GetGDriveParentId(), 1)
		if err != nil {
			log.Println("GDriveCreateDir: " + err.Error())
			return link, err
		} else {
			G.CopyFolder(meta.Id, new_dir.Id)
		}
		link = G.FormatLink(new_dir.Id)
	} else {
		file, err := G.CopyFile(meta.Id, utils.GetGDriveParentId(), 1)
		if err != nil {
			return link, err
		}
		link = G.FormatLink(file.Id)
		G.TotalLength += meta.Size
	}
	log.Println("CloneDone: " + meta.Name)
	out_str := fmt.Sprintf("<a href='%s'>%s</a> (%s)", link, meta.Name, utils.GetHumanBytes(G.TotalLength))
	in_url := utils.GetIndexUrl()
	if in_url != "" {
		in_url = in_url + "/" + meta.Name
		if meta.MimeType == G.GDRIVE_DIR_MIMETYPE {
			in_url += "/"
		}
		out_str += fmt.Sprintf("\n\n Shareable Link: <a href='%s'>here</a>", in_url)
	}
	return out_str, nil
}

func (G *GoogleDriveClient) CopyFolder(folderId, parentId string) {
	files := G.ListFilesByParentId(folderId, "", -1)
	for _, file := range files {
		if file.MimeType == G.GDRIVE_DIR_MIMETYPE {
			new_dir, err := G.CreateDir(file.Name, parentId, 1)
			if err != nil {
				log.Println("GDriveCreateDir: " + err.Error())
			} else {
				G.CopyFolder(file.Id, new_dir.Id)
			}
		} else {
			_, err := G.CopyFile(file.Id, parentId, 1)
			if err != nil {
				log.Println("GDriveCopy: " + err.Error())
			}
			G.TotalLength += file.Size
		}
	}

}

func (G *GoogleDriveClient) CopyFile(fileId, parentId string, retry int) (*drive.File, error) {
	f := &drive.File{
		Parents: []string{parentId},
	}
	file, err := G.DriveSrv.Files.Copy(fileId, f).SupportsAllDrives(true).SupportsTeamDrives(true).Do()
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "rate") {
			if retry <= G.MaxRetries {
				log.Println("Encountered: ", err.Error(), " retryin: ", retry)
				time.Sleep(G.SleepTime)
				return G.CopyFile(fileId, parentId, retry+1)
			} else {
				log.Println("Could not copy file (even after retryin): " + err.Error())
				return nil, err
			}
		} else {
			log.Println("Could not copy file: " + err.Error())
			return nil, err
		}
	}
	if !utils.IsTeamDrive() {
		err = G.SetPermissions(file.Id, 1)
		if err != nil {
			log.Println(err)
		}
	}
	return file, err
}

func NewGDriveClient(size int64, listener *MirrorListener) *GoogleDriveClient {
	return &GoogleDriveClient{TotalLength: size, Listener: listener, MaxRetries: 5, SleepTime: 5 * time.Second}
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
	return MirrorStatusUploading
}

func (g *GoogleDriveStatus) Path() string {
	return path.Join(utils.GetDownloadDir(), utils.ParseIntToString(g.GetListener().GetUid()), g.Name())
}

func (g *GoogleDriveStatus) Percentage() float32 {
	return float32(g.CompletedLength()*100) / float32(g.TotalLength())
}

func (g *GoogleDriveStatus) GetListener() *MirrorListener {
	return g.DriveObj.Listener
}

func (g *GoogleDriveStatus) Index() int {
	return g.Index_
}

func (g *GoogleDriveStatus) CancelMirror() bool {
	listener := g.GetListener()
	listener.OnUploadError("Canceled by user.")
	return true
}

func NewGoogleDriveStatus(driveobj *GoogleDriveClient, name string, gid string) *GoogleDriveStatus {
	return &GoogleDriveStatus{DriveObj: driveobj, name: name, gid: gid}
}
