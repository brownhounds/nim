package tests

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/brownhounds/nim"
)

func TestNewClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr   error
		name      string
		cfg       nim.Config
		wantExist bool
	}{
		{
			name:    "empty root path",
			cfg:     nim.Config{},
			wantErr: nim.ErrCacheRootPathEmpty,
		},
		{
			name:      "creates root path",
			cfg:       nim.Config{RootPath: "tmp-cache"},
			wantExist: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.cfg
			if cfg.RootPath != "" && !filepath.IsAbs(cfg.RootPath) {
				cfg.RootPath = filepath.Join(t.TempDir(), cfg.RootPath)
			}

			client, err := nim.New(cfg)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("New(%+v) error=%v wantErr=%v", cfg, err, tc.wantErr)
				}
				if client != nil {
					t.Fatalf("New(%+v) client=%v want nil", cfg, client)
				}
				return
			}

			if err != nil {
				t.Fatalf("New(%+v) unexpected error=%v", cfg, err)
			}
			if client == nil {
				t.Fatalf("New(%+v) returned nil client", cfg)
			}
			if tc.wantExist {
				info, statErr := os.Stat(cfg.RootPath)
				if statErr != nil {
					t.Fatalf("expected root path to exist, stat error=%v", statErr)
				}
				if !info.IsDir() {
					t.Fatalf("expected root path to be dir, got file")
				}
			}
		})
	}
}

func TestSetMaxSize(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr  error
		name     string
		maxBytes int
		dataLen  int
	}{
		{
			name:     "exceeds max bytes",
			maxBytes: 2,
			dataLen:  3,
			wantErr:  nim.ErrCacheValueTooLarge,
		},
		{
			name:     "within max bytes",
			maxBytes: 3,
			dataLen:  3,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rootPath := t.TempDir()
			client, err := nim.New(nim.Config{
				RootPath: rootPath,
				MaxBytes: tc.maxBytes,
			})
			if err != nil {
				t.Fatalf("New error=%v", err)
			}

			data := make([]byte, tc.dataLen)
			err = client.Set("size::limit", data, 0)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Set error=%v wantErr=%v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Set unexpected error=%v", err)
			}
		})
	}
}
