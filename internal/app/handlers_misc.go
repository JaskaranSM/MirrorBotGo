package app

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gotd/botapi"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"

	"mirrorbot/internal/util"
)

func (a *App) cmdPing(c *botapi.Context) error {
	if !a.authorized(c, c.Message()) {
		return nil
	}
	start := time.Now()
	m, err := c.Reply("Starting ping")
	if err != nil {
		return err
	}
	elapsed := time.Since(start).Milliseconds()
	return a.bot.EditHTML(a.deps.Ctx, c.Message().Chat.ID, m.MessageID, fmt.Sprintf("Pong %d ms", elapsed), nil)
}

func (a *App) cmdStats(c *botapi.Context) error {
	if !a.authorized(c, c.Message()) {
		return nil
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	uptime := time.Since(a.startTime)

	out := fmt.Sprintf("BotUptime: %s\n", util.HumanizeDuration(uptime))
	out += fmt.Sprintf("MirrorsRunning: %d\n", a.mgr.Count())
	if du, err := disk.Usage(a.cfg.DownloadDir); err == nil {
		out += fmt.Sprintf("Total: %s\n", util.GetHumanBytes(int64(du.Total)))
		out += fmt.Sprintf("Used: %s\n", util.GetHumanBytes(int64(du.Used)))
		out += fmt.Sprintf("Free: %s\n", util.GetHumanBytes(int64(du.Free)))
	}
	if pct, err := cpu.Percent(10*time.Millisecond, false); err == nil && len(pct) > 0 {
		out += fmt.Sprintf("CPU: %.2f%%\n", pct[0])
	}
	out += fmt.Sprintf("RAM: %s\n", util.GetHumanBytes(int64(mem.Alloc)))
	out += fmt.Sprintf("Cores: %d\n", runtime.NumCPU())
	out += fmt.Sprintf("Goroutines: %d\n", runtime.NumGoroutine())
	a.reply(c, out)
	return nil
}

func (a *App) cmdList(c *botapi.Context) error {
	if !a.authorized(c, c.Message()) {
		return nil
	}
	name := util.ParseMessageArgs(c.Message().Text)
	if name == "" {
		a.reply(c, "Provide a search query.")
		return nil
	}
	files, err := a.drive.List(a.deps.Ctx, a.cfg.GDriveParentID, name, 20)
	if err != nil {
		a.reply(c, "List failed: "+err.Error())
		return nil
	}
	if len(files) == 0 {
		a.reply(c, "No results.")
		return nil
	}
	var b strings.Builder
	for _, f := range files {
		link := "https://drive.google.com/open?id=" + f.Id
		if f.MimeType == "application/vnd.google-apps.folder" {
			fmt.Fprintf(&b, "⁍ <a href='%s'>%s</a> (folder)\n", link, f.Name)
		} else {
			fmt.Fprintf(&b, "⁍ <a href='%s'>%s</a> (%s)\n", link, f.Name, util.GetHumanBytes(f.Size))
		}
	}
	_, _ = a.bot.SendHTML(a.deps.Ctx, c.Message().Chat.ID, b.String(), c.Message().MessageID, nil)
	return nil
}

func (a *App) cmdShell(c *botapi.Context) error {
	if !a.isOwner(c.Message()) {
		return nil
	}
	cmdText := util.ParseMessageArgs(c.Message().Text)
	if cmdText == "" {
		a.reply(c, "Provide proper arguments")
		return nil
	}
	w := &shellWriter{}
	m, err := c.Reply("...")
	if err != nil {
		return err
	}
	chatID := c.Message().Chat.ID
	cmd := exec.CommandContext(a.deps.Ctx, "bash", "-c", "stdbuf -o0 "+cmdText)
	cmd.Stdout = w
	cmd.Stderr = w
	go func() { _ = cmd.Run(); w.markDone() }()
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		last := ""
		for range ticker.C {
			text := w.tail(3800)
			if text != last {
				_ = a.bot.EditHTML(a.deps.Ctx, chatID, m.MessageID, escapeHTML(text), nil)
				last = text
			}
			if w.isDone() {
				return
			}
		}
	}()
	return nil
}

func (a *App) cmdLog(c *botapi.Context) error {
	if !a.isOwner(c.Message()) {
		return nil
	}
	if a.logFile == "" {
		a.reply(c, "No log file configured.")
		return nil
	}
	_, err := a.bot.SendDocument(a.deps.Ctx, c.Message().Chat.ID, a.logFile, "", c.Message().MessageID)
	if err != nil {
		a.reply(c, "Failed to send log: "+err.Error())
	}
	return nil
}

// --- authorization management (owner) ---

