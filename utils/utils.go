package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/lithammer/shortuuid"
)

const ConfigJsonPath string = "config.json"

const PROGRESS_MAX_SIZE = 100 / 8

var PROGRESS_INCOMPLETE []string = []string{"▏", "▎", "▍", "▌", "▋", "▊", "▉"}

type ConfigJson struct {
	BOT_TOKEN              string `json:"bot_token"`
	SUDO_USERS             []int  `json:"sudo_users"`
	OWNER_ID               int    `json:"owner_id"`
	DOWNLOAD_DIR           string `json:"download_dir"`
	IS_TEAM_DRIVE          bool   `json:"is_team_drive"`
	GDRIVE_PARENT_ID       string `json:"gdrive_parent_id"`
	STATUS_UPDATE_INTERVAL int    `json:"status_update_interval"`
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

func GetGDriveParentId() string {
	if Config.GDRIVE_PARENT_ID == "" {
		return "root"
	}
	return Config.GDRIVE_PARENT_ID
}

func IsTeamDrive() bool {
	return Config.IS_TEAM_DRIVE
}

func GetStatusUpdateInterval() time.Duration {
	return time.Duration(Config.STATUS_UPDATE_INTERVAL) * time.Second
}

func GetHumanBytes(b int64) string {
	const unit = 1000
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
	fileName, err := GetFileNameLink(link)
	if err != nil {
		if strings.HasSuffix(link, ".torrent") {
			return false, nil
		}
		if strings.Contains(err.Error(), "no media") {
			return false, nil
		}
		return false, err
	}
	if strings.HasSuffix(fileName, ".torrent") {
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
	for i := 0; i <= cFull; i++ {
		outStr += pStr
	}
	if cPart >= 0 {
		outStr += PROGRESS_INCOMPLETE[cPart]
	}
	for i := 0; i <= PROGRESS_MAX_SIZE-cFull; i++ {
		outStr += sStr
	}
	return fmt.Sprintf("[%s]", outStr)
}
