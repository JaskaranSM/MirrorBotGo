package tgbot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"reflect"
	"unsafe"

	"github.com/gotd/botapi"
	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/tg"
)

// ReplyMedia describes a downloadable media object extracted from a message.
type ReplyMedia struct {
	Location tg.InputFileLocationClass
	FileName string
	Size     int64
	MIME     string // document mime type (empty for photos)
}

// FetchReplyMedia fetches a message by id over MTProto and returns its media
// (document or photo) as a downloadable location. botapi delivers a reply as
// only the replied-message id, so the content must be fetched separately.
func (b *Bot) FetchReplyMedia(ctx context.Context, chatID int64, msgID int) (*ReplyMedia, error) {
	ref, err := b.api.PeerRef(ctx, botapi.ID(chatID))
	if err != nil {
		return nil, err
	}
	ids := []tg.InputMessageClass{&tg.InputMessageID{ID: msgID}}

	var res tg.MessagesMessagesClass
	if ref.Kind == "channel" {
		res, err = b.api.Raw().ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
			Channel: &tg.InputChannel{ChannelID: ref.ID, AccessHash: ref.AccessHash},
			ID:      ids,
		})
	} else {
		res, err = b.api.Raw().MessagesGetMessages(ctx, ids)
	}
	if err != nil {
		return nil, err
	}

	mod, ok := res.AsModified()
	if !ok {
		return nil, errors.New("no messages returned")
	}
	for _, mc := range mod.GetMessages() {
		m, ok := mc.(*tg.Message)
		if !ok {
			continue
		}
		media, ok := m.GetMedia()
		if !ok {
			continue
		}
		if rm := mediaFromTg(media); rm != nil {
			return rm, nil
		}
	}
	return nil, errors.New("replied message has no downloadable media")
}

// mediaFromTg converts a raw message media object into a downloadable ReplyMedia
// (documents of any kind, and photos). Returns nil for unsupported media.
func mediaFromTg(media tg.MessageMediaClass) *ReplyMedia {
	switch mm := media.(type) {
	case *tg.MessageMediaDocument:
		if doc, ok := mm.Document.(*tg.Document); ok {
			return &ReplyMedia{
				Location: doc.AsInputDocumentFileLocation(""),
				FileName: documentFilename(doc),
				Size:     doc.Size,
				MIME:     doc.MimeType,
			}
		}
	case *tg.MessageMediaPhoto:
		if photo, ok := mm.Photo.(*tg.Photo); ok {
			thumb, size := largestPhotoSize(photo)
			return &ReplyMedia{
				Location: photo.AsInputPhotoFileLocation(thumb),
				FileName: fmt.Sprintf("photo_%d.jpg", photo.ID),
				Size:     size,
			}
		}
	}
	return nil
}

// InterceptMedia chains a raw UpdateNewMessage handler ahead of botapi's so it
// can capture media messages that botapi would otherwise drop during conversion
// (e.g. forwards whose origin peer cannot be resolved). fn receives the chat id,
// message id and extracted media; returning true consumes the message so botapi
// does not also process it. Non-media messages and unconsumed ones fall through
// to botapi's normal routing.
func (b *Bot) InterceptMedia(fn func(chatID int64, msgID int, media *ReplyMedia) bool) {
	disp := b.api.Dispatcher()
	orig := existingNewMessageHandler(disp)
	if orig == nil {
		// We could not capture botapi's existing handler to chain. Replacing it
		// would route nothing for non-media messages and silently break all
		// commands, so leave botapi's handler untouched and skip the interceptor.
		// (Pinned to gotd/botapi v0.2.0 so this path is not normally reached; see
		// go.mod.) The only loss is capturing forwards with unresolvable origins.
		log.Printf("tgbot: media interceptor disabled (could not chain update handler)")
		return
	}
	disp.OnNewMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewMessage) error {
		if m, ok := u.Message.(*tg.Message); ok {
			if media, ok := m.GetMedia(); ok {
				if rm := mediaFromTg(media); rm != nil {
					if chatID := peerToChatID(m.PeerID); chatID != 0 && fn(chatID, m.ID, rm) {
						return nil // consumed
					}
				}
			}
		}
		return orig(ctx, e, u)
	})
}

