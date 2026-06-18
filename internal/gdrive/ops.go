package gdrive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

// Upload uploads a local file or directory to parentID. Returns the created
// Drive file/folder id.
func (t *Transfer) Upload(ctx context.Context, path, parentID string) (_ string, retErr error) {
	defer func() { t.recordDone(retErr) }()
	t.setName(filepath.Base(path))
	t.nameMu.Lock()
	t.path = path
	t.nameMu.Unlock()
	t.startObserver()
	defer t.stopObserver()

	fi, err := os.Stat(path)
	if err != nil {
		t.setErr(err)
		return "", err
	}

	if fi.IsDir() {
		t.total.Store(localDirSize(path))
		srv, err := t.client.auth.Service(ctx)
		if err != nil {
			t.setErr(err)
			return "", err
		}
		dir, err := createDir(ctx, srv, filepath.Base(path), parentID)
		if err != nil {
			t.setErr(err)
			return "", err
		}
		if err := t.uploadDir(ctx, path, dir.Id); err != nil {
			t.setErr(err)
			return "", err
		}
		t.completedFlag.Store(true)
		return dir.Id, nil
	}

	t.total.Store(fi.Size())
	if err := t.uploadFile(ctx, path, parentID); err != nil {
		t.setErr(err)
		return "", err
	}
	t.completedFlag.Store(true)
	return t.FileID(), nil
}

func (t *Transfer) uploadDir(ctx context.Context, root, parentID string) error {
	type item struct{ src, dst string }
	queue := []item{{root, parentID}}
	var firstErr error
	var errMu sync.Mutex
	fail := func(e error) {
		errMu.Lock()
		if firstErr == nil {
			firstErr = e
		}
		errMu.Unlock()
	}

	for len(queue) > 0 {
		if t.cancelled.Load() {
			return errCancelled
		}
		it := queue[0]
		queue = queue[1:]
		entries, err := os.ReadDir(it.src)
		if err != nil {
			return err
		}
		srv, err := t.client.auth.Service(ctx)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if t.cancelled.Load() {
				return errCancelled
			}
			abs := filepath.Join(it.src, e.Name())
			if e.IsDir() {
				sub, err := createDir(ctx, srv, e.Name(), it.dst)
				if err != nil {
					return err
				}
				queue = append(queue, item{abs, sub.Id})
				continue
			}
			t.sem <- struct{}{}
			t.wg.Add(1)
			go func(p, parent string) {
				defer t.wg.Done()
				defer func() { <-t.sem }()
				if err := t.uploadFile(ctx, p, parent); err != nil {
					fail(err)
				}
			}(abs, it.dst)
		}
	}
	t.wg.Wait()
	return firstErr
}

func (t *Transfer) uploadFile(ctx context.Context, path, parentID string) error {
	var lastErr error
	for retry := 0; retry < maxRetries; retry++ {
		if t.cancelled.Load() {
			return errCancelled
		}
		srv, err := t.client.auth.Service(ctx)
		if err != nil {
			lastErr = err
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		var lastCompleted int64
		drvFile := &drive.File{Name: filepath.Base(path), Parents: []string{parentID}}
		created, err := srv.Files.Create(drvFile).
			SupportsAllDrives(true).
			Media(&cancelReader{t: t, r: f}, googleapi.ChunkSize(chunkSize)).
			ProgressUpdater(func(cur, _ int64) {
				t.addCompleted(cur - lastCompleted)
				lastCompleted = cur
			}).Do()
		f.Close()
		if err != nil {
			if t.cancelled.Load() || errors.Is(err, errCancelled) {
				return errCancelled
			}
			t.addCompleted(-lastCompleted) // roll back partial progress before retry
			lastErr = err
			t.client.auth.RotateSA()
			continue
		}
		t.setFileID(created.Id)
		return nil
	}
	return lastErr
}

// Download downloads a Drive file/folder to localDir. Returns the local output path.
func (t *Transfer) Download(ctx context.Context, fileID, localDir string) (_ string, retErr error) {
	defer func() { t.recordDone(retErr) }()
	t.startObserver()
	defer t.stopObserver()

	srv, err := t.client.auth.Service(ctx)
	if err != nil {
		t.setErr(err)
		return "", err
	}
	meta, err := getMeta(ctx, srv, fileID)
	if err != nil {
		t.setErr(err)
		return "", err
	}
	t.setName(meta.Name)
	if t.total.Load() == 0 {
		if meta.MimeType == folderMIME {
			t.total.Store(t.folderSize(ctx, srv, meta.Id))
		} else {
			t.total.Store(meta.Size)
		}
	}

	if meta.MimeType == folderMIME {
		out := filepath.Join(localDir, meta.Name)
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.setErr(err)
			return "", err
		}
		t.nameMu.Lock()
		t.path = out
		t.nameMu.Unlock()
		if err := t.downloadDir(ctx, meta.Id, out); err != nil {
			t.setErr(err)
			return "", err
		}
		t.completedFlag.Store(true)
		return out, nil
	}

	out := filepath.Join(localDir, meta.Name)
	t.nameMu.Lock()
	t.path = out
	t.nameMu.Unlock()
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.setErr(err)
		return "", err
	}
	if err := t.downloadFile(ctx, meta, localDir); err != nil {
		t.setErr(err)
		return "", err
	}
	t.completedFlag.Store(true)
	return out, nil
}

