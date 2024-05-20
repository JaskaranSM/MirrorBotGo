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
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/gabriel-vasile/mimetype"
	"github.com/lithammer/shortuuid"
)

const ConfigJsonPath string = "config.json"

const ProgressMaxSize = 100 / 8
const MaxMessageTextLength int = 4000

var ProgressIncomplete []string = []string{"▏", "▎", "▍", "▌", "▋", "▊", "▉"}

const (
	MagnetRegex    string = "magnet:\\?xt=urn:btih:[a-zA-Z0-9]*"
	UrlRegex       string = "(?:(?:https?|ftp):\\/\\/)?[\\w/\\-?=%.]+\\.[\\w/\\-?=%.]+"
	DriveLinkRegex string = `https://drive\.google\.com/(drive)?/?u?/?\d?/?(mobile)?/?(file)?(folders)?/?d?/([-\w]+)[?+]?/?(w+)?`
)

func IsMagnetLink(link string) bool {
	match := regexp.MustCompile(MagnetRegex)
	return match.MatchString(link)
}

func IsUrlLink(link string) bool {
	match := regexp.MustCompile(UrlRegex)
	return match.MatchString(link)
}

type ConfigJson struct {
	BotToken                                    string  `json:"bot_token"`
	SudoUsers                                   []int64 `json:"sudo_users"`
	AuthorizedChats                             []int64 `json:"authorized_chats"`
	OwnerId                                     int64   `json:"owner_id"`
	DownloadDir                                 string  `json:"download_dir"`
	IsTeamDrive                                 bool    `json:"is_team_drive"`
	GdriveParentId                              string  `json:"gdrive_parent_id"`
	StatusUpdateInterval                        int     `json:"status_update_interval"`
	AutoDeleteTimeout                           int     `json:"auto_delete_timeout"`
	DbUri                                       string  `json:"db_uri"`
	UseSa                                       bool    `json:"use_sa"`
	IndexUrl                                    string  `json:"index_url"`
	TgAppId                                     string  `json:"tg_app_id"`
	TgAppHash                                   string  `json:"tg_app_hash"`
	MegaEmail                                   string  `json:"mega_email"`
	MegaPassword                                string  `json:"mega_password"`
	MegaAPIKey                                  string  `json:"mega_api_key"`
	MegaSDKRestServiceURL                       string  `json:"mega_sdk_rest_service_url"`
	StatusMessagesPerPage                       int     `json:"status_messages_per_page"`
	EncryptionPassword                          string  `json:"encryption_password"`
	Seed                                        bool    `json:"seed"`
	HealthCheckRouterURL                        string  `json:"health_check_router_url"`
	TransferServiceURL                          string  `json:"transfer_service_url"`
	UsenetClientURL                             string  `json:"usenet_client_url"`
	UsenetClientUsername                        string  `json:"usenet_client_username"`
	UsenetClientPassword                        string  `json:"usenet_client_password"`
	TorrentClientListenPort                     int     `json:"torrent_client_listen_port"`
	TorrentClientHTTPUserAgent                  string  `json:"torrent_client_http_user_agent"`
	TorrentClientBep20                          string  `json:"torrent_client_bep_20"`
	TorrentClientUpnpID                         string  `json:"torrent_client_upnp_id"`
	TorrentClientMaxUploadRate                  string  `json:"torrent_client_max_upload_rate"`
	TorrentClientMinDialTimeout                 int     `json:"torrent_client_min_dial_timeout"`
	TorrentClientEstablishedConnsPerTorrent     int     `json:"torrent_client_established_conns_per_torrent"`
	TorrentClientExtendedHandshakeClientVersion string  `json:"torrent_client_extended_handshake_client_version"`
	TorrentUseTrackerList                       bool    `json:"torrent_use_tracker_list"`
	TorrentTrackerListURL                       string  `json:"torrent_tracker_list_url"`
	KedgeURL                                    string  `json:"kedge_url"`
	ZipStreamerURL                              string  `json:"zip_streamer_url"`
	SpamFilterMessagesPerDuration               int     `json:"spam_filter_messages_per_duration"`
	SpamFilterDurationValue                     int     `json:"spam_filter_duration_value"`
	StatusMessageAutoDeleteTime                 int     `json:"status_message_auto_delete_time"`
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
	Config.SudoUsers = append(Config.SudoUsers, Config.OwnerId)
	log.Println(Config.SudoUsers)
	return &Config
}

