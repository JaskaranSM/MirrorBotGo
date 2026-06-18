// Package tgbot wraps github.com/gotd/botapi with resilient send/edit/delete
// helpers (bounded retries, flood-wait handling, a per-chat sender rate limiter)
// and a token-scrubbing filter, mirroring the original bot's message transport.
package tgbot

import (
	"context"
	"errors"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gotd/botapi"
	"github.com/gotd/log/logzap"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"mirrorbot/internal/metrics"
)

const (
	maxRetries      = 5
	sleepMultiplier = 1.5
	maxSleep        = 120 * time.Second
)

// ErrMessageNotFound is returned by edit/delete when the target message no
// longer exists, so callers can drop it from their tracking.
var ErrMessageNotFound = errors.New("tgbot: message not found")

// HTML is the HTML parse mode option shorthand.
var HTML = botapi.WithParseMode(botapi.ParseModeHTML)

// Bot wraps a botapi.Bot with transport helpers.
type Bot struct {
	api   *botapi.Bot
	token string
	queue *senderQueue
}

// New constructs a Bot. msgsPerWindow/window configure the per-chat send rate.
// If debug is true, the underlying MTProto client logs verbosely to stderr.
func New(token string, appID int, appHash string, msgsPerWindow, windowSeconds int, debug bool) (*Bot, error) {
	opts := botapi.Options{
		AppID:     appID,
		AppHash:   appHash,
		FloodWait: true,
		OnStart: func(ctx context.Context) {
			log.Println("[telegram] bot authorized and update gap recovery live")
		},
	}
	if debug {
		zl, err := zap.NewDevelopment()
		if err == nil {
			opts.Logger = logzap.New(zl)
		}
	}
	api, err := botapi.New(token, opts)
	if err != nil {
		return nil, err
	}
	if msgsPerWindow < 1 {
		msgsPerWindow = 1
	}
	if windowSeconds < 1 {
		windowSeconds = 1
	}
	return &Bot{
		api:   api,
		token: token,
		queue: newSenderQueue(msgsPerWindow, time.Duration(windowSeconds)*time.Second),
	}, nil
}

// API returns the underlying botapi.Bot for handler registration and raw calls.
func (b *Bot) API() *botapi.Bot { return b.api }

// Run connects and serves updates until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) error { return b.api.Run(ctx) }

// scrub removes the bot token from outgoing text.
func (b *Bot) scrub(text string) string {
	if b.token == "" {
		return text
	}
	return strings.ReplaceAll(text, b.token, "***")
}

// SendHTML sends an HTML message, optionally as a reply and with reply markup,
// with bounded retries and flood-wait handling.
func (b *Bot) SendHTML(ctx context.Context, chatID int64, text string, replyTo int, markup botapi.ReplyMarkup) (*botapi.Message, error) {
	opts := []botapi.SendOption{HTML}
	if replyTo != 0 {
		opts = append(opts, botapi.ReplyTo(replyTo))
	}
	if markup != nil {
		opts = append(opts, botapi.WithReplyMarkup(markup))
	}
	var msg *botapi.Message
	err := b.withRetry(ctx, "send", func() error {
		m, err := b.api.SendMessage(ctx, botapi.ID(chatID), b.scrub(text), opts...)
		if err == nil {
			msg = m
		}
		return err
	})
	return msg, err
}

// EditHTML edits a message's text (and markup if non-nil). "Not modified" is
// treated as success; a missing message returns ErrMessageNotFound.
func (b *Bot) EditHTML(ctx context.Context, chatID int64, messageID int, text string, markup botapi.ReplyMarkup) error {
	opts := []botapi.SendOption{HTML}
	if markup != nil {
		opts = append(opts, botapi.WithReplyMarkup(markup))
	}
	return b.withRetry(ctx, "edit", func() error {
		_, err := b.api.EditMessageText(ctx, botapi.ID(chatID), messageID, b.scrub(text), opts...)
		return err
	})
}

// Delete removes a message, ignoring "not found".
func (b *Bot) Delete(ctx context.Context, chatID int64, messageID int) error {
	err := b.withRetry(ctx, "delete", func() error {
		return b.api.DeleteMessage(ctx, botapi.ID(chatID), messageID)
	})
	if errors.Is(err, ErrMessageNotFound) {
		return nil
	}
	return err
}

