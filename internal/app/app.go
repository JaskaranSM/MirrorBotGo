// Package app wires the subsystems together and implements the Telegram command
// handlers.
package app

import (
	"context"
	"strings"
	"time"

	"github.com/gotd/botapi"

	"mirrorbot/internal/config"
	"mirrorbot/internal/gdrive"
	"mirrorbot/internal/mirror"
	"mirrorbot/internal/sources/torrentdl"
	"mirrorbot/internal/store"
	"mirrorbot/internal/tgbot"
	"mirrorbot/internal/util"
)

// App holds all wired subsystems.
type App struct {
	cfg       *config.Config
	bot       *tgbot.Bot
	store     *store.Store
	mgr       *mirror.Manager
	status    *mirror.StatusService
	drive     *gdrive.Client
	torrents  *torrentdl.Engine
	deps      *mirror.Deps
	bulk      *bulkManager
	startTime time.Time
	logFile   string
}

// New constructs the App and all subsystems. ctx is the long-lived root context.
func New(ctx context.Context, cfg *config.Config, bot *tgbot.Bot, st *store.Store, logFile string) (*App, error) {
	drive, err := gdrive.New(gdrive.Config{
		UseSA:           cfg.UseSA,
		SADir:           cfg.SADir,
		CredentialsFile: cfg.GDriveCredentials,
		TokenFile:       cfg.GDriveTokenFile,
	}, cfg.GDriveConcurrency)
	if err != nil {
		return nil, err
	}

	engine, err := torrentdl.NewEngine(torrentdl.EngineConfig{
		DownloadDir:                cfg.DownloadDir,
		ListenPort:                 cfg.TorrentListenPort,
		PeerIDPrefix:               cfg.TorrentPeerIDPrefix,
		MaxUploadRate:              cfg.TorrentMaxUploadRate,
		EstablishedConnsPerTorrent: cfg.TorrentEstablishedConnsPerTorrent,
		UseTrackerList:             cfg.TorrentUseTrackerList,
		TrackerListURL:             cfg.TorrentTrackerListURL,
	})
	if err != nil {
		return nil, err
	}

	mgr := mirror.NewManager()
	statusSvc := mirror.NewStatusService(bot, mgr, cfg.StatusUpdateInterval, cfg.StatusMessagesPerPage, cfg.StatusMessageAutoDeleteTime)
	statusSvc.SetRootContext(ctx)

	deps := &mirror.Deps{
		Ctx:             ctx,
		Bot:             bot,
		Mgr:             mgr,
		Status:          statusSvc,
		Drive:           drive,
		DownloadDir:     cfg.DownloadDir,
		DefaultParentID: cfg.GDriveParentID,
		IndexURL:        cfg.IndexURL,
	}

	return &App{
		cfg:       cfg,
		bot:       bot,
		store:     st,
		mgr:       mgr,
		status:    statusSvc,
		drive:     drive,
		torrents:  engine,
		deps:      deps,
		bulk:      newBulkManager(),
		startTime: time.Now(),
		logFile:   logFile,
	}, nil
}

// Run starts serving updates (blocks until ctx is cancelled).
func (a *App) Run(ctx context.Context) error { return a.bot.Run(ctx) }

// Close releases subsystem resources (the torrent client).
func (a *App) Close() {
	if a.torrents != nil {
		a.torrents.Close()
	}
}

// CancelAll cancels every active and seeding mirror (used on shutdown).
func (a *App) CancelAll() {
	for _, s := range a.mgr.All() {
		s.Cancel()
	}
	for _, s := range a.mgr.AllSeeding() {
		s.Cancel()
	}
}