func GetSpamFilterMessagesPerDuration() int {
	if Config.SpamFilterMessagesPerDuration == 0 {
		return 1
	}
	return Config.SpamFilterMessagesPerDuration
}

func GetSpamFilterDurationValue() int {
	if Config.SpamFilterDurationValue == 0 {
		return 10
	}
	return Config.SpamFilterDurationValue
}

func GetBotToken() string {
	return Config.BotToken
}

func GetSudoUsers() []int64 {
	return Config.SudoUsers
}

func GetKedgeURL() string {
	if Config.KedgeURL == "" {
		return "http://localhost:16180/api"
	}
	return Config.KedgeURL
}

func GetAuthorizedChats() []int64 {
	return Config.AuthorizedChats
}

func GetTgAppId() string {
	return Config.TgAppId
}

func GetMegaEmail() string {
	return Config.MegaEmail
}

func GetHealthCheckRouterURL() string {
	if Config.HealthCheckRouterURL == "" {
		return "localhost:7870"
	}
	return Config.HealthCheckRouterURL
}

func GetTransferServiceURL() string {
	if Config.TransferServiceURL == "" {
		return "http://127.0.0.1:6969/api/v1"
	}
	return Config.TransferServiceURL
}

func GetUsenetClientURL() string {
	if Config.UsenetClientURL == "" {
		return "http://127.0.0.1:6789"
	}
	return Config.UsenetClientURL
}

func GetUsenetClientUsername() string {
	if Config.UsenetClientUsername == "" {
		return "nzbget"
	}
	return Config.UsenetClientUsername
}

func GetUsenetClientPassword() string {
	if Config.UsenetClientPassword == "" {
		return "tegbzn6789"
	}
	return Config.UsenetClientPassword
}

func GetTorrentClientListenPort() int {
	if Config.TorrentClientListenPort == 0 {
		return 42069
	}
	return Config.TorrentClientListenPort
}

func GetTorrentClientBep20() string {
	if Config.TorrentClientBep20 == "" {
		return "-qB4380-"
	}
	return Config.TorrentClientBep20
}

func GetTorrentClientUpnpID() string {
	if Config.TorrentClientUpnpID == "" {
		return "qBittorrent 4.3.8"
	}
	return Config.TorrentClientUpnpID
}

func GetTorrentClientExtendedHandshakeClientVersion() string {
	if Config.TorrentClientExtendedHandshakeClientVersion == "" {
		return "qBittorrent/4.3.8"
	}
	return Config.TorrentClientExtendedHandshakeClientVersion
}

func GetTorrentClientMinDialTimeout() time.Duration {
	if Config.TorrentClientMinDialTimeout == 0 {
		return 10 * time.Second
	}
	return time.Duration(Config.TorrentClientMinDialTimeout) * time.Second
}

func GetTorrentClientEstablishedConnsPerTorrent() int {
	if Config.TorrentClientEstablishedConnsPerTorrent == 0 {
		return 100
	}
	return Config.TorrentClientEstablishedConnsPerTorrent
}

