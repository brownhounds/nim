package tests

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func caseRootPath(t *testing.T, caseName string) string {
	t.Helper()

	safeName := strings.NewReplacer("/", "_", " ", "_").Replace(caseName)
	return filepath.Join(testCacheRootDir(t), safeName)
}

func cacheKeyDir(rootPath, key string) string {
	parts := strings.Split(key, "::")
	return filepath.Join(append([]string{rootPath}, parts...)...)
}

func listSymlinkNames(t *testing.T, dirPath string) []string {
	t.Helper()

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		t.Fatalf("ReadDir(%s) error=%v", dirPath, err)
	}

	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			out = append(out, entry.Name())
		}
	}

	return out
}

func assertFilesystemSetLayout(
	t *testing.T,
	rootPath string,
	key string,
	wantSymlink bool,
	wantSymlinkName bool,
) {
	t.Helper()

	dirPath := cacheKeyDir(rootPath, key)
	cachePath := filepath.Join(dirPath, "cache")
	info, err := os.Stat(cachePath)
	if err != nil {
		t.Fatalf("Stat(cache) error=%v", err)
	}
	if info.IsDir() {
		t.Fatalf("cache path is dir, want file")
	}

	symlinks := listSymlinkNames(t, dirPath)
	if wantSymlink && len(symlinks) != 1 {
		t.Fatalf("symlink count=%d want=1 names=%v", len(symlinks), symlinks)
	}
	if !wantSymlink && len(symlinks) != 0 {
		t.Fatalf("symlink count=%d want=0 names=%v", len(symlinks), symlinks)
	}

	if len(symlinks) == 1 {
		target, readErr := os.Readlink(filepath.Join(dirPath, symlinks[0]))
		if readErr != nil {
			t.Fatalf("Readlink error=%v", readErr)
		}
		if target != "cache" {
			t.Fatalf("Readlink target=%q want=%q", target, "cache")
		}
	}

	if wantSymlinkName && len(symlinks) == 1 {
		if _, parseErr := strconv.ParseInt(symlinks[0], 10, 64); parseErr != nil {
			t.Fatalf("symlink name=%q parseErr=%v", symlinks[0], parseErr)
		}
	}
}

func TestFilesystemSetLayoutTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		key             string
		firstTTL        time.Duration
		secondTTL       time.Duration
		runSecondSet    bool
		wantSymlink     bool
		wantSymlinkName bool
	}{
		{
			name:            "ttl zero keeps cache file and no symlink",
			key:             "fs::layout::zero",
			firstTTL:        0,
			wantSymlink:     false,
			wantSymlinkName: false,
		},
		{
			name:            "positive ttl creates expiry symlink",
			key:             "fs::layout::positive",
			firstTTL:        time.Second,
			wantSymlink:     true,
			wantSymlinkName: true,
		},
		{
			name:            "setting ttl zero after positive removes symlink",
			key:             "fs::layout::reset",
			firstTTL:        time.Second,
			secondTTL:       0,
			runSecondSet:    true,
			wantSymlink:     false,
			wantSymlinkName: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newClientForCase(t, tc.name, 1024)
			rootPath := caseRootPath(t, tc.name)

			if err := client.Set(tc.key, "payload", tc.firstTTL); err != nil {
				t.Fatalf("Set(first) error=%v", err)
			}

			if tc.runSecondSet {
				if err := client.Set(tc.key, "payload-2", tc.secondTTL); err != nil {
					t.Fatalf("Set(second) error=%v", err)
				}
			}
			assertFilesystemSetLayout(t, rootPath, tc.key, tc.wantSymlink, tc.wantSymlinkName)
		})
	}
}

func TestFilesystemSymlinkHandlingTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name              string
		key               string
		addTempTTLLink    bool
		addInvalidTTLLink bool
		addExpiredTTLLink bool
		wantExists        bool
		wantDirExists     bool
	}{
		{
			name:           "temp ttl symlink is ignored",
			key:            "fs::symlink::temp",
			addTempTTLLink: true,
			wantExists:     true,
			wantDirExists:  true,
		},
		{
			name:              "invalid symlink name is ignored",
			key:               "fs::symlink::invalid",
			addInvalidTTLLink: true,
			wantExists:        true,
			wantDirExists:     true,
		},
		{
			name:              "expired symlink invalidates cache and removes dir",
			key:               "fs::symlink::expired",
			addExpiredTTLLink: true,
			wantExists:        false,
			wantDirExists:     false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newClientForCase(t, tc.name, 1024)
			rootPath := caseRootPath(t, tc.name)

			if err := client.Set(tc.key, "payload", 0); err != nil {
				t.Fatalf("Set error=%v", err)
			}

			dirPath := cacheKeyDir(rootPath, tc.key)

			if tc.addTempTTLLink {
				future := strconv.FormatInt(time.Now().Add(time.Hour).UnixNano(), 10)
				tmpName := "ttl-temp-" + future
				if err := os.Symlink("cache", filepath.Join(dirPath, tmpName)); err != nil {
					t.Fatalf("Symlink(temp) error=%v", err)
				}
			}

			if tc.addInvalidTTLLink {
				if err := os.Symlink("cache", filepath.Join(dirPath, "not-a-timestamp")); err != nil {
					t.Fatalf("Symlink(invalid) error=%v", err)
				}
			}

			if tc.addExpiredTTLLink {
				expired := strconv.FormatInt(time.Now().Add(-time.Minute).UnixNano(), 10)
				if err := os.Symlink("cache", filepath.Join(dirPath, expired)); err != nil {
					t.Fatalf("Symlink(expired) error=%v", err)
				}
			}

			exists, err := client.Exists(tc.key)
			if err != nil {
				t.Fatalf("Exists error=%v", err)
			}
			if exists != tc.wantExists {
				t.Fatalf("Exists=%v want=%v", exists, tc.wantExists)
			}

			_, statErr := os.Stat(dirPath)
			dirExists := statErr == nil
			if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("Stat(dir) error=%v", statErr)
			}
			if dirExists != tc.wantDirExists {
				t.Fatalf("dir exists=%v want=%v", dirExists, tc.wantDirExists)
			}
		})
	}
}
