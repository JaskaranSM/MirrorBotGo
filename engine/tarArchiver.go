package engine

import (
	"MirrorBotGo/utils"
	"log"
	"time"

	"github.com/mholt/archiver"
)

// I do not have method to get progress of tar archivals, so adding methods with dummy methods for MirrorStatus interface implementation.
type TarStatus struct {
	name     string
	listener *MirrorListener
	gid      string
	Index_   int
	tar      *TarArchiver
}

func (t *TarStatus) Name() string {
	return t.name
}

func (t *TarStatus) CompletedLength() int64 {
	return t.tar.Completed
}

func (t *TarStatus) TotalLength() int64 {
	return t.tar.Total
}

func (t *TarStatus) Speed() int64 {
	return t.tar.Speed
}

func (t *TarStatus) ETA() *time.Duration {
	dur := t.tar.ETA
	return &dur
}

func (t *TarStatus) Gid() string {
	return t.gid
}

func (t *TarStatus) Percentage() float32 {
	return float32(t.CompletedLength()*100) / float32(t.TotalLength())
}

func (t *TarStatus) GetStatusType() string {
	return MirrorStatusArchiving
}

func (t *TarStatus) Path() string {
	return t.Name()
}

func (t *TarStatus) Index() int {
	return t.Index_
}

func (t *TarStatus) GetListener() *MirrorListener {
	return t.listener
}

func (t *TarStatus) CancelMirror() bool {
	return false
}

func NewTarStatus(gid string, name string, listener *MirrorListener, archiver *TarArchiver) *TarStatus {
	return &TarStatus{gid: gid, name: name, listener: listener, tar: archiver}
}

//TarArchiver struct
type TarArchiver struct {
	Prg       *archiver.Progress
	Speed     int64
	StartTime time.Time
	Completed int64
	Total     int64
	ETA       time.Duration
}

func NewProgress() *archiver.Progress {
	return &archiver.Progress{}
}

//NewTarArchiver constructor
func NewTarArchiver(p *archiver.Progress, total int64) *TarArchiver {
	return &TarArchiver{Prg: p, Total: total, StartTime: time.Now()}
}

//OnTarProgress progress function
func (t *TarArchiver) OnTarProgress() {
	t.Completed = t.Prg.Get()
	if t.Completed == 0 {
		return
	}
	now := time.Now()
	diff := int64(now.Sub(t.StartTime).Seconds())
	if diff != 0 {
		t.Speed = t.Completed / diff
	} else {
		t.Speed = 0
	}
	if t.Speed != 0 {
		t.ETA = utils.CalculateETA(t.Total-t.Completed, t.Speed)
	} else {
		t.ETA = time.Duration(0)
	}
}

func (t *TarArchiver) ProgressLoop() {
	for {
		t.OnTarProgress()
		time.Sleep(1 * time.Second)
	}
}

//TarPath start tarring
func (t *TarArchiver) TarPath(path string) string {
	outPath := path + ".tar"
	log.Printf("[TarPath]: %s -> %s\n", path, outPath)
	tar := archiver.NewTar()
	tar.Prg = t.Prg
	tar.ImplicitTopLevelFolder = true
	go t.ProgressLoop()
	tar.Archive([]string{path}, outPath)
	return outPath
}