func GetTorrentClientMaxUploadRate() (uint64, error) {
	return humanize.ParseBytes(Config.TorrentClientMaxUploadRate)
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

func GetMegaPassword() string {
	return Config.MegaPassword
}

func GetTgAppHash() string {
	return Config.TgAppHash
}

func GetZipStreamerURL() string {
	return Config.ZipStreamerURL
}

func GetMaxMessageTextLength() int {
	return MaxMessageTextLength
}

func UseSa() bool {
	return Config.UseSa
}

func IsUserOwner(userId int64) bool {
	return Config.OwnerId == userId
}

func IsUserSudo(userId int64) bool {
	for _, i := range Config.SudoUsers {
		if i == userId {
			return true
		}
	}
	return false
}

func GetMegaAPIKey() string {
	return Config.MegaAPIKey
}

func GetMegaSDKRestServiceURL() string {
	if Config.MegaSDKRestServiceURL == "" {
		return "http://localhost:8069"
	}
	return Config.MegaSDKRestServiceURL
}

func GetSeed() bool {
	return Config.Seed
}

func GetDownloadDir() string {
	return Config.DownloadDir
}

func GetIndexUrl() string {
	return Config.IndexUrl
}

func GetAutoDeleteTimeOut() time.Duration {
	return time.Duration(Config.AutoDeleteTimeout) * time.Second
}

func GetGDriveParentId() string {
	if Config.GdriveParentId == "" {
		return "root"
	}
	return Config.GdriveParentId
}

func GetTorrentTrackerListURL() string {
	if Config.TorrentTrackerListURL == "" {
		return "https://raw.githubusercontent.com/ngosang/trackerslist/master/trackers_all.txt"
	}
	return Config.TorrentTrackerListURL
}

func GetTorrentUseTrackerList() bool {
	return Config.TorrentUseTrackerList
}

func GetTorrentClientHTTPUserAgent() string {
	if Config.TorrentClientHTTPUserAgent == "" {
		return "qBittorrent/4.3.8"
	}
	return Config.TorrentClientHTTPUserAgent
}

func GetHttpUserAgent() string {
	return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/85.0.4183.102 Safari/537.36"
}

func IsTeamDrive() bool {
	return Config.IsTeamDrive
}

func GetStatusUpdateInterval() time.Duration {
	return time.Duration(Config.StatusUpdateInterval) * time.Second
}

func GetDbUri() string {
	return Config.DbUri
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

func GetFileContentTypePath(filePath string) (string, error) {
	file, err := os.Open(filePath)
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
		outStr += ProgressIncomplete[cPart]
	}
	outStr += strings.Repeat(sStr, ProgressMaxSize-cFull)
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

func GetFileIdByGDriveLinkParams(link string) string {
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

func GetFileIdByGDriveLink(link string) string {
	if !strings.Contains(link, "https://drive.google.com") {
		return ""
	}
	id := GetFileIdByGDriveLinkParams(link)
	if id != "" {
		return id
	}
	match := regexp.MustCompile(DriveLinkRegex)
	matches := match.FindStringSubmatch(link)
	if len(matches) >= 2 {
		return matches[len(matches)-2]
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
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("GetLinksFromTextFileLink: Error while closing response handle: %s : %v", url, err)
		}
	}(res.Body)
	cnt, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return links, err
	}
	cntString := string(cnt)
	data := strings.Split(cntString, "\n")
	for _, d := range data {
		if d != "" {
			links = append(links, strings.TrimSpace(d))
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
	var folderId string
	var folderKey string
	if len(data) > 1 {
		folderKey = data[1]
	}
	dt := strings.Split(data[0], "/")
	folderId = dt[len(dt)-1]
	return folderId, folderKey
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

func TrimString(text string) string {
	if len(text) > 20 {
		return string(text[:20]) + "..."
	}
	return text
}

func GetStatusMessageAutoDeleteTime() int {
	return Config.StatusMessageAutoDeleteTime
}

func ParseMessageFloodWaitDuration(err error) (int, error) {
	if err == nil {
		return -1, fmt.Errorf("error cannot be nil for this function")
	}
	valueInString := strings.SplitN(err.Error(), "retry after ", 2)[1]
	value, err := strconv.Atoi(valueInString)
	return value, err
}
