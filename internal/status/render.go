package status

import (
	"fmt"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"

	"mirrorbot/internal/util"
)

var threadProfile = pprof.Lookup("threadcreate")

// cpuUsage returns the current CPU usage as a "NN.NN%" string, matching the original.
func cpuUsage() string {
	data, err := cpu.Percent(10*time.Millisecond, false)
	if err != nil || len(data) == 0 {
		return "NA"
	}
	return fmt.Sprintf("%.2f", data[0]) + "%"
}

// StatsFooter returns the runtime stats line ("Alloc: ... | CPU: ...").
func StatsFooter() string {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	var out string
	out += fmt.Sprintf("Alloc: %s | ", util.GetHumanBytes(int64(mem.Alloc)))
	out += fmt.Sprintf("TAlloc: %s | ", util.GetHumanBytes(int64(mem.TotalAlloc)))
	out += fmt.Sprintf("GC: %d | ", mem.NumGC)
	out += fmt.Sprintf("GR: %d | ", runtime.NumGoroutine())
	threads := 0
	if threadProfile != nil {
		threads = threadProfile.Count()
	}
	out += fmt.Sprintf("TH: %d | ", threads)
	out += fmt.Sprintf("CPU: %s", cpuUsage())
	return out
}

// RenderProgress renders the status message for one page of statuses, exactly
// matching the original GetReadableProgressMessage output. footer is appended
// after the per-unit lines (pass StatsFooter()); it is separated out so tests
// can supply a deterministic value.
func RenderProgress(dls []Status, footer string) string {
	var globalDownloadSpeed int64
	var globalUploadSpeed int64
	msg := ""
	for i := 0; i <= len(dls)-1; i++ {
		dl := dls[i]
		msg += fmt.Sprintf("<i>%s</i> -", dl.Name())
		msg += fmt.Sprintf(" %s\n", dl.StatusType())
		if dl.StatusType() == Downloading {
			globalDownloadSpeed += dl.Speed()
		}
		if dl.StatusType() == Seeding || dl.StatusType() == Uploading {
			globalUploadSpeed += dl.Speed()
		}
		if dl.StatusType() == Cloning {
			msg += fmt.Sprintf("%s of ", util.GetHumanBytes(dl.CompletedLength()))
			msg += fmt.Sprintf("%s at ", util.GetHumanBytes(dl.TotalLength()))
			msg += fmt.Sprintf("%s/s, ", util.GetHumanBytes(dl.Speed()))
			msg += fmt.Sprintf("\nGID: <code>%s</code> ", dl.GID())
			msg += fmt.Sprintf("I: <code>%d</code>", dl.Index())
			msg += "\n\n"
			continue
		}
		msg += fmt.Sprintf("<code>%s %.2f%% </code>", util.GetProgressBarString(int(dl.CompletedLength()), int(dl.TotalLength())), dl.Percentage())
		msg += fmt.Sprintf(", %s of ", util.GetHumanBytes(dl.CompletedLength()))
		msg += fmt.Sprintf("%s at ", util.GetHumanBytes(dl.TotalLength()))
		msg += fmt.Sprintf("%s/s, ", util.GetHumanBytes(dl.Speed()))
		if dl.ETA() != nil {
			if dl.StatusType() == Seeding && dl.CompletedLength() > dl.TotalLength() {
				msg += fmt.Sprintf("ST: %s", util.HumanizeDuration(*dl.ETA()))
			} else {
				msg += fmt.Sprintf("ETA: %s", dl.ETA())
			}
		} else {
			msg += "ETA: -"
		}
		if ti, ok := dl.(TorrentInfo); ok {
			msg += fmt.Sprintf(" | P: %d | S: %d | PC: %d/%d", ti.Peers(), ti.Seeders(), ti.PiecesCompleted(), ti.PiecesTotal())
		}
		msg += fmt.Sprintf("\nGID: <code>%s</code> ", dl.GID())
		msg += fmt.Sprintf("I: <code>%d</code>", dl.Index())
		msg += "\n\n"
	}
	msg += footer
	msg += fmt.Sprintf(" | DL: %s", util.GetHumanBytes(globalDownloadSpeed))
	msg += fmt.Sprintf(" | UP: %s", util.GetHumanBytes(globalUploadSpeed))
	return msg
}
