package engine

import (
	"MirrorBotGo/utils"
	"archive/tar"
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mholt/archiver"
	"github.com/nwaples/rardecode"
)

type UnArchiverStatus struct {
	name       string
	listener   *MirrorListener
	gid        string
	Index_     int
	unarchiver *UnArchiver
}

func (t *UnArchiverStatus) Name() string {
	return t.name
}

func (t *UnArchiverStatus) CompletedLength() int64 {
	return t.unarchiver.Completed
}

func (t *UnArchiverStatus) TotalLength() int64 {
	return t.unarchiver.Total
}

func (t *UnArchiverStatus) Speed() int64 {
	return t.unarchiver.Speed
}

func (t *UnArchiverStatus) ETA() *time.Duration {
	dur := t.unarchiver.ETA
	return &dur
}

func (t *UnArchiverStatus) Gid() string {
	return t.gid
}

func (t *UnArchiverStatus) Percentage() float32 {
	return float32(t.CompletedLength()*100) / float32(t.TotalLength())
}

func (t *UnArchiverStatus) GetStatusType() string {
	return MirrorStatusUnArchiving
}

func (t *UnArchiverStatus) Path() string {
	return t.Name()
}

func (t *UnArchiverStatus) Index() int {
	return t.Index_
}

func (t *UnArchiverStatus) GetListener() *MirrorListener {
	return t.listener
}

func (t *UnArchiverStatus) GetCloneListener() *CloneListener {
	return nil
}

func (t *UnArchiverStatus) CancelMirror() bool {
	return false
}

func NewUnArchiverStatus(gid string, name string, listener *MirrorListener, unarchiver *UnArchiver) *UnArchiverStatus {
	return &UnArchiverStatus{gid: gid, name: name, listener: listener, unarchiver: unarchiver}
}

type UnArchiver struct {
	Prg       *archiver.Progress
	Speed     int64
	StartTime time.Time
	Completed int64
	isDone    bool
	Total     int64
	ETA       time.Duration
}

func (t *UnArchiver) OnUnArchiveProgress() {
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

func (t *UnArchiver) ProgressLoop() {
	for {
		if t.isDone {
			break
		}
		t.OnUnArchiveProgress()
		time.Sleep(1 * time.Second)
	}
}

func (t *UnArchiver) UnArchivePath(path string) string {
	outPath := utils.TrimExt(path)
	os.MkdirAll(outPath, 0755)
	L().Infof("[UnArchivePath]: %s -> %s", path, outPath)
	go t.ProgressLoop()
	err := Unarchive(path, outPath, true, t.Prg)
	t.isDone = true
	if err != nil {
		L().Errorf("[UnArchiveError]: %v, uploading without unarchive.", err)
		return path
	}
	return outPath
}

func createFilesSizeWalker(accumulator *int64) func(archiver.File) error {
	return func(f archiver.File) error {
		if f.IsDir() {
			return nil
		}
		*accumulator += f.Size()
		return nil
	}
}

func GetArchiveContentSize(path string) (int64, error) {
	var size int64
	walker := createFilesSizeWalker(&size)
	err := archiver.Walk(path, walker)
	return size, err
}

func NewUnArchiver(p *archiver.Progress, total int64) *UnArchiver {
	return &UnArchiver{Prg: p, Total: total, StartTime: time.Now()}
}

func Unarchive(inputFile string, outputDir string, archiveHasBaseDir bool, prg *archiver.Progress) error {
	inputFile = filepath.ToSlash(inputFile)
	outputDir = filepath.ToSlash(outputDir)
	extractFilesWalker := createExtractFilesWalker(prg, archiveHasBaseDir, outputDir)
	return archiver.Walk(inputFile, extractFilesWalker)
}

func createExtractFilesWalker(prg *archiver.Progress, archiveHasBaseDir bool, outputDir string) func(archiver.File) error {
	pathSeparator := fmt.Sprintf("%c", os.PathSeparator)
	return func(f archiver.File) error {
		name := f.Name()
		switch h := f.Header.(type) {
		case zip.FileHeader:
			name = h.Name
		case *tar.Header:
			name = h.Name
		case *rardecode.FileHeader:
			name = h.Name
		default:
			return fmt.Errorf("Unable to process %v", h)
		}

		dirname := filepath.ToSlash(filepath.Join(outputDir, filepath.Dir(name)))
		if archiveHasBaseDir {
			namePathParts := strings.Split(filepath.Dir(name), pathSeparator)
			dirname = filepath.ToSlash(filepath.Join(outputDir, filepath.Join(namePathParts[1:]...)))
			os.MkdirAll(dirname, 0755)
		}

		if f.IsDir() {
			os.MkdirAll(dirname, 0755)
			return nil
		}

		outFile := filepath.Join(dirname, filepath.Base(name))
		outf, err := os.Create(outFile)
		if err != nil {
			return err
		}
		_, err = io.Copy(outf, io.TeeReader(f, prg))
		if err != nil {
			fmt.Println("error lol: ", err)
		}
		return err
	}
}