func (t *Transfer) downloadDir(ctx context.Context, folderID, localDir string) error {
	type item struct {
		src string
		dst string
	}
	queue := []item{{folderID, localDir}}
	var firstErr error
	var errMu sync.Mutex
	fail := func(e error) {
		errMu.Lock()
		if firstErr == nil {
			firstErr = e
		}
		errMu.Unlock()
	}
	for len(queue) > 0 {
		if t.cancelled.Load() {
			return errCancelled
		}
		it := queue[0]
		queue = queue[1:]
		srv, err := t.client.auth.Service(ctx)
		if err != nil {
			return err
		}
		files, err := listAll(ctx, srv, it.src)
		if err != nil {
			return err
		}
		for _, file := range files {
			if t.cancelled.Load() {
				return errCancelled
			}
			abs := filepath.Join(it.dst, file.Name)
			if file.MimeType == folderMIME {
				if err := os.MkdirAll(abs, 0o755); err != nil {
					return err
				}
				queue = append(queue, item{file.Id, abs})
				continue
			}
			t.sem <- struct{}{}
			t.wg.Add(1)
			go func(f *drive.File, dir string) {
				defer t.wg.Done()
				defer func() { <-t.sem }()
				if err := t.downloadFile(ctx, f, dir); err != nil {
					fail(err)
				}
			}(file, it.dst)
		}
	}
	t.wg.Wait()
	return firstErr
}

