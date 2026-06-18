// Package archive provides tar creation and archive extraction with progress,
// exposed through the status.Status interface so the operations show up in the
// Telegram status display like any other unit of work.
package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"mirrorbot/internal/metrics"
	"mirrorbot/internal/status"
	"mirrorbot/internal/util"
)

var errCancelled = errors.New("cancelled by user")

// recordArchive emits the op result + byte counters for a finished archive op.
func (p *progress) recordArchive(op string, err error) {
	res := metrics.ResultComplete
	switch {
	case err == nil:
	case p.cancelled.Load() || errors.Is(err, errCancelled):
		res = metrics.ResultCancelled
	default:
		res = metrics.ResultError
	}
	metrics.ArchiveOps.WithLabelValues(op, res).Inc()
	metrics.ArchiveBytes.WithLabelValues(op).Add(float64(p.completed.Load()))
}

// progress is the shared status.Status backing for archive operations.
type progress struct {
	name string
	gid  string
	path string
	typ  status.StatusType

	completed atomic.Int64
	total     atomic.Int64
	speed     atomic.Int64
	index     atomic.Int64
	cancelled atomic.Bool

	observerStop chan struct{}
}

func (p *progress) Name() string           { return p.name }
func (p *progress) CompletedLength() int64 { return p.completed.Load() }
func (p *progress) TotalLength() int64     { return p.total.Load() }
func (p *progress) Speed() int64           { return p.speed.Load() }
func (p *progress) GID() string            { return p.gid }
func (p *progress) Path() string           { return p.path }
func (p *progress) StatusType() status.StatusType {
	if p.cancelled.Load() {
		return status.Canceled
	}
	return p.typ
}
func (p *progress) Index() int     { return int(p.index.Load()) }
func (p *progress) SetIndex(i int) { p.index.Store(int64(i)) }
func (p *progress) Cancel() bool   { p.cancelled.Store(true); return true }

func (p *progress) Percentage() float32 {
	total := p.total.Load()
	if total == 0 {
		return 0
	}
	return float32(p.completed.Load()*100) / float32(total)
}

func (p *progress) ETA() *time.Duration {
	left := p.total.Load() - p.completed.Load()
	if left < 0 {
		left = 0
	}
	eta := util.CalculateETA(left, p.speed.Load())
	return &eta
}

func (p *progress) startObserver() {
	p.observerStop = make(chan struct{})
	go func() {
		last := p.completed.Load()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-p.observerStop:
				return
			case <-ticker.C:
				now := p.completed.Load()
				p.speed.Store(now - last)
				last = now
			}
		}
	}()
}

func (p *progress) stopObserver() {
	if p.observerStop != nil {
		close(p.observerStop)
	}
}

// countWriter increments progress as bytes flow through.
type countWriter struct {
	p *progress
	w io.Writer
}

func (c *countWriter) Write(b []byte) (int, error) {
	if c.p.cancelled.Load() {
		return 0, errCancelled
	}
	n, err := c.w.Write(b)
	c.p.completed.Add(int64(n))
	return n, err
}

// TarArchiver creates a .tar archive of a path.
type TarArchiver struct{ progress }

// NewTarArchiver returns a TarArchiver status for name/gid.
func NewTarArchiver(name, gid string) *TarArchiver {
	t := &TarArchiver{}
	t.name = name
	t.gid = gid
	t.typ = status.Archiving
	return t
}

// Tar archives srcPath into "<srcPath>.tar" and returns the new path.
func (t *TarArchiver) Tar(srcPath string) (_ string, retErr error) {
	defer func() { t.recordArchive("tar", retErr) }()
	t.path = srcPath
	t.total.Store(localSize(srcPath))
	t.startObserver()
	defer t.stopObserver()

	outPath := srcPath + ".tar"
	out, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	tw := tar.NewWriter(&countWriter{p: &t.progress, w: out})
	defer tw.Close()

	base := filepath.Dir(srcPath)
	walkErr := filepath.WalkDir(srcPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if t.cancelled.Load() {
			return errCancelled
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
	if walkErr != nil {
		return "", walkErr
	}
	return outPath, nil
}

// UnArchiver extracts an archive (.zip, .tar, .tar.gz/.tgz) with progress.
type UnArchiver struct{ progress }

// NewUnArchiver returns an UnArchiver status for name/gid.
func NewUnArchiver(name, gid string) *UnArchiver {
	u := &UnArchiver{}
	u.name = name
	u.gid = gid
	u.typ = status.UnArchiving
	return u
}

// Supported reports whether path has a supported archive extension.
func Supported(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".zip") ||
		strings.HasSuffix(lower, ".tar") ||
		strings.HasSuffix(lower, ".tar.gz") ||
		strings.HasSuffix(lower, ".tgz")
}

// Extract unpacks srcPath into a sibling directory and returns the directory.
func (u *UnArchiver) Extract(srcPath string) (_ string, retErr error) {
	defer func() { u.recordArchive("untar", retErr) }()
	u.path = srcPath
	u.startObserver()
	defer u.stopObserver()

	outDir := stripArchiveExt(srcPath)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}

	lower := strings.ToLower(srcPath)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return outDir, u.extractZip(srcPath, outDir)
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		return outDir, u.extractTarGz(srcPath, outDir)
	case strings.HasSuffix(lower, ".tar"):
		return outDir, u.extractTar(srcPath, outDir)
	default:
		return "", errors.New("unsupported archive format")
	}
}

func (u *UnArchiver) extractZip(srcPath, outDir string) error {
	zr, err := zip.OpenReader(srcPath)
	if err != nil {
		return err
	}
	defer zr.Close()
	var total int64
	for _, f := range zr.File {
		total += int64(f.UncompressedSize64)
	}
	u.total.Store(total)
	for _, f := range zr.File {
		if u.cancelled.Load() {
			return errCancelled
		}
		dest := filepath.Join(outDir, filepath.Clean(f.Name))
		if !strings.HasPrefix(dest, filepath.Clean(outDir)+string(os.PathSeparator)) && dest != filepath.Clean(outDir) {
			continue // zip-slip guard
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(dest)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(&countWriter{p: &u.progress, w: out}, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (u *UnArchiver) extractTarGz(srcPath, outDir string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	return u.untar(gz, outDir)
}

func (u *UnArchiver) extractTar(srcPath, outDir string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return u.untar(f, outDir)
}

func (u *UnArchiver) untar(r io.Reader, outDir string) error {
	tr := tar.NewReader(r)
	for {
		if u.cancelled.Load() {
			return errCancelled
		}
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		dest := filepath.Join(outDir, filepath.Clean(hdr.Name))
		if !strings.HasPrefix(dest, filepath.Clean(outDir)+string(os.PathSeparator)) && dest != filepath.Clean(outDir) {
			continue // tar-slip guard
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}
			out, err := os.Create(dest)
			if err != nil {
				return err
			}
			_, err = io.Copy(&countWriter{p: &u.progress, w: out}, tr)
			out.Close()
			if err != nil {
				return err
			}
		}
	}
}

func stripArchiveExt(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"):
		return path[:len(path)-len(".tar.gz")]
	case strings.HasSuffix(lower, ".tgz"):
		return path[:len(path)-len(".tgz")]
	default:
		return strings.TrimSuffix(path, filepath.Ext(path))
	}
}

func localSize(root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, e := d.Info(); e == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}
