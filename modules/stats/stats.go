package stats

import (
	"MirrorBotGo/db"
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/mackerelio/go-osstat/cpu"
	"github.com/mackerelio/go-osstat/memory"

	"github.com/PaulSonOfLars/gotgbot"
	"github.com/PaulSonOfLars/gotgbot/ext"
	"github.com/PaulSonOfLars/gotgbot/handlers"
	"github.com/ricochet2200/go-disk-usage/du"
	"go.uber.org/zap"
)

var startTime time.Time = time.Now()

func GetMemoryUsage() string {
	out := ""
	memory, err := memory.Get()
	if err != nil {
		log.Println(err)
		return out
	}
	out += fmt.Sprintf("%d%%", memory.Free*100/memory.Total)
	return out
}

func GetCpuUsage() string {
	out := ""
	before, err := cpu.Get()
	if err != nil {
		log.Println(err)
		return out
	}
	time.Sleep(time.Duration(1) * time.Second)
	after, err := cpu.Get()
	if err != nil {
		log.Println(err)
		return out
	}
	total := after.User - before.User
	out += fmt.Sprintf("%d%%", total)
	return out
}

func StatsHandler(b ext.Bot, u *gotgbot.Update) error {
	if !db.IsAuthorized(u.EffectiveMessage) {
		return nil
	}
	message := u.EffectiveMessage
	out := ""
	uptime := time.Now().Sub(startTime)
	diskStats := du.NewDiskUsage("/")
	out += fmt.Sprintf("BotUptime: %s\n", utils.HumanizeDuration(uptime))
	out += fmt.Sprintf("Total: %s\n", utils.GetHumanBytes(int64(diskStats.Size())))
	out += fmt.Sprintf("Used: %s\n", utils.GetHumanBytes(int64(diskStats.Used())))
	out += fmt.Sprintf("Free: %s\n", utils.GetHumanBytes(int64(diskStats.Free())))
	out += fmt.Sprintf("CPU: %s\n", GetCpuUsage())
	out += fmt.Sprintf("RAM: %s\n", GetMemoryUsage())
	out += fmt.Sprintf("Cores: %d\n", runtime.NumCPU())
	out += fmt.Sprintf("Goroutines: %d", runtime.NumGoroutine())
	engine.SendMessage(b, out, message)
	return nil
}

func LoadStatsHandler(updater *gotgbot.Updater, l *zap.SugaredLogger) {
	defer l.Info("Stats Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("stats", StatsHandler))
}
