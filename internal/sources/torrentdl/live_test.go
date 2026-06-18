package torrentdl

import (
	"context"
	"os"
	"testing"
	"time"
)

type noopListener struct{}

func (noopListener) OnDownloadStart()      {}
func (noopListener) OnDownloadComplete()   {}
func (noopListener) OnDownloadError(error) {}

// TestLiveMagnet verifies the embedded torrent engine can fetch metadata and
// connect to peers for a real magnet. Gated behind TORRENT_LIVE_TEST=1.
func TestLiveMagnet(t *testing.T) {
	if os.Getenv("TORRENT_LIVE_TEST") != "1" {
		t.Skip("set TORRENT_LIVE_TEST=1 to run")
	}
	const magnet = "magnet:?xt=urn:btih:b91d49b1313f5aa27f2b3a9dc48af8248f40908e&dn=ubuntu-mate-25.10-desktop-amd64.iso&tr=https%3A%2F%2Ftorrent.ubuntu.com%2Fannounce"

	dir := t.TempDir()
	eng, err := NewEngine(EngineConfig{DownloadDir: dir, ListenPort: 56999})
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	defer eng.Close()

	d, err := eng.AddMagnet(magnet, dir, false, noopListener{})
	if err != nil {
		t.Fatalf("add magnet: %v", err)
	}
	d.Start(context.Background())

	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		if d.hasInfo() {
			t.Logf("METADATA OK: name=%q total=%d pieces=%d peers=%d seeders=%d",
				d.Name(), d.TotalLength(), d.PiecesTotal(), d.Peers(), d.Seeders())
			// Observe a few seconds of progress.
			time.Sleep(8 * time.Second)
			t.Logf("PROGRESS: completed=%d speed=%d peers=%d seeders=%d pieces=%d/%d",
				d.CompletedLength(), d.Speed(), d.Peers(), d.Seeders(), d.PiecesCompleted(), d.PiecesTotal())
			d.Cancel()
			return
		}
		t.Logf("waiting for metadata... name=%q peers=%d", d.Name(), d.Peers())
		time.Sleep(3 * time.Second)
	}
	d.Cancel()
	t.Fatal("timed out waiting for torrent metadata")
}