func (a *App) cmdAddUser(c *botapi.Context) error {
	if !a.isOwner(c.Message()) {
		return nil
	}
	id := extractUserID(c.Message())
	if id == 0 {
		a.reply(c, "Provide a proper userId.")
		return nil
	}
	if err := a.store.AddAuthorizedUser(a.deps.Ctx, id); err != nil {
		a.reply(c, "Error authorizing user: "+err.Error())
		return nil
	}
	a.reply(c, "Authorized user.")
	return nil
}

func (a *App) cmdRemoveUser(c *botapi.Context) error {
	if !a.isOwner(c.Message()) {
		return nil
	}
	id := extractUserID(c.Message())
	if id == 0 {
		a.reply(c, "Provide a proper userId.")
		return nil
	}
	if err := a.store.RemoveAuthorizedUser(a.deps.Ctx, id); err != nil {
		a.reply(c, "Error de-authorizing user: "+err.Error())
		return nil
	}
	a.reply(c, "De-authorized user.")
	return nil
}

func (a *App) cmdAddChat(c *botapi.Context) error {
	if !a.isOwner(c.Message()) {
		return nil
	}
	id := extractChatID(c.Message())
	if id == 0 {
		a.reply(c, "Provide a proper chatId.")
		return nil
	}
	if err := a.store.AddAuthorizedChat(a.deps.Ctx, id); err != nil {
		a.reply(c, "Error authorizing chat: "+err.Error())
		return nil
	}
	a.reply(c, "Authorized chat.")
	return nil
}

func (a *App) cmdRemoveChat(c *botapi.Context) error {
	if !a.isOwner(c.Message()) {
		return nil
	}
	id := extractChatID(c.Message())
	if id == 0 {
		a.reply(c, "Provide a proper chatId.")
		return nil
	}
	if err := a.store.RemoveAuthorizedChat(a.deps.Ctx, id); err != nil {
		a.reply(c, "Error de-authorizing chat: "+err.Error())
		return nil
	}
	a.reply(c, "De-authorized chat.")
	return nil
}

// --- configuration (owner) ---

func (a *App) cmdSetGotdThreads(c *botapi.Context) error {
	if !a.isOwner(c.Message()) {
		return nil
	}
	n, err := strconv.Atoi(util.ParseMessageArgs(c.Message().Text))
	if err != nil || n <= 0 {
		a.reply(c, "Provide a positive integer.")
		return nil
	}
	if err := a.store.SetSetting(a.deps.Ctx, "gotd_threads", strconv.Itoa(n)); err != nil {
		a.reply(c, "Error: "+err.Error())
		return nil
	}
	a.reply(c, fmt.Sprintf("Set Telegram download threads to %d.", n))
	return nil
}

func (a *App) cmdGetGotdThreads(c *botapi.Context) error {
	if !a.isOwner(c.Message()) {
		return nil
	}
	v, _ := a.store.GetSetting(a.deps.Ctx, "gotd_threads")
	if v == "" {
		v = "default"
	}
	a.reply(c, "Telegram download threads: "+v)
	return nil
}

func (a *App) cmdMirrorMsg(c *botapi.Context) error {
	if !a.isOwner(c.Message()) {
		return nil
	}
	gid := util.ParseMessageArgs(c.Message().Text)
	if gid == "" {
		a.reply(c, "Provide a gid.")
		return nil
	}
	dl := a.mgr.ByGID(gid)
	if dl == nil {
		a.reply(c, "No mirror with that gid.")
		return nil
	}
	a.reply(c, fmt.Sprintf("Name: %s\nStatus: %s\nIndex: %d\nGID: %s",
		dl.Name(), dl.StatusType(), dl.Index(), dl.GID()))
	return nil
}

// --- helpers ---

func extractUserID(msg *botapi.Message) int64 {
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil {
		return msg.ReplyToMessage.From.ID
	}
	return util.ParseInt64(util.ParseMessageArgs(msg.Text))
}

func extractChatID(msg *botapi.Message) int64 {
	if arg := util.ParseMessageArgs(msg.Text); arg != "" {
		return util.ParseInt64(arg)
	}
	return msg.Chat.ID
}

func escapeHTML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

// shellWriter accumulates command output for live editing.
type shellWriter struct {
	mu   sync.Mutex
	buf  []byte
	done bool
}

func (w *shellWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	w.buf = append(w.buf, p...)
	w.mu.Unlock()
	return len(p), nil
}

func (w *shellWriter) tail(n int) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buf) == 0 {
		return "No output"
	}
	if len(w.buf) > n {
		return string(w.buf[len(w.buf)-n:])
	}
	return string(w.buf)
}

func (w *shellWriter) markDone() { w.mu.Lock(); w.done = true; w.mu.Unlock() }
func (w *shellWriter) isDone() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.done
}
