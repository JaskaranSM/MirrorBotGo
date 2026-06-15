package gdrive

import (
	"context"
	"os"
	"testing"
)

// TestLiveList exercises real service-account auth + Drive list against the
// configured parent folder. Gated behind GDRIVE_LIVE_TEST=1 so it never runs in
// normal `go test ./...`.
func TestLiveList(t *testing.T) {
	if os.Getenv("GDRIVE_LIVE_TEST") != "1" {
		t.Skip("set GDRIVE_LIVE_TEST=1 (plus SA_DIR, GDRIVE_PARENT_ID) to run")
	}
	saDir := os.Getenv("SA_DIR")
	parent := os.Getenv("GDRIVE_PARENT_ID")

	c, err := New(Config{UseSA: true, SADir: saDir}, 4)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	t.Logf("loaded %d service accounts", c.auth.NumAccounts())

	files, err := c.List(context.Background(), parent, "", 10)
	if err != nil {
		t.Fatalf("list %s: %v", parent, err)
	}
	t.Logf("listed %d entries in folder %s", len(files), parent)
	for _, f := range files {
		t.Logf("  id=%s  name=%q  mime=%s  size=%d", f.Id, f.Name, f.MimeType, f.Size)
	}
}
