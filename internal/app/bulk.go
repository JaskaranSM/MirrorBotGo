package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gotd/botapi"

	"mirrorbot/internal/mirror"
	"mirrorbot/internal/sources/bulktg"
	"mirrorbot/internal/tgbot"
	"mirrorbot/internal/util"
)

const (
	bulkListPageSize = 10
	// bulkPromptDebounce collects rapid forwards before refreshing the prompt,
	// so bursts of files produce one prompt update instead of one per file.
	bulkPromptDebounce = 3500 * time.Millisecond
)

// bulkItem is one queued file (the user's forwarded message id + its media).
type bulkItem struct {
	msgID int
	media tgbot.ReplyMedia
}

// bulkSession is the per-chat "file listener" state.
type bulkSession struct {
	chatID      int64
	userID      int64
	cmdMsgID    int
	folderName  string
	items       []bulkItem
	promptMsgID int
	listMsgID   int
	listPage    int

	mu          sync.Mutex
	promptTimer *time.Timer
}

func (s *bulkSession) stopPromptTimer() {
	s.mu.Lock()
	if s.promptTimer != nil {
		s.promptTimer.Stop()
		s.promptTimer = nil
	}
	s.mu.Unlock()
}

// bulkManager tracks one listening session per chat.
type bulkManager struct {
	mu       sync.Mutex
	sessions map[int64]*bulkSession
}

func newBulkManager() *bulkManager {
	return &bulkManager{sessions: make(map[int64]*bulkSession)}
}

func (m *bulkManager) start(chatID, userID int64, cmdMsgID int, folder string) *bulkSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := &bulkSession{chatID: chatID, userID: userID, cmdMsgID: cmdMsgID, folderName: folder}
	m.sessions[chatID] = s
	return s
}

func (m *bulkManager) get(chatID int64) *bulkSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[chatID]
}

func (m *bulkManager) add(chatID int64, item bulkItem) *bulkSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[chatID]
	if s == nil {
		return nil
	}
	s.items = append(s.items, item)
	return s
}

func (m *bulkManager) deleteItem(chatID int64, msgID int) (*bulkSession, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[chatID]
	if s == nil {
		return nil, false
	}
	for i, it := range s.items {
		if it.msgID == msgID {
			s.items = append(s.items[:i], s.items[i+1:]...)
			return s, true
		}
	}
	return s, false
}

func (m *bulkManager) remove(chatID int64) *bulkSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[chatID]
	delete(m.sessions, chatID)
	return s
}

// --- commands ---

func (a *App) cmdBulkStart(c *botapi.Context) error {
	if !a.authorized(c, c.Message()) {
		return nil
	}
	msg := c.Message()
	folder := util.ParseMessageArgs(msg.Text)
	sess := a.bulk.start(msg.Chat.ID, effectiveUserID(msg), msg.MessageID, folder)
	a.refreshBulkPrompt(a.deps.Ctx, sess)
	return nil
}

func (a *App) cmdBulkList(c *botapi.Context) error {
	if !a.authorized(c, c.Message()) {
		return nil
	}
	sess := a.bulk.get(c.Message().Chat.ID)
	if sess == nil {
		a.reply(c, "No active bulk session. Start one with /bulktgmirror")
		return nil
	}
	// Keep a single control message: the list (which carries Finish/Cancel)
	// supersedes the standalone prompt, and a previous list is replaced.
	sess.stopPromptTimer()
	if sess.promptMsgID != 0 {
		_ = a.bot.Delete(a.deps.Ctx, sess.chatID, sess.promptMsgID)
		sess.promptMsgID = 0
	}
	if sess.listMsgID != 0 {
		_ = a.bot.Delete(a.deps.Ctx, sess.chatID, sess.listMsgID)
		sess.listMsgID = 0
	}
	text, markup := a.renderBulkList(sess, 0)
	m, err := a.bot.SendHTML(a.deps.Ctx, sess.chatID, text, c.Message().MessageID, markup)
	if err == nil && m != nil {
		sess.listMsgID = m.MessageID
		sess.listPage = 0
	}
	return nil
}

func (a *App) cmdBulkCancel(c *botapi.Context) error {
	if !a.authorized(c, c.Message()) {
		return nil
	}
	sess := a.bulk.remove(c.Message().Chat.ID)
	if sess == nil {
		a.reply(c, "No active bulk session.")
		return nil
	}
	a.cleanupBulkMessages(a.deps.Ctx, sess)
	a.reply(c, "Bulk mirror listening cancelled.")
	return nil
}

// onBulkMessage is the catch-all (registered last): it handles /deletebulktg_<id>
// taps. Forwarded media is captured by the raw interceptor (captureBulkMedia),
// which sees messages botapi would drop during conversion.
func (a *App) onBulkMessage(c *botapi.Context) error {
	msg := c.Message()
	if msg == nil {
		return nil
	}
	if strings.HasPrefix(msg.Text, "/deletebulktg_") {
		if !a.authorized(c, msg) {
			return nil
		}
		return a.handleDeleteBulk(c, msg.Text)
	}
	return nil
}

