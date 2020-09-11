package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"time"
)

const ConfigJsonPath string = "config.json"

type ConfigJson struct {
	BOT_TOKEN    string `json:"bot_token"`
	SUDO_USERS   []int  `json:"sudo_users"`
	OWNER_ID     int    `json:"owner_id"`
	DOWNLOAD_DIR string `json:"download_dir"`
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

func GetDownloadDir() string {
	return Config.DOWNLOAD_DIR
}

func GetSleepTime() time.Duration {
	return 5 * time.Second
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
