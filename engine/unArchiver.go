package engine

import (
	"MirrorBotGo/utils"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mholt/archiver/v4"
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

func (t *UnArchiverStatus) IsTorrent() bool {
	return false
}

func (t *UnArchiverStatus) GetPeers() int {
	return 0
}

func (t *UnArchiverStatus) GetSeeders() int {
	return 0
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
	Speed     int64
	StartTime time.Time
	Completed int64
	isDone    bool
	Total     int64
	ETA       time.Duration
}

func (t *UnArchiver) SetTotal(total int64) {
	t.Total = total
}

func (t *UnArchiver) Write(b []byte) (int, error) {
	length := len(b)
	completed := t.Completed + int64(length)
	t.OnTransferUpdate(completed, t.Total)
	return length, nil
}

func (t *UnArchiver) OnTransferUpdate(completed int64, total int64) {
	t.Completed = completed
	t.Total = total
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

func (t *UnArchiver) CalculateTotalSize(path string) (int64, error) {
	reader, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	format, archiveReader, err := archiver.Identify(path, reader)
	if err != nil {
		return 0, err
	}
	ctx := context.Background()
	var size int64
	if ex, ok := format.(archiver.Extractor); ok {
		err = ex.Extract(ctx, archiveReader, nil, func(ctx context.Context, f archiver.File) error {
			size += f.Size()
			return nil
		})
		if err != nil {
			L().Errorf("UnArchiver: CalculateTotalSize: %s : %v", path, err)
			return 0, err
		}
	} else {
		return 0, errors.New("Unsupported archive")
	}
	return size, nil
}

func fileNameWithoutExtSliceNotation(fileName string) string {
	return fileName[:len(fileName)-len(filepath.Ext(fileName))]
}

func (t *UnArchiver) UnArchivePath(path string) (string, error) {
	L().Infof("[Unarchive] starting unarchive: %s", path)
	outPath := fileNameWithoutExtSliceNotation(path)
	err := os.MkdirAll(outPath, 0755)
	if err != nil {
		L().Errorf("UnArchiver: UnArchivePath: os.MkdirAll: %s : %v", path, err)
		return "", err
	}
	reader, err := os.Open(path)
	if err != nil {
		return path, err
	}
	format, archiveReader, err := archiver.Identify(path, reader)
	if err != nil {
		return path, err
	}
	ctx := context.Background()
	if ex, ok := format.(archiver.Extractor); ok {
		err = ex.Extract(ctx, archiveReader, nil, func(ctx context.Context, f archiver.File) error {
			if f.IsDir() {
				return nil
			}
			writerPath := filepath.Join(outPath, f.NameInArchive)
			dir := filepath.Dir(writerPath)
			err = os.MkdirAll(dir, 0755)
			if err != nil {
				L().Errorf("UnArchiver: UnArchivePath: ex.Extract: os.MkdirAll: %s : %v", path, err)
				//in case directory exists, no need to stop the extraction.
			}
			writer, err := os.Create(writerPath)
			if err != nil {
				return err
			}
			reader, err := f.Open()
			if err != nil {
				return err
			}
			defer func(reader io.ReadCloser) {
				err := reader.Close()
				if err != nil {
					L().Errorf("UnArchiver: UnArchivePath: ex.Extract: Failed to close reader: %v", err)
				}
			}(reader)
			_, err = io.Copy(io.MultiWriter(writer, t), reader)
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return path, err
		}
	} else {
		return path, errors.New("Unsupported archive")
	}
	return outPath, nil
}

func NewUnArchiver() *UnArchiver {
	return &UnArchiver{StartTime: time.Now()}
}
