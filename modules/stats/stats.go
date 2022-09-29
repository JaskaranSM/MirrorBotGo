package stats

import (
	"MirrorBotGo/db"
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/ricochet2200/go-disk-usage/du"
	"go.uber.org/zap"
)

var startTime time.Time = time.Now()

func GetMemoryUsage() string {
	out := ""
	memoryStat, err := mem.VirtualMemory()
	if err != nil {
		return "NA"
	}
	out += fmt.Sprintf("%.2f", memoryStat.UsedPercent) + "%"
	return out
}

func GetCpuUsage() string {
	out := ""
	data, err := cpu.Percent(time.Second, false)
	if err != nil {
		return "NA"
	}
	out += fmt.Sprintf("%.2f", data[0]) + "%"
	return out
}

func GetMemoryStats() string {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	outStr := ""
	outStr += fmt.Sprintf("Alloc: %s\n", utils.GetHumanBytes(int64(mem.Alloc)))
	outStr += fmt.Sprintf("TotalAlloc: %s\n", utils.GetHumanBytes(int64(mem.TotalAlloc)))
	outStr += fmt.Sprintf("HeapAlloc: %s\n", utils.GetHumanBytes(int64(mem.HeapAlloc)))
	outStr += fmt.Sprintf("NumGC: %d", mem.NumGC)
	return outStr
}

func ProfileHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
	return nil
}

func StatsHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	message := ctx.EffectiveMessage
	out := ""
	uptime := time.Now().Sub(startTime)
	diskStats := du.NewDiskUsage("/")
	out += fmt.Sprintf("BotUptime: %s\n", utils.HumanizeDuration(uptime))
	out += fmt.Sprintf("MirrorsRunning: %d\n", engine.GetAllMirrorsCount())
	out += fmt.Sprintf("Total: %s\n", utils.GetHumanBytes(int64(diskStats.Size())))
	out += fmt.Sprintf("Used: %s\n", utils.GetHumanBytes(int64(diskStats.Used())))
	out += fmt.Sprintf("Free: %s\n", utils.GetHumanBytes(int64(diskStats.Free())))
	out += fmt.Sprintf("CPU: %s\n", GetCpuUsage())
	out += fmt.Sprintf("RAM: %s\n", GetMemoryUsage())
	out += fmt.Sprintf("Cores: %d\n", runtime.NumCPU())
	out += fmt.Sprintf("Goroutines: %d\n", runtime.NumGoroutine())
	sysStats := GetMemoryStats()
	out += sysStats
	engine.SendMessage(b, out, message)
	return nil
}

func LoadStatsHandler(updater *ext.Updater, l *zap.SugaredLogger) {
	defer l.Info("Stats Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("stats", StatsHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("profile", ProfileHandler))
}