// captureBulkMedia is the raw-update interceptor: if a bulk session is active in
// the chat, it queues the forwarded media and returns true to consume the
// message. It runs before botapi's conversion, so it captures forwards whose
// origin peer botapi cannot resolve.
func (a *App) captureBulkMedia(chatID int64, msgID int, media *tgbot.ReplyMedia) bool {
	if a.bulk.get(chatID) == nil {
		return false
	}
	s := a.bulk.add(chatID, bulkItem{msgID: msgID, media: *media})
	if s == nil {
		return false
	}
	a.scheduleBulkPrompt(s)
	return true
}

// scheduleBulkPrompt debounces prompt refreshes so a burst of forwarded files
// produces a single "Finish listening" prompt after the burst settles.
func (a *App) scheduleBulkPrompt(sess *bulkSession) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if sess.promptTimer != nil {
		sess.promptTimer.Stop()
	}
	sess.promptTimer = time.AfterFunc(bulkPromptDebounce, func() {
		a.refreshBulkPrompt(a.deps.Ctx, sess)
	})
}

func (a *App) handleDeleteBulk(c *botapi.Context, text string) error {
	idStr := strings.TrimPrefix(text, "/deletebulktg_")
	if f := strings.Fields(idStr); len(f) > 0 {
		idStr = f[0]
	}
	idStr = strings.SplitN(idStr, "@", 2)[0]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		a.reply(c, "Invalid delete command.")
		return nil
	}
	sess, removed := a.bulk.deleteItem(c.Message().Chat.ID, id)
	if sess == nil {
		a.reply(c, "No active bulk session.")
		return nil
	}
	if !removed {
		a.reply(c, "That file is not in the queue.")
		return nil
	}
	a.reply(c, "Removed from queue.")
	if sess.listMsgID != 0 {
		a.editBulkList(a.deps.Ctx, sess, sess.listPage)
	}
	return nil
}

// --- callbacks ---

func (a *App) cbBulkFinish(c *botapi.Context) error {
	_ = c.AnswerCallback()
	if cq := c.Update.CallbackQuery; cq != nil {
		if chatID := callbackChatID(cq.Data); chatID != 0 {
			a.finishBulk(a.deps.Ctx, chatID)
		}
	}
	return nil
}

func (a *App) cbBulkCancel(c *botapi.Context) error {
	_ = c.AnswerCallback()
	cq := c.Update.CallbackQuery
	if cq == nil {
		return nil
	}
	chatID := callbackChatID(cq.Data)
	sess := a.bulk.remove(chatID)
	if sess == nil {
		return nil
	}
	a.cleanupBulkMessages(a.deps.Ctx, sess)
	_, _ = a.bot.SendHTML(a.deps.Ctx, sess.chatID, "Bulk mirror listening cancelled.", 0, nil)
	return nil
}

func (a *App) cbBulkList(c *botapi.Context) error {
	_ = c.AnswerCallback()
	cq := c.Update.CallbackQuery
	if cq == nil {
		return nil
	}
	// data is "bulklist:<chatID>:<page>"
	chatID := callbackChatID(cq.Data)
	page := 0
	if parts := strings.SplitN(cq.Data, ":", 3); len(parts) >= 3 {
		page, _ = strconv.Atoi(parts[2])
	}
	sess := a.bulk.get(chatID)
	if sess == nil {
		return nil
	}
	a.editBulkList(a.deps.Ctx, sess, page)
	return nil
}

// --- finish: start the combined mirror ---

func (a *App) finishBulk(ctx context.Context, chatID int64) {
	sess := a.bulk.remove(chatID)
	if sess == nil {
		return
	}
	a.cleanupBulkMessages(ctx, sess)
	if len(sess.items) == 0 {
		_, _ = a.bot.SendHTML(ctx, chatID, "Bulk mirror queue is empty; nothing to do.", sess.cmdMsgID, nil)
		return
	}

	folder := displayFolder(sess)
	uid := int64(sess.cmdMsgID)
	baseDir := filepath.Join(a.cfg.DownloadDir, strconv.FormatInt(uid, 10), sanitizeFolder(folder))
	listener := mirror.NewMirrorListener(a.deps, uid, chatID, sess.cmdMsgID, false, false, false, "")

	items := make([]bulktg.Item, 0, len(sess.items))
	for _, it := range sess.items {
		items = append(items, bulktg.Item{Location: it.media.Location, FileName: it.media.FileName, Size: it.media.Size})
	}
	src := bulktg.New(a.bot, items, folder, baseDir, util.RandGID(16), listener)
	src.SetIndex(a.mgr.NextIndex())
	a.mgr.Set(uid, src)
	a.status.SendStatus(ctx, chatID, 0, 0)
	a.status.Start()
	go src.Start(ctx)
}

// --- rendering ---