// existingNewMessageHandler reads botapi's already-registered UpdateNewMessage
// handler so InterceptMedia can chain rather than replace it. botapi v0.2.0 has
// no public accessor, so the unexported dispatcher map is read reflectively;
// failure is non-fatal (the chain simply has no downstream).
func existingNewMessageHandler(disp *tg.UpdateDispatcher) (h tg.Handler) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("tgbot: could not read existing update handler to chain: %v", r)
			h = nil
		}
	}()
	field := reflect.ValueOf(disp).Elem().FieldByName("handlers")
	field = reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
	handlers, ok := field.Interface().(map[uint32]tg.Handler)
	if !ok {
		return nil
	}
	return handlers[tg.UpdateNewMessageTypeID]
}

// peerToChatID converts a raw peer into the Bot API chat id convention so it
// matches the chat ids used elsewhere (sessions, etc.).
func peerToChatID(p tg.PeerClass) int64 {
	switch v := p.(type) {
	case *tg.PeerUser:
		return v.UserID
	case *tg.PeerChat:
		return -v.ChatID
	case *tg.PeerChannel:
		return -1000000000000 - v.ChannelID
	}
	return 0
}

// documentFilename returns a usable filename for any document type. Telegram
// videos, GIFs, stickers, audio and voice notes usually lack an explicit
// filename attribute, so synthesize "<kind>_<id>.<ext>" from the document's
// type attributes and MIME type.
func documentFilename(doc *tg.Document) string {
	for _, a := range doc.Attributes {
		if fn, ok := a.(*tg.DocumentAttributeFilename); ok && fn.FileName != "" {
			return fn.FileName
		}
	}

	var isVideo, isAnimated, isSticker, isAudio, isVoice bool
	for _, a := range doc.Attributes {
		switch v := a.(type) {
		case *tg.DocumentAttributeVideo:
			isVideo = true
		case *tg.DocumentAttributeAnimated:
			isAnimated = true
		case *tg.DocumentAttributeSticker:
			isSticker = true
		case *tg.DocumentAttributeAudio:
			isAudio = true
			isVoice = v.Voice
		}
	}

	kind := "file"
	switch {
	case isSticker:
		kind = "sticker"
	case isAnimated:
		kind = "animation"
	case isVoice:
		kind = "voice"
	case isAudio:
		kind = "audio"
	case isVideo:
		kind = "video"
	}
	return fmt.Sprintf("%s_%d%s", kind, doc.ID, extForMIME(doc.MimeType))
}

// extForMIME maps a MIME type to a file extension, preferring well-known
// Telegram media types and falling back to the stdlib mime database.
func extForMIME(mimeType string) string {
	switch mimeType {
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "application/x-tgsticker":
		return ".tgs"
	case "audio/ogg":
		return ".ogg"
	case "audio/mpeg":
		return ".mp3"
	case "audio/flac":
		return ".flac"
	}
	if exts, err := mime.ExtensionsByType(mimeType); err == nil && len(exts) > 0 {
		return exts[0]
	}
	return ".bin"
}

// largestPhotoSize picks the highest-resolution size variant and its byte size.
func largestPhotoSize(p *tg.Photo) (thumbType string, size int64) {
	best := int64(-1)
	for _, s := range p.Sizes {
		var sz int64
		var t string
		switch v := s.(type) {
		case *tg.PhotoSize:
			sz, t = int64(v.Size), v.Type
		case *tg.PhotoSizeProgressive:
			t = v.Type
			for _, n := range v.Sizes {
				if int64(n) > sz {
					sz = int64(n)
				}
			}
		case *tg.PhotoCachedSize:
			sz, t = int64(len(v.Bytes)), v.Type
		default:
			continue
		}
		if sz > best {
			best, thumbType, size = sz, t, sz
		}
	}
	if thumbType == "" && len(p.Sizes) > 0 {
		thumbType = p.Sizes[len(p.Sizes)-1].GetType()
	}
	return thumbType, size
}

// DownloadMedia streams a Telegram media location to w over MTProto.
func (b *Bot) DownloadMedia(ctx context.Context, loc tg.InputFileLocationClass, w io.Writer) error {
	_, err := downloader.NewDownloader().Download(b.api.Raw(), loc).Stream(ctx, w)
	return err
}
