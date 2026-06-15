# mirrorbot

A single-binary Telegram **mirror bot**: send it a link (torrent/magnet, HTTP,
Google Drive, or a Telegram file) and it downloads the content, optionally
archives/extracts it, uploads it to Google Drive, and replies with a shareable
link — showing a live, continuously-edited progress message the whole time.

This is a ground-up Go rewrite of the original multi-process *MirrorBotGo*
(which split work across `kedge`, `transfer-service`, a MEGA REST service, NZBGet
and MongoDB). Everything is now embedded in one process:

| Concern            | Implementation                                   |
|--------------------|--------------------------------------------------|
| Telegram           | [`gotd/botapi`](https://github.com/gotd/botapi) (MTProto; large-file downloads built in) |
| Torrents           | [`anacrolix/torrent`](https://github.com/anacrolix/torrent) (embedded engine) |
| Google Drive       | [`google.golang.org/api/drive/v3`](https://pkg.go.dev/google.golang.org/api/drive/v3) |
| Database           | `database/sql` — **SQLite** (`modernc.org/sqlite`, pure Go) or **PostgreSQL** (`pgx`) |
| Config             | environment variables (docker-compose friendly)  |

## Architecture

```
cmd/mirrorbot         entrypoint: config, logging, signals, wiring
internal/config       ENV loader + validation
internal/store        SQL store (sqlite|postgres): auth lists + settings
internal/status       DownloadStatus interface + exact status-message renderer
internal/mirror       Manager (state), MirrorListener/CloneListener (pipeline), StatusService (edit loop)
internal/tgbot        botapi wrapper: send/edit/delete (retry+flood-wait), MTProto media fetch/download
internal/sources/*    torrentdl, httpdl, tgfile, bulktg download sources
internal/gdrive       embedded Drive upload/download/clone/list/metadata
internal/archive      tar create + archive extraction (.zip/.tar/.tar.gz)
internal/health       /health + /healthcount
```

Every unit of work (a download, an archive step, a Drive upload/clone) implements
one `status.Status` interface; the renderer and the status-message edit loop read
only that. A mirror flows: **Initializing → Downloading → (Archiving|UnArchiving)
→ Uploading → link reply**, and seeding torrents move to a seeding view after upload.

## Commands

| Command | Who | What |
|---|---|---|
| `/start` | authorized | greeting |
| `/mirror <link>` `/mirrors` | authorized | mirror a link (`s` = silent, no status msg). `link \| <drive folder link>` overrides destination. Reply to a Telegram file/photo/video/audio/sticker/`.torrent` to mirror it. |
| `/tarmirror(s)` | authorized | mirror then archive to `.tar` before upload |
| `/unarchmirror(s)` | authorized | mirror then extract before upload |
| `/seedtorrent(s)` | authorized | torrent mirror, keep seeding after upload |
| `/bulktgmirror [folder]` | authorized | start a bulk Telegram listener; forward many files, then mirror them as one folder (see below) |
| `/bulktglist` | authorized | list queued bulk files (paginated) with tap-to-delete; carries Finish/Cancel |
| `/deletebulktg_<msgid>` | authorized | remove one file from the bulk queue (tap from `/bulktglist`) |
| `/cancelbulktgmirror` | authorized | cancel the bulk listener |
| `/clone <drive link>` `/clones` | authorized | server-side Drive→Drive copy; `src \| dest` supported |
| `/status` | authorized | show/refresh the live status message (with pagination buttons) |
| `/cancel <gid>` (or reply) | authorized | cancel one mirror |
| `/cancelall` `/cid <index>` | owner | cancel all / cancel by index |
| `/list <query>` | authorized | search the default Drive folder |
| `/stats` `/ping` | authorized | system stats / round-trip latency |
| `/sh <cmd>` `/log` | owner | run a shell command (live output) / fetch the log file |
| `/adduser` `/rmuser` `/addchat` `/rmchat` | owner | manage authorization (id via arg or reply) |
| `/setgotdthreads <n>` `/getgotdthreads` | owner | Telegram download concurrency setting |
| `/mirrormsg <gid>` | owner | inspect a mirror by gid |

### Telegram media

Replying to a message with `/mirror` (or forwarding into a bulk session) works for
every media kind — documents, photos, video, GIFs/animations, stickers, audio,
voice and video notes. Files keep their original name; media without one is named
by type and MIME (e.g. `video_<id>.mp4`, `sticker_<id>.webp`, `voice_<id>.ogg`).

> Telegram media is fetched over MTProto directly, so it is not subject to the
> Bot API's 20 MB download limit. Forwarded files are captured from the raw update,
> which also works for forwards whose original sender can't be resolved.

### Bulk Telegram mirror

`/bulktgmirror [folder]` opens a per-chat **file listener**:

1. Forward any number of files to the bot. After a short burst settles (~3.5 s) the
   bot posts a single prompt with **Finish listening** / **Cancel** buttons and the
   folder name (no per-file spam).
2. `/bulktglist` shows the queue, paginated, each entry with a tappable
   `/deletebulktg_<msgid>` to remove it. The list also carries Finish/Cancel.
3. **Finish listening** downloads every queued file into one folder, shown as a
   single entry in `/status`, then uploads that folder to Drive — so it lands as one
   folder you can rename. Without a `folder` argument the folder is named
   `<firstMsgId>...<lastMsgId>`.
4. **Cancel** (button or `/cancelbulktgmirror`) discards the listener.

## Configuration

All configuration is via environment variables — see [`.env.example`](.env.example)
for the full list with defaults. Required: `BOT_TOKEN`, `TG_APP_ID`, `TG_APP_HASH`.

Google Drive auth is either **service accounts** (`USE_SA=true`, drop `*.json`
keys into `SA_DIR`) or **OAuth** (`USE_SA=false`, provide `credentials.json`; on
first run the bot prints an auth URL and reads the pasted code from stdin, caching
`token.json`). Set `GDRIVE_PARENT_ID` to your destination folder (a Shared Drive
id works).

Database is `DB_DRIVER=sqlite` (default, file at `DB_DSN`) or `DB_DRIVER=postgres`
with a `postgres://…` DSN.

## Running with Docker

```bash
cp .env.example .env       # fill in BOT_TOKEN, TG_APP_ID, TG_APP_HASH, GDRIVE_PARENT_ID
mkdir -p data downloads accounts
# put service-account JSON keys into ./accounts  (USE_SA=true)
docker compose up -d --build
```

The container exposes `/health` on `:7870` for the compose healthcheck. To use
PostgreSQL, uncomment the `db` service in `docker-compose.yml` and set
`DB_DRIVER`/`DB_DSN` in `.env`.

## Running locally

```bash
go build -o mirrorbot ./cmd/mirrorbot
set -a; source .env; set +a
./mirrorbot
```

(SQLite + the torrent engine are pure Go, so `CGO_ENABLED=0` builds a static binary.)

## Tests

```bash
go test ./...
```

The status renderer has byte-exact golden tests (`internal/status`) and the SQL
store has a round-trip test against in-memory SQLite (`internal/store`).

## Not yet ported (v1 scope)

MEGA, Usenet/NZB, and the DDL JavaScript extractor engine from the original are
intentionally deferred. The source-selection seam in
`internal/app/handlers_mirror.go` and the `status.Listener` interface make adding
them straightforward later.