// SendDocument uploads a local file as a document.
func (b *Bot) SendDocument(ctx context.Context, chatID int64, path, caption string, replyTo int) (*botapi.Message, error) {
	opts := []botapi.SendOption{}
	if replyTo != 0 {
		opts = append(opts, botapi.ReplyTo(replyTo))
	}
	msg, err := b.api.SendDocument(ctx, botapi.ID(chatID), botapi.FileFromPath(path), caption, opts...)
	metrics.TelegramOps.WithLabelValues("document", result(err)).Inc()
	return msg, err
}

// DownloadToPath downloads a Telegram file to a local path.
func (b *Bot) DownloadToPath(ctx context.Context, fileID, path string) error {
	return b.api.DownloadFileToPath(ctx, fileID, path)
}

// DownloadToWriter streams a Telegram file into w (used for progress tracking).
func (b *Bot) DownloadToWriter(ctx context.Context, fileID string, w io.Writer) (int64, error) {
	n, err := b.api.DownloadFile(ctx, fileID, w)
	metrics.TelegramOps.WithLabelValues("download", result(err)).Inc()
	return n, err
}

// AnswerCallback acknowledges a callback query (clears the loading spinner).
func (b *Bot) AnswerCallback(ctx context.Context, callbackID string) error {
	err := b.api.AnswerCallbackQuery(ctx, callbackID)
	metrics.TelegramOps.WithLabelValues("answer_callback", result(err)).Inc()
	return err
}

// result maps an error to the ok/error result label.
func result(err error) string {
	if err == nil {
		return metrics.ResultOK
	}
	return metrics.ResultError
}

// QueueSend runs fn through the per-chat rate limiter (used for status message
// creation so concurrent commands don't flood a chat).
func (b *Bot) QueueSend(ctx context.Context, chatID int64, fn func()) {
	b.queue.run(ctx, chatID, fn)
}

// withRetry runs op with bounded retries, flood-wait sleeps, and tolerant
// handling of "not modified" / "not found" edit errors. opName labels the
// emitted Prometheus metrics.
func (b *Bot) withRetry(ctx context.Context, opName string, op func() error) error {
	var lastErr error
	for retries := 1; retries <= maxRetries; retries++ {
		if retries > 1 {
			metrics.TelegramRetries.Inc()
		}
		err := op()
		if err == nil {
			metrics.TelegramOps.WithLabelValues(opName, metrics.ResultOK).Inc()
			return nil
		}
		if isNotModified(err) {
			metrics.TelegramOps.WithLabelValues(opName, metrics.ResultOK).Inc()
			return nil
		}
		if isNotFound(err) {
			metrics.TelegramOps.WithLabelValues(opName, "notfound").Inc()
			return ErrMessageNotFound
		}
		lastErr = err
		var sleep time.Duration
		if d, ok := botapi.AsFloodWait(err); ok {
			metrics.TelegramFloodWaits.Inc()
			sleep = d
		} else {
			sleep = time.Duration(sleepMultiplier*float32(retries)) * time.Second
		}
		if sleep >= maxSleep {
			break
		}
		select {
		case <-ctx.Done():
			metrics.TelegramOps.WithLabelValues(opName, metrics.ResultError).Inc()
			return ctx.Err()
		case <-time.After(sleep):
		}
	}
	metrics.TelegramOps.WithLabelValues(opName, metrics.ResultError).Inc()
	return lastErr
}

func isNotModified(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "not modified")
}

func isNotFound(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "message to edit not found") ||
		strings.Contains(msg, "message to delete not found") ||
		strings.Contains(msg, "message_id_invalid") ||
		strings.Contains(msg, "message not found")
}

// senderQueue enforces a per-chat send rate using token-bucket limiters.
type senderQueue struct {
	mu       sync.Mutex
	limiters map[int64]*rate.Limiter
	msgs     int
	window   time.Duration
}

func newSenderQueue(msgs int, window time.Duration) *senderQueue {
	return &senderQueue{limiters: make(map[int64]*rate.Limiter), msgs: msgs, window: window}
}

func (q *senderQueue) limiter(chatID int64) *rate.Limiter {
	q.mu.Lock()
	defer q.mu.Unlock()
	l, ok := q.limiters[chatID]
	if !ok {
		l = rate.NewLimiter(rate.Limit(float64(q.msgs)/q.window.Seconds()), q.msgs)
		q.limiters[chatID] = l
	}
	return l
}

func (q *senderQueue) run(ctx context.Context, chatID int64, fn func()) {
	l := q.limiter(chatID)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				metrics.Panics.WithLabelValues("queued_send").Inc()
				log.Printf("tgbot: recovered from panic in queued send: %v", r)
			}
		}()
		if err := l.Wait(ctx); err != nil {
			return
		}
		fn()
	}()
}
