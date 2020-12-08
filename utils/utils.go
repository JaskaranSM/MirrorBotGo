package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/signal"
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
	BOT_TOKEN              string `json:"bot_token"`
	SUDO_USERS             []int  `json:"sudo_users"`
	AUTHORIZED_CHATS       []int  `json:"authorized_chats"`
	OWNER_ID               int    `json:"owner_id"`
	DOWNLOAD_DIR           string `json:"download_dir"`
	IS_TEAM_DRIVE          bool   `json:"is_team_drive"`
	GDRIVE_PARENT_ID       string `json:"gdrive_parent_id"`
	STATUS_UPDATE_INTERVAL int    `json:"status_update_interval"`
	AUTO_DELETE_TIMEOUT    int    `json:"auto_delete_timeout"`
	DB_URI                 string `json:"db_uri"`
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

func GetSudoUsers() []int {
	return Config.SUDO_USERS
}

func GetAuthorizedChats() []int {
	return Config.AUTHORIZED_CHATS
}

func GetMaxMessageTextLength() int {
	return MaxMessageTextLength
}

func IsUserOwner(userId int) bool {
	return Config.OWNER_ID == userId
}

func IsUserSudo(userId int) bool {
	for _, i := range Config.SUDO_USERS {
		if i == userId {
			return true
		}
	}
	return false
}

func GetDownloadDir() string {
	return Config.DOWNLOAD_DIR
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

func GetReaderHandleByUrl(link string) (io.Reader, error) {
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

func ParseInterfaceToInt(i interface{}) int {
	tempType := reflect.TypeOf(i).Name()
	if tempType == "int32" {
		return int(i.(int32))
	}
	return int(i.(int64))
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
