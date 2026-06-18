// Package gdrive embeds the Google Drive transfer subsystem (upload, download,
// server-side clone, list, metadata). It replaces the original standalone
// transfer-service and exposes progress through the status.Status interface.
package gdrive

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"

	"mirrorbot/internal/metrics"
)

const folderMIME = "application/vnd.google-apps.folder"

// Config configures Drive authentication.
type Config struct {
	UseSA           bool
	SADir           string
	CredentialsFile string
	TokenFile       string
}

// Auth builds authorized *drive.Service instances, rotating across service
// accounts when configured (spreading quota and surviving per-account limits).
type Auth struct {
	cfg     Config
	saFiles []string
	saIndex atomic.Int64
	mu      sync.Mutex // guards OAuth token file access
}

// NewAuth loads the service-account list (when UseSA) and returns an Auth.
func NewAuth(cfg Config) (*Auth, error) {
	a := &Auth{cfg: cfg}
	if cfg.UseSA {
		entries, err := os.ReadDir(cfg.SADir)
		if err != nil {
			return nil, fmt.Errorf("read service account dir %q: %w", cfg.SADir, err)
		}
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
				continue
			}
			a.saFiles = append(a.saFiles, filepath.Join(cfg.SADir, e.Name()))
		}
		if len(a.saFiles) == 0 {
			return nil, fmt.Errorf("no service account .json files found in %q", cfg.SADir)
		}
	}
	return a, nil
}

// RotateSA advances the service-account pointer (call on quota/rate-limit errors).
func (a *Auth) RotateSA() {
	if len(a.saFiles) > 0 {
		a.saIndex.Add(1)
		metrics.DriveSARotations.Inc()
	}
}

// NumAccounts returns the number of loaded service accounts (0 for OAuth).
func (a *Auth) NumAccounts() int { return len(a.saFiles) }

// Service returns an authorized Drive service. With service accounts it uses
// the current rotation slot; with OAuth it uses the cached/interactive token.
func (a *Auth) Service(ctx context.Context) (*drive.Service, error) {
	client, err := a.httpClient(ctx)
	if err != nil {
		return nil, err
	}
	return drive.NewService(ctx, option.WithHTTPClient(client))
}

func (a *Auth) httpClient(ctx context.Context) (*http.Client, error) {
	if a.cfg.UseSA {
		idx := int(a.saIndex.Load()) % len(a.saFiles)
		b, err := os.ReadFile(a.saFiles[idx])
		if err != nil {
			return nil, fmt.Errorf("read SA file: %w", err)
		}
		jwtCfg, err := google.JWTConfigFromJSON(b, drive.DriveScope)
		if err != nil {
			return nil, fmt.Errorf("parse SA JWT config: %w", err)
		}
		return jwtCfg.Client(ctx), nil
	}

	b, err := os.ReadFile(a.cfg.CredentialsFile)
	if err != nil {
		return nil, fmt.Errorf("read credentials file: %w", err)
	}
	oauthCfg, err := google.ConfigFromJSON(b, drive.DriveScope)
	if err != nil {
		return nil, fmt.Errorf("parse oauth config: %w", err)
	}
	tok, err := a.token(ctx, oauthCfg)
	if err != nil {
		return nil, err
	}
	return oauthCfg.Client(ctx, tok), nil
}

func (a *Auth) token(ctx context.Context, cfg *oauth2.Config) (*oauth2.Token, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if tok, err := tokenFromFile(a.cfg.TokenFile); err == nil {
		return tok, nil
	}
	// Interactive first-run flow: print URL, read code from stdin.
	authURL := cfg.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Open this URL to authorize Google Drive, then paste the code:\n%v\n", authURL)
	var code string
	if _, err := fmt.Scan(&code); err != nil {
		return nil, fmt.Errorf("read auth code: %w", err)
	}
	tok, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange auth code: %w", err)
	}
	saveToken(a.cfg.TokenFile, tok)
	return tok, nil
}

func tokenFromFile(path string) (*oauth2.Token, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	return tok, json.NewDecoder(f).Decode(tok)
}

func saveToken(path string, token *oauth2.Token) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_ = json.NewEncoder(f).Encode(token)
}
