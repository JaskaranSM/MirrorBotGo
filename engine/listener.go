package engine

import (
	"MirrorBotGo/utils"
	"fmt"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type MirrorListener struct {
	Update         *ext.Context
	bot            *gotgbot.Bot
	isTar          bool
	isSeed         bool
	isTorrent      bool
	doUnArchive    bool
	parentId       string
	customParentId bool
	isCanceled     bool
}

func (m *MirrorListener) GetUid() int64 {
	return m.Update.EffectiveMessage.MessageId
}

func (m *MirrorListener) GetDownload() MirrorStatus {
	return GetMirrorByUid(m.GetUid())
}

func (m *MirrorListener) OnDownloadStart(text string) {
	L().Infof("Initiated Download: %s | %s | %d | %s | %s ", m.Update.Message.From.FirstName, m.Update.Message.From.Username, m.Update.Message.From.Id, text, m.Update.Message.Text)
	UpdateAllMessages(m.bot)
}

func (m *MirrorListener) Clean() {
	MoveMirrorToCancel(m.GetUid(), GetMirrorByUid(m.GetUid()))
	RemoveMirrorLocal(m.GetUid())
	if GetAllMirrorsCount()+GetAllSeedingMirrorsCount() == 0 {
		DeleteAllMessages(m.bot)
	}
	UpdateAllMessages(m.bot)
	runtime.GC()
}

func (m *MirrorListener) OnDownloadComplete() {
	dl := m.GetDownload()
	name := dl.Name()
	size := dl.TotalLength()
	p := dl.Path()
	L().Infof("[DownloadComplete]: %s (%d)", name, size)
	if m.isSeed {
		MoveMirrorToSeeding(m.GetUid(), m.GetDownload())
	}
	if m.isTar {
		archiver := NewTarArchiver(dl.TotalLength())
		tarStatus := NewTarStatus(dl.Gid(), dl.Name(), nil, archiver)
		tarStatus.Index_ = dl.Index()
		AddMirrorLocal(m.GetUid(), tarStatus)
		var err error
		p, err = archiver.TarPath(p)
		if err != nil {
			L().Errorf("Failed to archive the contents, uploading as it is: %s: %v", p, err)
			SendMessage(m.bot, fmt.Sprintf("Failed to archive the contents, uploading as it is: %s\nERR: %s\nGid: <code>%s</code>", dl.Name(), err.Error(), dl.Gid()), m.Update.Message)
		}
	}
	if m.doUnArchive {
		unarchiver := NewUnArchiver()
		totalSize, err := unarchiver.CalculateTotalSize(p)
		if err != nil {
			L().Errorf("Failed to get archive contents size, uploading as it is: %s: %v", p, err)
			SendMessage(m.bot, fmt.Sprintf("Failed to get archive contents size, uploading as it is: %s\nERR: %s\nGid: <code>%s</code>", dl.Name(), err.Error(), dl.Gid()), m.Update.Message)
		} else {
			unarchiver.SetTotal(totalSize)
			unArchiverStatus := NewUnArchiverStatus(dl.Gid(), dl.Name(), nil, unarchiver)
			unArchiverStatus.Index_ = dl.Index()
			AddMirrorLocal(m.GetUid(), unArchiverStatus)
			p, err = unarchiver.UnArchivePath(p)
			if err != nil {
				L().Errorf("Failed to unarchive the contents, uploading as it is: %s: %v", p, err)
				SendMessage(m.bot, fmt.Sprintf("Failed to unarchive the contents, uploading as it is: %s\nERR: %s\nGid: <code>%s</code>", dl.Name(), err.Error(), dl.Gid()), m.Update.Message)
			} else {
				size = totalSize
			}
		}
	}
	var parentId string
	if m.parentId != "" {
		_, err := transferServiceClient.GetFileMetadata(m.parentId)
		if err != nil {
			L().Warn("Error while checking for user supplied parentId so uploading to main parentId: ", err)
			parentId = utils.GetGDriveParentId()
		} else {
			parentId = m.parentId
			m.customParentId = true
		}
	} else {
		parentId = utils.GetGDriveParentId()
	}
	trGid, err := transferServiceClient.AddUpload(&UploadRequest{
		Path:        p,
		ParentId:    parentId,
		Concurrency: 10,
		Size:        size,
	})
	if err != nil {
		L().Error(err)
		m.OnUploadError(err.Error())
	} else {
		trListener := NewGoogleDriveTransferListener(m, nil, false, trGid)
		driveStatus := NewGoogleDriveTransferStatus(trGid, p, dl.GetListener(), nil)
		driveStatus.Index_ = dl.Index()
		AddMirrorLocal(m.GetUid(), driveStatus)
		trListener.StartListener()
		UpdateAllMessages(m.bot)
	}
}

func (m *MirrorListener) OnDownloadError(err string) {
	if m.isCanceled {
		return
	}
	m.isCanceled = true
	dl := m.GetDownload()
	if dl != nil {
		name := dl.Name()
		size := dl.TotalLength()
		L().Errorf("[DownloadError]: %s (%d)", name, size)
		m.Clean()
	}
	msg := "Your download has been stopped due to: %s"
	SendMessage(m.bot, fmt.Sprintf(msg, err), m.Update.Message)
	if dl != nil {
		m.CleanDownload()
	}
}

func (m *MirrorListener) OnUploadError(err string) {
	dl := m.GetDownload()
	name := dl.Name()
	size := dl.TotalLength()
	L().Errorf("[UploadError]: %s (%d)", name, size)
	msg := "Your upload has been stopped due to: %s"
	if m.isSeed {
		seedStatus := GetSeedingMirrorByUid(m.GetUid())
		AddMirrorLocal(m.GetUid(), seedStatus)
		RemoveMirrorSeeding(m.GetUid())
		UpdateAllMessages(m.bot)
	}
	if !m.isSeed {
		m.Clean()
	}
	SendMessage(m.bot, fmt.Sprintf(msg, err), m.Update.Message)
	if !m.isSeed {
		m.CleanDownload()
	}
}

func (m *MirrorListener) OnUploadComplete(link string) {
	dl := m.GetDownload()
	name := dl.Name()
	size := dl.TotalLength()
	L().Infof("[UploadComplete]: %s (%d)", name, size)
	link = strings.ReplaceAll(link, "'", "")
	msg := fmt.Sprintf("<a href='%s'>%s</a> (%s)", link, dl.Name(), utils.GetHumanBytes(dl.TotalLength()))
	inUrl := utils.GetIndexUrl()
	if inUrl != "" {
		if m.customParentId {
			msg += "\n\nShareable Link: Mirror belongs to a custom parentId"
		} else {
			inUrl = fmt.Sprintf("%s/%s", inUrl, name)
			if utils.IsPathDir(dl.Path()) {
				inUrl += "/"
			}
			msg += fmt.Sprintf("\n\nShareable Link: <a href='%s'>here</a>", inUrl)
		}
	}
	if m.isSeed {
		seedStatus := GetSeedingMirrorByUid(m.GetUid())
		AddMirrorLocal(m.GetUid(), seedStatus)
		RemoveMirrorSeeding(m.GetUid())
		UpdateAllMessages(m.bot)
	}
	if !m.isSeed {
		m.Clean()
	}
	SendMessage(m.bot, msg, m.Update.Message)
	if !m.isSeed {
		m.CleanDownload()
	}
}

func (m *MirrorListener) OnSeedingStart(text string) {
	L().Info(text)
}

func (m *MirrorListener) OnSeedingError(err error) {
	if m.isCanceled {
		return
	}
	m.isCanceled = true
	dl := m.GetDownload()
	name := dl.Name()
	size := dl.TotalLength()
	L().Errorf("[SeedError]: %s (%d)", name, size)
	m.Clean()
	msg := "Your seeding has been stopped due to: %s"
	SendMessage(m.bot, fmt.Sprintf(msg, err.Error()), m.Update.Message)
	m.CleanDownload()
}

func (m *MirrorListener) CleanDownload() {
	err := utils.RemoveByPath(path.Join(utils.GetDownloadDir(), utils.ParseInt64ToString(m.GetUid())))
	if err != nil {
		L().Errorf("MirrorListener: CleanDownload: RemoveByPath: %v", err)
		return
	}
}

func NewMirrorListener(b *gotgbot.Bot, update *ext.Context, isTar bool, doUnArchive bool, parentId string) MirrorListener {
	return MirrorListener{bot: b, Update: update, isTar: isTar, doUnArchive: doUnArchive, parentId: parentId}
}

type CloneListener struct {
	Update     *ext.Context
	bot        *gotgbot.Bot
	parentId   string
	isCanceled bool
}

func (m *CloneListener) GetUid() int64 {
	return m.Update.EffectiveMessage.MessageId
}

func (m *CloneListener) GetDownload() MirrorStatus {
	return GetMirrorByUid(m.GetUid())
}

func (m *CloneListener) OnCloneStart(text string) {
	L().Infof("Initiated Clone: %s | %s | %d | %s | %s ", m.Update.Message.From.FirstName, m.Update.Message.From.Username, m.Update.Message.From.Id, text, m.Update.Message.Text)
	UpdateAllMessages(m.bot)
}

func (m *CloneListener) Clean() {
	MoveMirrorToCancel(m.GetUid(), GetMirrorByUid(m.GetUid()))
	RemoveMirrorLocal(m.GetUid())
	if GetAllMirrorsCount()+GetAllSeedingMirrorsCount() == 0 {
		DeleteAllMessages(m.bot)
	}
	UpdateAllMessages(m.bot)
	runtime.GC()
}

func (m *CloneListener) OnCloneError(err string) {
	if m.isCanceled {
		return
	}
	m.isCanceled = true
	dl := m.GetDownload()
	name := dl.Name()
	size := dl.TotalLength()
	L().Infof("[onCloneError]: %s (%d) %s", name, size, err)
	m.Clean()
	msg := "Your clone has been stopped due to: %s"
	SendMessage(m.bot, fmt.Sprintf(msg, err), m.Update.Message)
}

func (m *CloneListener) OnCloneComplete(link string) {
	dl := m.GetDownload()
	name := dl.Name()
	size := dl.TotalLength()
	L().Infof("[CloneComplete]: %s (%d)", name, size)
	link = strings.ReplaceAll(link, "'", "")
	name = strings.ReplaceAll(dl.Name(), "'", "")
	msg := fmt.Sprintf("<a href='%s'>%s</a> (%s)", link, name, utils.GetHumanBytes(dl.CompletedLength()))
	inUrl := utils.GetIndexUrl()
	if inUrl != "" {
		inUrl = inUrl + "/" + name
		msg += fmt.Sprintf("\n\nShareable Link: <a href='%s'>here</a>", inUrl)
	}
	m.Clean()
	SendMessage(m.bot, msg, m.Update.Message)
}

func NewCloneListener(b *gotgbot.Bot, update *ext.Context, parentId string) CloneListener {
	return CloneListener{bot: b, Update: update, parentId: parentId}
}

func NewInitializingStatus(name string, gid string, dir string, listener *MirrorListener) *InitializingStatus {
	return &InitializingStatus{
		name:     name,
		gid:      gid,
		dir:      dir,
		listener: listener,
	}
}

type InitializingStatus struct {
	name     string
	gid      string
	listener *MirrorListener
	dir      string //we do not know the "Path()" yet
	Index_   int
}

func (i *InitializingStatus) Name() string {
	return i.name
}

func (i *InitializingStatus) CompletedLength() int64 {
	return 0
}

func (i *InitializingStatus) TotalLength() int64 {
	return 0
}

func (i *InitializingStatus) Speed() int64 {
	return 0
}

func (i *InitializingStatus) ETA() *time.Duration {
	dur := time.Duration(0)
	return &dur
}

func (i *InitializingStatus) Percentage() float32 {
	return 0.0
}

func (i *InitializingStatus) Gid() string {
	return i.gid
}

func (i *InitializingStatus) Path() string {
	return i.dir
}

func (i *InitializingStatus) GetStatusType() string {
	return MirrorStatusInitializing
}

func (i *InitializingStatus) IsTorrent() bool {
	return false
}
func (i *InitializingStatus) PiecesCompleted() int {
	return 0
}
func (i *InitializingStatus) PiecesTotal() int {
	return 0
}
func (i *InitializingStatus) GetPeers() int {
	return 0
}
func (i *InitializingStatus) GetSeeders() int {
	return 0
}
func (i *InitializingStatus) Index() int {
	return i.Index_
}
func (i *InitializingStatus) GetListener() *MirrorListener {
	return i.listener
}
func (i *InitializingStatus) GetCloneListener() *CloneListener {
	return nil
}
func (i *InitializingStatus) CancelMirror() bool {
	return false
}

type MirrorStatus interface {
	Name() string
	CompletedLength() int64
	TotalLength() int64
	Speed() int64
	ETA() *time.Duration
	Gid() string
	Path() string
	Percentage() float32
	GetStatusType() string
	IsTorrent() bool
	PiecesCompleted() int
	PiecesTotal() int
	GetPeers() int
	GetSeeders() int
	Index() int
	GetListener() *MirrorListener
	GetCloneListener() *CloneListener
	CancelMirror() bool
}

type TransferListener interface {
	OnTransferUpdate(int64, int64)
}
