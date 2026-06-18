package mirror

import (
	"context"
	"errors"
	"log"
	"strconv"
	"sync"
	"time"

	"mirrorbot/internal/metrics"

	"github.com/gotd/botapi"

	"mirrorbot/internal/status"
	"mirrorbot/internal/tgbot"
)

// statusView tracks the single status message shown in one chat.
type statusView struct {
	messageID int
	page      int
	lastText  string
}

// StatusService owns the per-chat status messages and the background "spinner"
// goroutine that edits them on an interval. It reproduces the original
// edit-message loop, including pagination and self-termination when idle.
type StatusService struct {
	bot           *tgbot.Bot
	mgr           *Manager
	interval      time.Duration
	perPage       int
	autoDeleteSec int

	mu      sync.Mutex
	views   map[int64]*statusView
	running bool
	rootCtx context.Context
}

// NewStatusService constructs a StatusService.
func NewStatusService(bot *tgbot.Bot, mgr *Manager, interval time.Duration, perPage, autoDeleteSec int) *StatusService {
	if perPage < 1 {
		perPage = 5
	}
	return &StatusService{
		bot:           bot,
		mgr:           mgr,
		interval:      interval,
		perPage:       perPage,
		autoDeleteSec: autoDeleteSec,
		views:         make(map[int64]*statusView),
	}
}

// SetRootContext sets the long-lived context used by the spinner loop.
func (s *StatusService) SetRootContext(ctx context.Context) { s.rootCtx = ctx }

func (s *StatusService) idle() bool {
	return s.mgr.Count()+s.mgr.SeedingCount() == 0
}

// SendStatus creates or refreshes the status message for a chat. If
// deleteCmdMsgID != 0, that (command) message is deleted. The send is rate
// limited through the bot's sender queue.
func (s *StatusService) SendStatus(ctx context.Context, chatID int64, replyTo, deleteCmdMsgID int) {
	s.bot.QueueSend(ctx, chatID, func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		// Drop any previous status message for this chat.
		if old, ok := s.views[chatID]; ok {
			delete(s.views, chatID)
			_ = s.bot.Delete(ctx, chatID, old.messageID)
		}

		body := s.renderPage(0)
		var markup botapi.ReplyMarkup
		if s.mgr.Count() > s.perPage {
			chunks := s.mgr.Chunked(s.perPage)
			markup = pagination(false, true, "0", strconv.Itoa(len(chunks)-1), chatID)
		}
		msg, err := s.bot.SendHTML(ctx, chatID, body, replyTo, markup)
		if err != nil || msg == nil {
			return
		}
		if deleteCmdMsgID != 0 {
			_ = s.bot.Delete(ctx, chatID, deleteCmdMsgID)
		}
		s.views[chatID] = &statusView{messageID: msg.MessageID, page: 0, lastText: body}

		if s.autoDeleteSec > 0 {
			mid := msg.MessageID
			time.AfterFunc(time.Duration(s.autoDeleteSec)*time.Second, func() {
				_ = s.bot.Delete(ctx, chatID, mid)
			})
		}
	})
}

// Start launches the spinner if not already running.
func (s *StatusService) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()
	go s.spin()
}

func (s *StatusService) spin() {
	defer func() {
		if r := recover(); r != nil {
			metrics.Panics.WithLabelValues("spinner").Inc()
			log.Printf("status: recovered from panic in spinner: %v", r)
			s.stop()
		}
	}()
	ctx := s.rootCtx
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		s.mu.Lock()
		s.updateAll(ctx)
		s.mu.Unlock()

		select {
		case <-ctx.Done():
			s.stop()
			return
		case <-time.After(s.interval):
		}

		if s.idle() {
			s.mu.Lock()
			s.deleteAll(ctx)
			s.mu.Unlock()
			s.stop()
			return
		}
	}
}

func (s *StatusService) stop() {
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
}

