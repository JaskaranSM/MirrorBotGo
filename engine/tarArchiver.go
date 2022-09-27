package engine

import (
	"MirrorBotGo/utils"
	"context"
	"io"
	"os"
	"time"

	"github.com/mholt/archiver/v4"
)

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

func (t *TarStatus) IsTorrent() bool {
	return false
}

func (t *TarStatus) GetPeers() int {
	return 0
}

func (t *TarStatus) GetSeeders() int {
	return 0
}

func (t *TarStatus) GetListener() *MirrorListener {
	return t.listener
}

func (t *TarStatus) GetCloneListener() *CloneListener {
	return nil
}

func (t *TarStatus) CancelMirror() bool {
	return false
}

func NewTarStatus(gid string, name string, listener *MirrorListener, archiver *TarArchiver) *TarStatus {
	return &TarStatus{gid: gid, name: name, listener: listener, tar: archiver}
}

//TarArchiver struct
type TarArchiver struct {
	Speed     int64
	StartTime time.Time
	Completed int64
	isDone    bool
	Total     int64
	ETA       time.Duration
}

//NewTarArchiver constructor
func NewTarArchiver(total int64) *TarArchiver {
	return &TarArchiver{Total: total, StartTime: time.Now()}
}

func (t *TarArchiver) OnTransferUpdate(completed int64, total int64) {
	t.Completed = completed
	t.Total = total
	if completed == 0 {
		return
	}
	now := time.Now()
	diff := int64(now.Sub(t.StartTime).Seconds())
	if diff != 0 {
		t.Speed = completed / diff
	} else {
		t.Speed = 0
	}
	if t.Speed != 0 {
		t.ETA = utils.CalculateETA(total-completed, t.Speed)
	} else {
		t.ETA = time.Duration(0)
	}
}

func (t *TarArchiver) Write(b []byte) (int, error) {
	length := len(b)
	completed := t.Completed + int64(length)
	t.OnTransferUpdate(completed, t.Total)
	return length, nil
}

//TarPath start tarring
func (t *TarArchiver) TarPath(path string) string {
	outPath := path + ".tar"
	L().Infof("[TarPath]: %s -> %s", path, outPath)
	tar := archiver.Tar{}
	writer, err := os.Create(outPath)
	if err != nil {
		L().Errorf("[TarPath]: %v", err)
		return path
	}
	ctx := context.Background()
	var filesMap map[string]string = make(map[string]string)
	filesMap[path] = ""
	files, err := archiver.FilesFromDisk(&archiver.FromDiskOptions{}, filesMap)
	if err != nil {
		L().Errorf("[TarPath]: %v", err)
		return path
	}
	err = tar.Archive(ctx, io.MultiWriter(writer, t), files)
	if err != nil {
		L().Errorf("[TarPath]: %v", err)
		return path
	}
	return outPath
}
