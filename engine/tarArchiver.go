package engine

import (
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
}

func (t *TarStatus) Name() string {
	return t.name
}

func (t *TarStatus) CompletedLength() int64 {
	return 0
}

func (t *TarStatus) TotalLength() int64 {
	return 0
}

func (t *TarStatus) Speed() int64 {
	return 0
}

func (t *TarStatus) ETA() *time.Duration {
	dur := time.Duration(0)
	return &dur
}

func (t *TarStatus) Gid() string {
	return t.gid
}

func (t *TarStatus) Percentage() float32 {
	return float32(0)
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

func NewTarStatus(gid string, name string, listener *MirrorListener) *TarStatus {
	return &TarStatus{gid: gid, name: name, listener: listener}
}

func TarPath(path string) string {
	outPath := path + ".tar"
	log.Printf("[TarPath]: %s -> %s\n", path, outPath)
	tar := archiver.NewTar()
	tar.ImplicitTopLevelFolder = true
	tar.Archive([]string{path}, outPath)
	return outPath
}
