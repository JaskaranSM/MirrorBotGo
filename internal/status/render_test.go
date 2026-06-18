package status

import (
	"testing"
	"time"

	"mirrorbot/internal/util"
)

type fakeStatus struct {
	name      string
	completed int64
	total     int64
	speed     int64
	eta       *time.Duration
	gid       string
	pct       float32
	st        StatusType
	index     int
}

func (f *fakeStatus) Name() string           { return f.name }
func (f *fakeStatus) CompletedLength() int64 { return f.completed }
func (f *fakeStatus) TotalLength() int64     { return f.total }
func (f *fakeStatus) Speed() int64           { return f.speed }
func (f *fakeStatus) ETA() *time.Duration    { return f.eta }
func (f *fakeStatus) GID() string            { return f.gid }
func (f *fakeStatus) Path() string           { return "" }
func (f *fakeStatus) Percentage() float32    { return f.pct }
func (f *fakeStatus) StatusType() StatusType { return f.st }
func (f *fakeStatus) Index() int             { return f.index }
func (f *fakeStatus) SetIndex(i int)         { f.index = i }
func (f *fakeStatus) Cancel() bool           { return true }

type fakeTorrent struct {
	fakeStatus
	peers, seeders, pc, pt int
}

func (f *fakeTorrent) Peers() int           { return f.peers }
func (f *fakeTorrent) Seeders() int         { return f.seeders }
func (f *fakeTorrent) PiecesCompleted() int { return f.pc }
func (f *fakeTorrent) PiecesTotal() int     { return f.pt }

func TestRenderDownloading(t *testing.T) {
	eta := 44 * time.Second
	dl := &fakeStatus{
		name: "ubuntu.iso", completed: 450 * 1024 * 1024, total: 1000 * 1024 * 1024,
		speed: 12*1024*1024 + 512*1024, eta: &eta, gid: "abc123", pct: 45.0, st: Downloading, index: 0,
	}
	got := RenderProgress([]Status{dl}, "FOOTER")
	want := "<i>ubuntu.iso</i> - Downloading\n" +
		"<code>" + util.GetProgressBarString(450*1024*1024, 1000*1024*1024) + " 45.00% </code>, 450.0 MB of 1000.0 MB at 12.5 MB/s, ETA: 44s\n" +
		"GID: <code>abc123</code> I: <code>0</code>\n\n" +
		"FOOTER | DL: 12.5 MB | UP: 0 B"
	if got != want {
		t.Fatalf("render mismatch:\n got=%q\nwant=%q", got, want)
	}
}

func TestRenderCloning(t *testing.T) {
	dl := &fakeStatus{
		name: "My Folder", completed: 512 * 1024 * 1024, total: 2 * 1024 * 1024 * 1024,
		speed: 0, gid: "xyz", st: Cloning, index: 1,
	}
	got := RenderProgress([]Status{dl}, "FOOTER")
	want := "<i>My Folder</i> - Cloning\n" +
		"512.0 MB of 2.0 GB at 0 B/s, \n" +
		"GID: <code>xyz</code> I: <code>1</code>\n\n" +
		"FOOTER | DL: 0 B | UP: 0 B"
	if got != want {
		t.Fatalf("clone render mismatch:\n got=%q\nwant=%q", got, want)
	}
}

func TestRenderTorrentSuffix(t *testing.T) {
	eta := 10 * time.Second
	dl := &fakeTorrent{
		fakeStatus: fakeStatus{
			name: "t", completed: 50, total: 100, speed: 5, eta: &eta, gid: "g", pct: 50, st: Downloading,
		},
		peers: 12, seeders: 4, pc: 450, pt: 1000,
	}
	got := RenderProgress([]Status{dl}, "F")
	want := "<i>t</i> - Downloading\n" +
		"<code>" + util.GetProgressBarString(50, 100) + " 50.00% </code>, 50 B of 100 B at 5 B/s, ETA: 10s | P: 12 | S: 4 | PC: 450/1000\n" +
		"GID: <code>g</code> I: <code>0</code>\n\n" +
		"F | DL: 5 B | UP: 0 B"
	if got != want {
		t.Fatalf("torrent render mismatch:\n got=%q\nwant=%q", got, want)
	}
}
