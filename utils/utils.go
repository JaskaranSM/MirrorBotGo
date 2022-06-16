package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/lithammer/shortuuid"
)

const ConfigJsonPath string = "config.json"

const PROGRESS_MAX_SIZE = 100 / 8
const MaxMessageTextLength int = 4000

var PROGRESS_INCOMPLETE []string = []string{"▏", "▎", "▍", "▌", "▋", "▊", "▉"}

const (
	MAGNET_REGEX     string = "magnet:\\?xt=urn:btih:[a-zA-Z0-9]*"
	URL_REGEX        string = "(?:(?:https?|ftp):\\/\\/)?[\\w/\\-?=%.]+\\.[\\w/\\-?=%.]+"
	DRIVE_LINK_REGEX string = `https://drive\.google\.com/(drive)?/?u?/?\d?/?(mobile)?/?(file)?(folders)?/?d?/([-\w]+)[?+]?/?(w+)?`
)

func IsMagnetLink(link string) bool {
	match := regexp.MustCompile(MAGNET_REGEX)
	return match.MatchString(link)
}

func IsUrlLink(link string) bool {
	match := regexp.MustCompile(URL_REGEX)
	return match.MatchString(link)
}

type ConfigJson struct {
	BOT_TOKEN              string  `json:"bot_token"`
	SUDO_USERS             []int64 `json:"sudo_users"`
	AUTHORIZED_CHATS       []int64 `json:"authorized_chats"`
	OWNER_ID               int64   `json:"owner_id"`
	DOWNLOAD_DIR           string  `json:"download_dir"`
	IS_TEAM_DRIVE          bool    `json:"is_team_drive"`
	GDRIVE_PARENT_ID       string  `json:"gdrive_parent_id"`
	STATUS_UPDATE_INTERVAL int     `json:"status_update_interval"`
	AUTO_DELETE_TIMEOUT    int     `json:"auto_delete_timeout"`
	DB_URI                 string  `json:"db_uri"`
	USE_SA                 bool    `json:"use_sa"`
	INDEX_URL              string  `json:"index_url"`
	TG_APP_ID              string  `json:"tg_app_id"`
	TG_APP_HASH            string  `json:"tg_app_hash"`
	MegaEmail              string  `json:"mega_email"`
	MegaPassword           string  `json:"mega_password"`
	MegaAPIKey             string  `json:"mega_api_key"`
	StatusMessagesPerPage  int     `json:"status_messages_per_page"`
	EncryptionPassword     string  `json:"encryption_password"`
	Seed                   bool    `json:"seed"`
}

var Config *ConfigJson = InitConfig()

func InitConfig() *ConfigJson {
	file, err := ioutil.ReadFile(ConfigJsonPath)
	if err != nil {
		log.Fatal("Config File Bad, exiting!")
	}
	var Config ConfigJson
	err = json.Unmarshal([]byte(file), &Config)
	if err != nil {
		log.Fatal(err)
	}
	Config.SUDO_USERS = append(Config.SUDO_USERS, Config.OWNER_ID)
	log.Println(Config.SUDO_USERS)
	return &Config
}

func GetBotToken() string {
	return Config.BOT_TOKEN
}

func GetSudoUsers() []int64 {
	return Config.SUDO_USERS
}

func GetAuthorizedChats() []int64 {
	return Config.AUTHORIZED_CHATS
}

func GetTgAppId() string {
	return Config.TG_APP_ID
}

func GetMegaEmail() string {
	return Config.MegaEmail
}

func GetStatusMessagesPerPage() int {
	if Config.StatusMessagesPerPage == 0 {
		return 5
	}
	return Config.StatusMessagesPerPage
}

func GetEncryptionPassword() string {
	if Config.EncryptionPassword == "" {
		return "zerocool"
	}
	return Config.EncryptionPassword
}

func GetMegaPasssword() string {
	return Config.MegaPassword
}

func GetTgAppHash() string {
	return Config.TG_APP_HASH
}

func GetMaxMessageTextLength() int {
	return MaxMessageTextLength
}

func UseSa() bool {
	return Config.USE_SA
}

func IsUserOwner(userId int64) bool {
	return Config.OWNER_ID == userId
}

func IsUserSudo(userId int64) bool {
	for _, i := range Config.SUDO_USERS {
		if i == userId {
			return true
		}
	}
	return false
}

func GetMegaAPIKey() string {
	return Config.MegaAPIKey
}

func GetSeed() bool {
	return Config.Seed
}

func GetDownloadDir() string {
	return Config.DOWNLOAD_DIR
}

func GetIndexUrl() string {
	return Config.INDEX_URL
}

func GetAutoDeleteTimeOut() time.Duration {
	return time.Duration(Config.AUTO_DELETE_TIMEOUT) * time.Second
}

func GetGDriveParentId() string {
	if Config.GDRIVE_PARENT_ID == "" {
		return "root"
	}
	return Config.GDRIVE_PARENT_ID
}

