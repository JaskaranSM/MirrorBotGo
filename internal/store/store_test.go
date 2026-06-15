package store

import (
	"context"
	"testing"
)

func TestStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, err := Open("sqlite", "file:roundtrip?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if ok, _ := s.IsUserAuthorized(ctx, 42); ok {
		t.Fatal("user 42 should not be authorized initially")
	}
	if err := s.AddAuthorizedUser(ctx, 42); err != nil {
		t.Fatalf("add user: %v", err)
	}
	if err := s.AddAuthorizedUser(ctx, 42); err != nil {
		t.Fatalf("add user idempotent: %v", err)
	}
	if ok, _ := s.IsUserAuthorized(ctx, 42); !ok {
		t.Fatal("user 42 should be authorized")
	}
	users, err := s.ListAuthorizedUsers(ctx)
	if err != nil || len(users) != 1 || users[0] != 42 {
		t.Fatalf("list users = %v err=%v", users, err)
	}
	if err := s.RemoveAuthorizedUser(ctx, 42); err != nil {
		t.Fatalf("remove user: %v", err)
	}
	if ok, _ := s.IsUserAuthorized(ctx, 42); ok {
		t.Fatal("user 42 should be removed")
	}

	if err := s.AddAuthorizedChat(ctx, -100); err != nil {
		t.Fatalf("add chat: %v", err)
	}
	if ok, _ := s.IsChatAuthorized(ctx, -100); !ok {
		t.Fatal("chat -100 should be authorized")
	}

	if err := s.SetSetting(ctx, "gotd_threads", "4"); err != nil {
		t.Fatalf("set setting: %v", err)
	}
	if err := s.SetSetting(ctx, "gotd_threads", "8"); err != nil {
		t.Fatalf("upsert setting: %v", err)
	}
	v, err := s.GetSetting(ctx, "gotd_threads")
	if err != nil || v != "8" {
		t.Fatalf("get setting = %q err=%v", v, err)
	}
	if v, _ := s.GetSetting(ctx, "missing"); v != "" {
		t.Fatalf("missing setting should be empty, got %q", v)
	}
}