func (a *App) refreshBulkPrompt(ctx context.Context, sess *bulkSession) {
	if sess.promptMsgID != 0 {
		_ = a.bot.Delete(ctx, sess.chatID, sess.promptMsgID)
		sess.promptMsgID = 0
	}
	text := fmt.Sprintf("📥 Bulk Telegram mirror — folder: <b>%s</b>\n%d file(s) queued. Forward more files, /bulktglist to review, then tap below.",
		escapeHTML(displayFolder(sess)), len(sess.items))
	m, err := a.bot.SendHTML(ctx, sess.chatID, text, 0, bulkActionButtons(sess.chatID))
	if err == nil && m != nil {
		sess.promptMsgID = m.MessageID
	}
}

func (a *App) renderBulkList(sess *bulkSession, page int) (string, botapi.ReplyMarkup) {
	n := len(sess.items)
	pages := (n + bulkListPageSize - 1) / bulkListPageSize
	if pages < 1 {
		pages = 1
	}
	if page < 0 {
		page = 0
	}
	if page > pages-1 {
		page = pages - 1
	}
	start := page * bulkListPageSize
	end := start + bulkListPageSize
	if end > n {
		end = n
	}

	var b strings.Builder
	fmt.Fprintf(&b, "📂 Bulk queue — folder: <b>%s</b>\nPage %d/%d · %d file(s)\n\n", escapeHTML(displayFolder(sess)), page+1, pages, n)
	if n == 0 {
		b.WriteString("(queue is empty)\n")
	}
	for i := start; i < end; i++ {
		it := sess.items[i]
		fmt.Fprintf(&b, "%d. %s\n/deletebulktg_%d\n\n", i+1, escapeHTML(it.media.FileName), it.msgID)
	}

	cid := strconv.FormatInt(sess.chatID, 10)
	var rows [][]botapi.InlineKeyboardButton
	if pages > 1 {
		var nav []botapi.InlineKeyboardButton
		if page > 0 {
			nav = append(nav, botapi.InlineButtonData("◄ Prev", "bulklist:"+cid+":"+strconv.Itoa(page-1)))
		}
		if page < pages-1 {
			nav = append(nav, botapi.InlineButtonData("Next ►", "bulklist:"+cid+":"+strconv.Itoa(page+1)))
		}
		rows = append(rows, nav)
	}
	rows = append(rows, []botapi.InlineKeyboardButton{
		botapi.InlineButtonData("✅ Finish listening", "bulkfinish:"+cid),
		botapi.InlineButtonData("❌ Cancel", "bulkcancel:"+cid),
	})
	return b.String(), &botapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func (a *App) editBulkList(ctx context.Context, sess *bulkSession, page int) {
	if sess.listMsgID == 0 {
		return
	}
	text, markup := a.renderBulkList(sess, page)
	sess.listPage = page
	_ = a.bot.EditHTML(ctx, sess.chatID, sess.listMsgID, text, markup)
}

func (a *App) cleanupBulkMessages(ctx context.Context, sess *bulkSession) {
	sess.stopPromptTimer()
	if sess.promptMsgID != 0 {
		_ = a.bot.Delete(ctx, sess.chatID, sess.promptMsgID)
	}
	if sess.listMsgID != 0 {
		_ = a.bot.Delete(ctx, sess.chatID, sess.listMsgID)
	}
}

// bulkActionButtons is the Finish/Cancel row shown on the prompt. The chat id is
// encoded into the callback data because botapi does not populate
// CallbackQuery.Message.
func bulkActionButtons(chatID int64) *botapi.InlineKeyboardMarkup {
	cid := strconv.FormatInt(chatID, 10)
	return botapi.InlineKeyboard([]botapi.InlineKeyboardButton{
		botapi.InlineButtonData("✅ Finish listening", "bulkfinish:"+cid),
		botapi.InlineButtonData("❌ Cancel", "bulkcancel:"+cid),
	})
}

// callbackChatID extracts the chat id encoded as the second ":"-separated field
// of callback data (e.g. "bulkfinish:123" or "first:123" or "bulklist:123:2").
func callbackChatID(data string) int64 {
	parts := strings.SplitN(data, ":", 3)
	if len(parts) >= 2 {
		return util.ParseInt64(parts[1])
	}
	return 0
}

func displayFolder(sess *bulkSession) string {
	if sess.folderName != "" {
		return sess.folderName
	}
	if len(sess.items) > 0 {
		return fmt.Sprintf("%d...%d", sess.items[0].msgID, sess.items[len(sess.items)-1].msgID)
	}
	return "(auto)"
}

// sanitizeFolder neutralizes path separators and traversal names so the folder
// is a single directory component. Dots within a name (e.g. "13547...13549")
// are preserved.
func sanitizeFolder(name string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", "\x00", "")
	out := strings.TrimSpace(r.Replace(name))
	if out == "" || out == "." || out == ".." {
		return "bulk"
	}
	return out
}