// updateAll edits every tracked status message. Caller holds s.mu.
func (s *StatusService) updateAll(ctx context.Context) {
	count := s.mgr.Count()
	for chatID, view := range s.views {
		if s.idle() {
			_ = s.bot.EditHTML(ctx, chatID, view.messageID, "No active mirrors", nil)
			continue
		}
		chunks := s.mgr.Chunked(s.perPage)
		if view.page > len(chunks)-1 {
			view.page = len(chunks) - 1
		}
		if view.page < 0 {
			view.page = 0
		}
		progress := status.RenderProgress(chunks[view.page], status.StatsFooter())
		if progress == view.lastText {
			continue
		}
		var err error
		if count > s.perPage {
			previous := view.page > 0
			next := len(chunks) > view.page+1
			markup := pagination(previous, next, strconv.Itoa(view.page), strconv.Itoa(len(chunks)-view.page-1), chatID)
			err = s.bot.EditHTML(ctx, chatID, view.messageID, progress, markup)
		} else {
			err = s.bot.EditHTML(ctx, chatID, view.messageID, progress, nil)
		}
		if errors.Is(err, tgbot.ErrMessageNotFound) {
			delete(s.views, chatID)
			continue
		}
		view.lastText = progress
	}
}

func (s *StatusService) deleteAll(ctx context.Context) {
	for chatID, view := range s.views {
		_ = s.bot.Delete(ctx, chatID, view.messageID)
		delete(s.views, chatID)
	}
}

// renderPage renders a page; caller holds s.mu.
func (s *StatusService) renderPage(page int) string {
	if s.idle() {
		return "No active mirrors"
	}
	chunks := s.mgr.Chunked(s.perPage)
	if page > len(chunks)-1 {
		page = len(chunks) - 1
	}
	if page < 0 {
		page = 0
	}
	return status.RenderProgress(chunks[page], status.StatsFooter())
}

// --- pagination callbacks (wired to the inline buttons) ---

// Page adjusts the page for a chat's status view and refreshes immediately.
// delta of 0 with absolute>=0 jumps to an absolute page; use the helpers below.
func (s *StatusService) pageJump(ctx context.Context, chatID int64, fn func(view *statusView, chunks int)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	view, ok := s.views[chatID]
	if !ok {
		return
	}
	if s.mgr.Count() > s.perPage {
		chunks := len(s.mgr.Chunked(s.perPage))
		fn(view, chunks)
	}
	s.updateAll(ctx)
}

// First jumps to the first page.
func (s *StatusService) First(ctx context.Context, chatID int64) {
	s.pageJump(ctx, chatID, func(v *statusView, _ int) { v.page = 0 })
}

// Previous goes back one page.
func (s *StatusService) Previous(ctx context.Context, chatID int64) {
	s.pageJump(ctx, chatID, func(v *statusView, _ int) { v.page-- })
}

// Next advances one page.
func (s *StatusService) Next(ctx context.Context, chatID int64) {
	s.pageJump(ctx, chatID, func(v *statusView, _ int) { v.page++ })
}

// Last jumps to the last page.
func (s *StatusService) Last(ctx context.Context, chatID int64) {
	s.pageJump(ctx, chatID, func(v *statusView, chunks int) { v.page = chunks - 1 })
}

// pagination builds the First/<=/=>/Last inline keyboard row. The chat id is
// encoded into the callback data because botapi does not populate
// CallbackQuery.Message, so the handler cannot otherwise recover the chat.
func pagination(previous, next bool, prStr, nxStr string, chatID int64) *botapi.InlineKeyboardMarkup {
	cid := strconv.FormatInt(chatID, 10)
	var row []botapi.InlineKeyboardButton
	if previous {
		row = append(row, botapi.InlineButtonData("First", "first:"+cid))
		row = append(row, botapi.InlineButtonData("<=("+prStr+")", "previous:"+cid))
	}
	if next {
		row = append(row, botapi.InlineButtonData("=>("+nxStr+")", "next:"+cid))
		row = append(row, botapi.InlineButtonData("Last", "last:"+cid))
	}
	return botapi.InlineKeyboard(row)
}