// Register installs all command and callback handlers on the bot.
func (a *App) Register() {
	api := a.bot.API()

	api.OnCommand("start", "Start the bot", a.cmdStart)

	api.OnCommand("mirror", "Mirror a link to Google Drive", a.handler(func(c *botapi.Context) error {
		a.prepareMirror(c, false, false, true, false)
		return nil
	}))
	api.OnCommand("mirrors", "Mirror silently", a.handler(func(c *botapi.Context) error {
		a.prepareMirror(c, false, false, false, false)
		return nil
	}))
	api.OnCommand("tarmirror", "Mirror then archive", a.handler(func(c *botapi.Context) error {
		a.prepareMirror(c, true, false, true, false)
		return nil
	}))
	api.OnCommand("tarmirrors", "Mirror then archive silently", a.handler(func(c *botapi.Context) error {
		a.prepareMirror(c, true, false, false, false)
		return nil
	}))
	api.OnCommand("unarchmirror", "Mirror then extract", a.handler(func(c *botapi.Context) error {
		a.prepareMirror(c, false, true, true, false)
		return nil
	}))
	api.OnCommand("unarchmirrors", "Mirror then extract silently", a.handler(func(c *botapi.Context) error {
		a.prepareMirror(c, false, true, false, false)
		return nil
	}))
	api.OnCommand("seedtorrent", "Mirror torrent with seeding", a.handler(func(c *botapi.Context) error {
		a.prepareMirror(c, false, false, true, true)
		return nil
	}))
	api.OnCommand("seedtorrents", "Mirror torrent with seeding silently", a.handler(func(c *botapi.Context) error {
		a.prepareMirror(c, false, false, false, true)
		return nil
	}))

	api.OnCommand("clone", "Clone a Google Drive link", a.handler(func(c *botapi.Context) error {
		a.cmdClone(c, true)
		return nil
	}))
	api.OnCommand("clones", "Clone silently", a.handler(func(c *botapi.Context) error {
		a.cmdClone(c, false)
		return nil
	}))

	api.OnCommand("status", "Show mirror status", a.cmdStatus)
	api.OnCallbackQuery(a.cbStatus("first"), botapi.CallbackPrefix("first"))
	api.OnCallbackQuery(a.cbStatus("previous"), botapi.CallbackPrefix("previous"))
	api.OnCallbackQuery(a.cbStatus("next"), botapi.CallbackPrefix("next"))
	api.OnCallbackQuery(a.cbStatus("last"), botapi.CallbackPrefix("last"))

	api.OnCommand("cancel", "Cancel a mirror", a.cmdCancel)
	api.OnCommand("cancelall", "Cancel all mirrors", a.cmdCancelAll)
	api.OnCommand("cid", "Cancel by index", a.cmdCancelByIndex)

	api.OnCommand("list", "Search Google Drive", a.cmdList)
	api.OnCommand("stats", "Bot and system stats", a.cmdStats)
	api.OnCommand("ping", "Ping the bot", a.cmdPing)
	api.OnCommand("sh", "Run a shell command (owner)", a.cmdShell)
	api.OnCommand("log", "Send the log file (owner)", a.cmdLog)

	api.OnCommand("adduser", "Authorize a user (owner)", a.cmdAddUser)
	api.OnCommand("rmuser", "De-authorize a user (owner)", a.cmdRemoveUser)
	api.OnCommand("addchat", "Authorize a chat (owner)", a.cmdAddChat)
	api.OnCommand("rmchat", "De-authorize a chat (owner)", a.cmdRemoveChat)

	api.OnCommand("setgotdthreads", "Set Telegram download threads (owner)", a.cmdSetGotdThreads)
	api.OnCommand("getgotdthreads", "Get Telegram download threads (owner)", a.cmdGetGotdThreads)
	api.OnCommand("mirrormsg", "Inspect a mirror by gid (owner)", a.cmdMirrorMsg)

	// Bulk Telegram mirror: forward many files, then mirror them as one folder.
	api.OnCommand("bulktgmirror", "Start a bulk Telegram file listener", a.cmdBulkStart)
	api.OnCommand("bulktglist", "List files queued in the bulk session", a.cmdBulkList)
	api.OnCommand("cancelbulktgmirror", "Cancel the bulk Telegram listener", a.cmdBulkCancel)
	api.OnCallbackQuery(a.cbBulkFinish, botapi.CallbackPrefix("bulkfinish"))
	api.OnCallbackQuery(a.cbBulkCancel, botapi.CallbackPrefix("bulkcancel"))
	api.OnCallbackQuery(a.cbBulkList, botapi.CallbackPrefix("bulklist:"))

	// Catch-all message handler MUST be registered last: route() dispatches to
	// the first matching handler, so all OnCommand routes take precedence. This
	// handles /deletebulktg_<id> taps.
	api.OnMessage(a.onBulkMessage)

	// Raw interceptor for forwarded media: botapi drops messages whose forward
	// origin cannot be resolved, so capture bulk files from the raw update
	// before botapi's conversion. Must run after botapi installed its handlers
	// (i.e. after New), which it has by the time Register is called.
	a.bot.InterceptMedia(a.captureBulkMedia)
}

// handler wraps a handler with an authorization gate.
func (a *App) handler(h botapi.Handler) botapi.Handler {
	return func(c *botapi.Context) error {
		if !a.authorized(c, c.Message()) {
			return nil
		}
		return h(c)
	}
}

func (a *App) authorized(ctx context.Context, msg *botapi.Message) bool {
	if msg == nil {
		return false
	}
	uid := effectiveUserID(msg)
	cid := msg.Chat.ID
	if a.cfg.IsOwner(uid) {
		return true
	}
	for _, c := range a.cfg.AuthorizedChats {
		if c == cid {
			return true
		}
	}
	if uid != 0 {
		if ok, _ := a.store.IsUserAuthorized(ctx, uid); ok {
			return true
		}
	}
	if ok, _ := a.store.IsChatAuthorized(ctx, cid); ok {
		return true
	}
	return false
}

func (a *App) isOwner(msg *botapi.Message) bool {
	return msg != nil && a.cfg.IsOwner(effectiveUserID(msg))
}

// effectiveUserID resolves the sender's user id. In private chats Telegram omits
// from_id (it equals the peer), so botapi leaves Message.From nil and the user
// id is the chat id; fall back to that.
func effectiveUserID(msg *botapi.Message) int64 {
	if msg == nil {
		return 0
	}
	if msg.From != nil {
		return msg.From.ID
	}
	return msg.Chat.ID
}

// splitParent splits "link | drive-folder-link" into (link, parentFolderID).
func splitParent(arg string) (string, string) {
	if !strings.Contains(arg, "|") {
		return strings.TrimSpace(arg), ""
	}
	parts := strings.SplitN(arg, "|", 2)
	link := strings.TrimSpace(parts[0])
	parent := util.GetFileIDByGDriveLink(strings.TrimSpace(parts[1]))
	return link, parent
}
