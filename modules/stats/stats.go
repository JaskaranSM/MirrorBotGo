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

func GetMemoryStats() string {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	outStr := ""
	outStr += fmt.Sprintf("Alloc: %s\n", utils.GetHumanBytes(int64(memStats.Alloc)))
	outStr += fmt.Sprintf("TotalAlloc: %s\n", utils.GetHumanBytes(int64(memStats.TotalAlloc)))
	outStr += fmt.Sprintf("HeapAlloc: %s\n", utils.GetHumanBytes(int64(memStats.HeapAlloc)))
	outStr += fmt.Sprintf("NumGC: %d", memStats.NumGC)
	return outStr
}

func ProfileHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsUserOwner(ctx.EffectiveMessage.From.Id) {
		return nil
	}
	err := pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
	if err != nil {
		engine.L().Errorf("ProfileHandler: pprof.WriteTo: %v", err)
		return err
	}
	return nil
}

func TorrentStatsHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsUserOwner(ctx.EffectiveMessage.From.Id) {
		return nil
	}
	message := ctx.EffectiveMessage
	out := engine.GetAnacrolixTorrentClientStatus()
	_, err := b.SendDocument(message.Chat.Id, gotgbot.NamedFile{
		FileName: "torrentstats.txt",
		File:     &out,
	}, nil)
	if err != nil {
		engine.L().Error(err)
	}
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
	out += fmt.Sprintf("CPU: %s\n", engine.GetCpuUsage())
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
	updater.Dispatcher.AddHandler(handlers.NewCommand("torrentstats", TorrentStatsHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("profile", ProfileHandler))
}