func GetHttpUserAgent() string {
	return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/85.0.4183.102 Safari/537.36"
}

func IsTeamDrive() bool {
	return Config.IS_TEAM_DRIVE
}

func GetStatusUpdateInterval() time.Duration {
	return time.Duration(Config.STATUS_UPDATE_INTERVAL) * time.Second
}

func GetDbUri() string {
	return Config.DB_URI
}

func GetHumanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}

func ParseMessageArgs(m string) string {
	args := strings.SplitN(m, " ", 2)
	if len(args) >= 2 {
		return args[1]
	}
	return ""
}

func RemoveByPath(pth string) error {
	return os.RemoveAll(pth)
}

func GetFileContentTypePath(file_path string) (string, error) {
	file, err := os.Open(file_path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	return GetFileContentType(file)
}

func GetFileContentType(out *os.File) (string, error) {
	buffer := make([]byte, 512)

	_, err := out.Read(buffer)
	if err != nil {
		return "", err
	}
	contentType := http.DetectContentType(buffer)

	return contentType, nil
}

func GetShortId() string {
	return shortuuid.New()
}

func GetFileNameLink(link string) (string, error) {
	var err error
	var params map[string]string
	var fname string
	req, err := http.Get(link)
	if err != nil {
		return fname, err
	}
	_, params, err = mime.ParseMediaType(req.Header.Get("Content-Disposition"))
	if err != nil {
		return fname, err
	} else {
		for i, j := range params {
			if i == "filename" {
				fname = j
			}
		}
	}
	return fname, nil
}

func EndsWithTorrent(str string) bool {
	return strings.HasSuffix(str, ".torrent")
}

func GetUrlInfo(url string) (string, string, error) { // fileName,mimeType,error
	var err error
	var params map[string]string
	var fname string
	var mimeType string
	resp, err := http.Get(url)
	if err != nil {
		return fname, mimeType, err
	}
	mim, er := mimetype.DetectReader(resp.Body)
	if er != nil {
		return fname, mimeType, err
	}
	mimeType = mim.String()
	_, params, _ = mime.ParseMediaType(resp.Header.Get("Content-Disposition"))
	for i, j := range params {
		if i == "filename" {
			fname = j
		}
	}
	return fname, mimeType, err
}

func GetFileBaseName(path string) string {
	data := strings.Split(path, "/")
	if len(data) >= 1 {
		return data[len(data)-1]
	}
	return path
}

func IsTorrentLink(link string) (bool, error) {
	if strings.Contains(link, "magnet") {
		return true, nil
	}
	fname, mimeType, err := GetUrlInfo(link)
	if err != nil {
		return false, err
	}
	if EndsWithTorrent(fname) || strings.Contains(mimeType, "torrent") {
		return true, nil
	}
	return false, nil
}

func HumanizeDuration(duration time.Duration) string {
	if duration.Seconds() < 60.0 {
		return fmt.Sprintf("%ds", int64(duration.Seconds()))
	}
	if duration.Minutes() < 60.0 {
		remainingSeconds := math.Mod(duration.Seconds(), 60)
		return fmt.Sprintf("%dm %ds", int64(duration.Minutes()), int64(remainingSeconds))
	}
	if duration.Hours() < 24.0 {
		remainingMinutes := math.Mod(duration.Minutes(), 60)
		remainingSeconds := math.Mod(duration.Seconds(), 60)
		return fmt.Sprintf("%dh %dm %ds",
			int64(duration.Hours()), int64(remainingMinutes), int64(remainingSeconds))
	}
	remainingHours := math.Mod(duration.Hours(), 24)
	remainingMinutes := math.Mod(duration.Minutes(), 60)
	remainingSeconds := math.Mod(duration.Seconds(), 60)
	return fmt.Sprintf("%dd %dh %dm %ds",
		int64(duration.Hours()/24), int64(remainingHours),
		int64(remainingMinutes), int64(remainingSeconds))
}

func GetProgressBarString(current, total int) string {
	var (
		p      int
		cFull  int
		cPart  int
		pStr   string = "█"
		sStr   string = " "
		outStr string
	)
	if total == 0 {
		p = 0
	} else {
		p = current * 100 / total
	}
	p = int(math.Min(math.Max(float64(p), 0), 100))
	cFull = p / 8
	cPart = p%8 - 1
	outStr += strings.Repeat(pStr, cFull)
	if cPart >= 0 {
		outStr += PROGRESS_INCOMPLETE[cPart]
	}
	outStr += strings.Repeat(sStr, PROGRESS_MAX_SIZE-cFull)
	return fmt.Sprintf("[%s]", outStr)
}

func CalculateETA(bytesLeft, speed int64) time.Duration {
	if speed == 0 {
		return time.Duration(0)
	}
	eta := time.Duration(bytesLeft/speed) * time.Second
	switch {
	case eta > 8*time.Hour:
		eta = eta.Round(time.Hour)
	case eta > 4*time.Hour:
		eta = eta.Round(30 * time.Minute)
	case eta > 2*time.Hour:
		eta = eta.Round(15 * time.Minute)
	case eta > time.Hour:
		eta = eta.Round(5 * time.Minute)
	case eta > 30*time.Minute:
		eta = eta.Round(1 * time.Minute)
	case eta > 15*time.Minute:
		eta = eta.Round(30 * time.Second)
	case eta > 5*time.Minute:
		eta = eta.Round(15 * time.Second)
	case eta > time.Minute:
		eta = eta.Round(5 * time.Second)
	}
	return eta
}

func GetReaderHandleByUrl(link string) (io.ReadCloser, error) {
	resp, err := http.Get(link)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func FormatTGFileLink(sub string, token string) string {
	return fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", token, sub)
}

func ExitCleanup() {
	killSignal := make(chan os.Signal, 1)
	signal.Notify(killSignal, os.Interrupt)
	<-killSignal
	log.Println("Exit Cleanup")
	RemoveByPath(GetDownloadDir())
	os.Exit(1)
}

func ParseStringToInt64(str string) int64 {
	if n, err := strconv.Atoi(str); err == nil {
		return int64(n)
	} else {
		return 0
	}
}

func ParseIntToString(i int) string {
	return strconv.Itoa(i)
}

func ParseInt64ToString(i int64) string {
	return strconv.FormatInt(i, 10)
}

func ParseInterfaceToInt(i interface{}) int {
	tempType := reflect.TypeOf(i).Name()
	if tempType == "int32" {
		return int(i.(int32))
	}
	return int(i.(int64))
}

func ParseInterfaceToInt64(i interface{}) int64 {
	tempType := reflect.TypeOf(i).Name()
	if tempType == "int32" {
		return int64(i.(int32))
	}
	return i.(int64)
}

func GetFileIdByGDriveLink(link string) string {
	match := regexp.MustCompile(DRIVE_LINK_REGEX)
	matches := match.FindStringSubmatch(link)
	if len(matches) >= 2 {
		return matches[len(matches)-2]
	}
	urlParsed, err := url.Parse(link)
	if err != nil {
		return ""
	}
	values := urlParsed.Query()
	if len(values) == 0 {
		return ""
	}
	for i, j := range values {
		if i == "id" {
			return j[0]
		}
	}
	return ""
}

func IsPathDir(pth string) bool {
	fi, err := os.Stat(pth)
	if err != nil {
		log.Println("CheckPathError: " + err.Error())
		return false
	}
	mode := fi.Mode()
	return mode.IsDir()
}

func IsPathExists(pth string) bool {
	if _, err := os.Stat(pth); os.IsNotExist(err) {
		return false
	}
	return true
}

func GetFileBaseNameNoExt(path string) string {
	basename := GetFileBaseName(path)
	return strings.TrimSuffix(basename, filepath.Ext(basename))
}

func TrimExt(path string) string {
	return strings.TrimSuffix(path, filepath.Ext(path))
}

func GetLinksFromTextFileLink(url string) ([]string, error) {
	var links []string
	res, err := http.Get(url)
	if err != nil {
		return links, err
	}
	defer res.Body.Close()
	cnt, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return links, err
	}
	cntString := string(cnt)
	data := strings.Split(cntString, "\n")
	for _, d := range data {
		if d != "" {
			links = append(links, d)
		}
	}
	return links, nil
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func IsMegaFolderLink(link string) bool {
	return strings.Contains(link, "/folder/")
}

func MegaLinkToFolderId(link string) (string, string) {
	data := strings.Split(link, "#")
	var folderid string
	var folderkey string
	if len(data) > 1 {
		folderkey = data[1]
	}
	dt := strings.Split(data[0], "/")
	folderid = dt[len(dt)-1]
	return folderid, folderkey
}

func IsMegaLink(link string) bool {
	return strings.Contains(link, "mega.nz")
}

func GetCommandLineArgs() map[string]string {
	var args map[string]string = make(map[string]string)
	if len(os.Args) < 2 {
		return args
	}
	for _, i := range os.Args[1:] {
		if strings.Contains(i, "=") {
			data := strings.Split(i, "=")
			if len(data) > 1 {
				args[data[0]] = data[1]
			}
		}
	}
	return args
}

func GetEnvironmentArgs(key string) map[string]string {
	var args map[string]string = make(map[string]string)
	for _, e := range os.Environ() {
		if !strings.Contains(e, "=") {
			continue
		}
		data := strings.SplitN(e, "=", 2)
		if len(data) > 1 {
			if data[0] == key {
				t := strings.Split(data[1], ",")
				for _, j := range t {
					if !strings.Contains(j, "=") {
						continue
					}
					h := strings.Split(j, "=")
					if len(h) > 1 {
						args[h[0]] = h[1]
					}
				}
			}
		}
	}
	return args
}