func (t *Transfer) downloadFile(ctx context.Context, file *drive.File, localDir string) error {
	dest := filepath.Join(localDir, file.Name)
	var lastErr error
	for retry := 0; retry < maxRetries; retry++ {
		if t.cancelled.Load() {
			return errCancelled
		}
		srv, err := t.client.auth.Service(ctx)
		if err != nil {
			lastErr = err
			continue
		}
		out, err := os.Create(dest)
		if err != nil {
			return err
		}
		resp, err := srv.Files.Get(file.Id).SupportsAllDrives(true).Download()
		if err != nil {
			out.Close()
			lastErr = err
			t.client.auth.RotateSA()
			continue
		}
		written, err := io.Copy(&countWriter{t: t, w: out}, resp.Body)
		resp.Body.Close()
		out.Close()
		if err != nil {
			if t.cancelled.Load() || errors.Is(err, errCancelled) {
				return errCancelled
			}
			t.addCompleted(-written) // roll back before retry
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}

// Clone server-side copies a Drive file/folder into destID. Returns the new id.
func (t *Transfer) Clone(ctx context.Context, srcID, destID string) (_ string, retErr error) {
	defer func() { t.recordDone(retErr) }()
	t.startObserver()
	defer t.stopObserver()

	srv, err := t.client.auth.Service(ctx)
	if err != nil {
		t.setErr(err)
		return "", err
	}
	meta, err := getMeta(ctx, srv, srcID)
	if err != nil {
		t.setErr(err)
		return "", err
	}
	t.setName(meta.Name)

	if meta.MimeType == folderMIME {
		t.total.Store(t.folderSize(ctx, srv, meta.Id))
		newDir, err := createDir(ctx, srv, meta.Name, destID)
		if err != nil {
			t.setErr(err)
			return "", err
		}
		if err := t.cloneDir(ctx, meta.Id, newDir.Id); err != nil {
			t.setErr(err)
			return "", err
		}
		t.completedFlag.Store(true)
		return newDir.Id, nil
	}

	t.total.Store(meta.Size)
	id, err := t.cloneFile(ctx, meta, destID)
	if err != nil {
		t.setErr(err)
		return "", err
	}
	t.completedFlag.Store(true)
	return id, nil
}

func (t *Transfer) cloneDir(ctx context.Context, srcID, destID string) error {
	type item struct{ src, dst string }
	queue := []item{{srcID, destID}}
	var firstErr error
	var errMu sync.Mutex
	fail := func(e error) {
		errMu.Lock()
		if firstErr == nil {
			firstErr = e
		}
		errMu.Unlock()
	}
	for len(queue) > 0 {
		if t.cancelled.Load() {
			return errCancelled
		}
		it := queue[0]
		queue = queue[1:]
		srv, err := t.client.auth.Service(ctx)
		if err != nil {
			return err
		}
		files, err := listAll(ctx, srv, it.src)
		if err != nil {
			return err
		}
		for _, file := range files {
			if t.cancelled.Load() {
				return errCancelled
			}
			if file.MimeType == folderMIME {
				newDir, err := createDir(ctx, srv, file.Name, it.dst)
				if err != nil {
					return err
				}
				queue = append(queue, item{file.Id, newDir.Id})
				continue
			}
			f := file
			dst := it.dst
			t.sem <- struct{}{}
			t.wg.Add(1)
			go func() {
				defer t.wg.Done()
				defer func() { <-t.sem }()
				if _, err := t.cloneFile(ctx, f, dst); err != nil {
					fail(err)
				}
			}()
		}
	}
	t.wg.Wait()
	return firstErr
}

func (t *Transfer) cloneFile(ctx context.Context, file *drive.File, destID string) (string, error) {
	var lastErr error
	for retry := 0; retry < maxRetries; retry++ {
		if t.cancelled.Load() {
			return "", errCancelled
		}
		srv, err := t.client.auth.Service(ctx)
		if err != nil {
			lastErr = err
			continue
		}
		newFile, err := srv.Files.Copy(file.Id, &drive.File{Parents: []string{destID}}).
			Fields("id").SupportsAllDrives(true).Do()
		if err != nil {
			lastErr = err
			t.client.auth.RotateSA()
			continue
		}
		t.addCompleted(file.Size)
		t.setFileID(newFile.Id)
		return newFile.Id, nil
	}
	return "", lastErr
}

func (t *Transfer) folderSize(ctx context.Context, srv *drive.Service, folderID string) int64 {
	var total int64
	files, err := listAll(ctx, srv, folderID)
	if err != nil {
		return total
	}
	for _, f := range files {
		if t.cancelled.Load() {
			return total
		}
		if f.MimeType == folderMIME {
			total += t.folderSize(ctx, srv, f.Id)
		} else {
			total += f.Size
		}
	}
	return total
}

// --- Client-level list / metadata ---

// Metadata fetches metadata for a Drive file/folder.
func (c *Client) Metadata(ctx context.Context, fileID string) (*drive.File, error) {
	srv, err := c.auth.Service(ctx)
	if err != nil {
		return nil, err
	}
	return getMeta(ctx, srv, fileID)
}

// List lists files under parentID, optionally filtered by name substring.
// count == -1 means unlimited.
func (c *Client) List(ctx context.Context, parentID, name string, count int) ([]*drive.File, error) {
	srv, err := c.auth.Service(ctx)
	if err != nil {
		return nil, err
	}
	return listLimited(ctx, srv, parentID, name, count)
}

// --- shared drive helpers ---

func createDir(ctx context.Context, srv *drive.Service, name, parentID string) (*drive.File, error) {
	d := &drive.File{Name: name, MimeType: folderMIME, Parents: []string{parentID}}
	f, err := srv.Files.Create(d).SupportsAllDrives(true).Do()
	if err != nil {
		return nil, fmt.Errorf("create dir %q: %w", name, err)
	}
	return f, nil
}

func getMeta(ctx context.Context, srv *drive.Service, fileID string) (*drive.File, error) {
	return srv.Files.Get(fileID).
		Fields("id,name,mimeType,size,md5Checksum").
		SupportsAllDrives(true).Do()
}

func listAll(ctx context.Context, srv *drive.Service, parentID string) ([]*drive.File, error) {
	return listLimited(ctx, srv, parentID, "", -1)
}

func listLimited(ctx context.Context, srv *drive.Service, parentID, name string, count int) ([]*drive.File, error) {
	query := fmt.Sprintf("'%s' in parents", parentID)
	if name != "" {
		query += fmt.Sprintf(" and name contains '%s'", name)
	}
	var files []*drive.File
	pageToken := ""
	for {
		call := srv.Files.List().Q(query).
			OrderBy("modifiedTime desc").
			SupportsAllDrives(true).
			IncludeItemsFromAllDrives(true).
			PageSize(1000).
			Fields("nextPageToken, files(id, name, size, mimeType)")
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		res, err := call.Do()
		if err != nil {
			return files, err
		}
		for _, f := range res.Files {
			if count != -1 && len(files) == count {
				return files, nil
			}
			files = append(files, f)
		}
		pageToken = res.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return files, nil
}

func localDirSize(root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

// cancelReader aborts an upload read when the transfer is cancelled.
type cancelReader struct {
	t *Transfer
	r io.Reader
}

func (c *cancelReader) Read(p []byte) (int, error) {
	if c.t.cancelled.Load() {
		return 0, errCancelled
	}
	return c.r.Read(p)
}

// countWriter counts bytes written and aborts on cancellation.
type countWriter struct {
	t *Transfer
	w io.Writer
}

func (c *countWriter) Write(p []byte) (int, error) {
	if c.t.cancelled.Load() {
		return 0, errCancelled
	}
	n, err := c.w.Write(p)
	c.t.addCompleted(int64(n))
	return n, err
}
