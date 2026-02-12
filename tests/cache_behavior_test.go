package tests

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/brownhounds/nim"
)

type sampleValue struct {
	Name  string
	Count int
}

func testCacheRootDir(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Join(filepath.Dir(thisFile), ".cache")
}

func newClientForCase(t *testing.T, caseName string, maxBytes int) *nim.Client {
	t.Helper()

	safeName := strings.NewReplacer("/", "_", " ", "_").Replace(caseName)
	rootPath := filepath.Join(testCacheRootDir(t), safeName)
	_ = os.RemoveAll(rootPath)
	t.Cleanup(func() {
		_ = os.RemoveAll(rootPath)
	})

	client, err := nim.New(nim.Config{
		RootPath: rootPath,
		MaxBytes: maxBytes,
	})
	if err != nil {
		t.Fatalf("New error=%v", err)
	}

	return client
}

func runSetGetCase(
	t *testing.T,
	client *nim.Client,
	valueKind int,
	stringVal string,
	bytesVal []byte,
	structVal sampleValue,
) {
	t.Helper()

	switch valueKind {
	case 0:
		if err := client.Set("cache::item", stringVal, 0); err != nil {
			t.Fatalf("Set error=%v", err)
		}
		var out string
		ok, err := client.Get("cache::item", &out)
		if err != nil {
			t.Fatalf("Get error=%v", err)
		}
		if !ok {
			t.Fatalf("Get ok=%v want=true", ok)
		}
		if out != stringVal {
			t.Fatalf("Get value=%q want=%q", out, stringVal)
		}
	case 1:
		if err := client.Set("cache::item", bytesVal, 0); err != nil {
			t.Fatalf("Set error=%v", err)
		}
		var out []byte
		ok, err := client.Get("cache::item", &out)
		if err != nil {
			t.Fatalf("Get error=%v", err)
		}
		if !ok {
			t.Fatalf("Get ok=%v want=true", ok)
		}
		if !bytes.Equal(out, bytesVal) {
			t.Fatalf("Get value=%q want=%q", string(out), string(bytesVal))
		}
	case 2:
		if err := client.Set("cache::item", structVal, 0); err != nil {
			t.Fatalf("Set error=%v", err)
		}
		var out sampleValue
		ok, err := client.Get("cache::item", &out)
		if err != nil {
			t.Fatalf("Get error=%v", err)
		}
		if !ok {
			t.Fatalf("Get ok=%v want=true", ok)
		}
		if out != structVal {
			t.Fatalf("Get value=%+v want=%+v", out, structVal)
		}
	default:
		t.Fatalf("unknown value kind=%d", valueKind)
	}
}

func assertGetStringValue(t *testing.T, client *nim.Client, key, want string) {
	t.Helper()

	var out string
	ok, err := client.Get(key, &out)
	if err != nil {
		t.Fatalf("Get error=%v", err)
	}
	if !ok {
		t.Fatalf("Get ok=%v want=true", ok)
	}
	if out != want {
		t.Fatalf("Get value=%q want=%q", out, want)
	}
}

func TestCacheSetGetTable(t *testing.T) {
	t.Parallel()

	const (
		valueKindString = iota
		valueKindBytes
		valueKindStruct
	)

	cases := []struct {
		stringVal string
		bytesVal  []byte
		name      string
		structVal sampleValue
		valueKind int
		maxBytes  int
	}{
		{
			name:      "set get string",
			valueKind: valueKindString,
			stringVal: "hello-world",
			maxBytes:  1024,
		},
		{
			name:      "set get bytes",
			valueKind: valueKindBytes,
			bytesVal:  []byte("hello-bytes"),
			maxBytes:  1024,
		},
		{
			name:      "set get struct",
			valueKind: valueKindStruct,
			structVal: sampleValue{Name: "alpha", Count: 7},
			maxBytes:  1024,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newClientForCase(t, tc.name, tc.maxBytes)
			runSetGetCase(t, client, tc.valueKind, tc.stringVal, tc.bytesVal, tc.structVal)
		})
	}
}

func TestCacheSetErrorTable(t *testing.T) {
	t.Parallel()

	const (
		valueModeString = iota
		valueModeBytes
	)
	const (
		wantErrNone = iota
		wantErrKeyEmpty
		wantErrKeySegment
		wantErrTooLarge
	)

	cases := []struct {
		name          string
		key           string
		maxBytes      int
		byteValueSize int
		valueMode     int
		wantErrKind   int
	}{
		{
			name:        "empty key",
			key:         "",
			maxBytes:    1024,
			valueMode:   valueModeString,
			wantErrKind: wantErrKeyEmpty,
		},
		{
			name:        "invalid segmented key",
			key:         "bad::::key",
			maxBytes:    1024,
			valueMode:   valueModeString,
			wantErrKind: wantErrKeySegment,
		},
		{
			name:          "max bytes exceeded",
			key:           "cache::item",
			maxBytes:      3,
			byteValueSize: 4,
			valueMode:     valueModeBytes,
			wantErrKind:   wantErrTooLarge,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newClientForCase(t, tc.name, tc.maxBytes)
			var err error
			switch tc.valueMode {
			case valueModeBytes:
				err = client.Set(tc.key, make([]byte, tc.byteValueSize), 0)
			case valueModeString:
				err = client.Set(tc.key, "value", 0)
			default:
				t.Fatalf("unknown value mode=%d", tc.valueMode)
			}

			var wantErr error
			switch tc.wantErrKind {
			case wantErrNone:
				wantErr = nil
			case wantErrKeyEmpty:
				wantErr = nim.ErrCacheKeyEmpty
			case wantErrKeySegment:
				wantErr = nim.ErrCacheKeyEmptySegment
			case wantErrTooLarge:
				wantErr = nim.ErrCacheValueTooLarge
			default:
				t.Fatalf("unknown want error kind=%d", tc.wantErrKind)
			}
			if !errors.Is(err, wantErr) {
				t.Fatalf("Set error=%v wantErr=%v", err, wantErr)
			}
		})
	}
}

func TestCacheExistsTTLAndRemoveTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		ttl             time.Duration
		wait            time.Duration
		removeAfterWait bool
		wantExists      bool
	}{
		{
			name:            "ttl zero persists",
			ttl:             0,
			wait:            25 * time.Millisecond,
			removeAfterWait: false,
			wantExists:      true,
		},
		{
			name:            "ttl expires",
			ttl:             15 * time.Millisecond,
			wait:            35 * time.Millisecond,
			removeAfterWait: false,
			wantExists:      false,
		},
		{
			name:            "remove deletes cache",
			ttl:             0,
			wait:            0,
			removeAfterWait: true,
			wantExists:      false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newClientForCase(t, tc.name, 1024)

			if err := client.Set("cache::item", "value", tc.ttl); err != nil {
				t.Fatalf("Set error=%v", err)
			}

			if tc.wait > 0 {
				time.Sleep(tc.wait)
			}

			if tc.removeAfterWait {
				if err := client.Remove("cache::item"); err != nil {
					t.Fatalf("Remove error=%v", err)
				}
			}

			exists, err := client.Exists("cache::item")
			if err != nil {
				t.Fatalf("Exists error=%v", err)
			}
			if exists != tc.wantExists {
				t.Fatalf("Exists=%v want=%v", exists, tc.wantExists)
			}
		})
	}
}

func TestCacheGetMissTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		key       string
		wantFound bool
	}{
		{
			name:      "cache miss",
			key:       "cache::missing",
			wantFound: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newClientForCase(t, tc.name, 1024)
			var out string
			found, err := client.Get(tc.key, &out)
			if err != nil {
				t.Fatalf("Get error=%v wantErr=nil", err)
			}
			if found != tc.wantFound {
				t.Fatalf("Get found=%v want=%v", found, tc.wantFound)
			}
		})
	}
}

func TestCacheGetDecodeFailureTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		key     string
		stored  string
		wantErr bool
	}{
		{
			name:    "get string as struct fails decode",
			key:     "cache::decode::mismatch",
			stored:  "plain-string",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newClientForCase(t, tc.name, 1024)
			if err := client.Set(tc.key, tc.stored, 0); err != nil {
				t.Fatalf("Set error=%v", err)
			}

			var out sampleValue
			ok, err := client.Get(tc.key, &out)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Get error=%v wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr && ok {
				t.Fatalf("Get ok=%v want=false on decode failure", ok)
			}
		})
	}
}

func TestCacheExistsPathIsDirectoryTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		key  string
	}{
		{
			name: "cache path directory returns ErrCachePathIsDir",
			key:  "cache::dir::error",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newClientForCase(t, tc.name, 1024)
			rootPath := caseRootPath(t, tc.name)
			dirPath := cacheKeyDir(rootPath, tc.key)
			cachePath := filepath.Join(dirPath, "cache")

			if err := os.MkdirAll(cachePath, 0o755); err != nil {
				t.Fatalf("MkdirAll(cache path as dir) error=%v", err)
			}

			ok, err := client.Exists(tc.key)
			if !errors.Is(err, nim.ErrCachePathIsDir) {
				t.Fatalf("Exists error=%v wantErr=%v", err, nim.ErrCachePathIsDir)
			}
			if ok {
				t.Fatalf("Exists ok=%v want=false", ok)
			}
		})
	}
}

func TestCacheRemoveMissingIdempotentTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		key  string
	}{
		{
			name: "remove missing key twice returns nil",
			key:  "cache::remove::missing",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newClientForCase(t, tc.name, 1024)

			if err := client.Remove(tc.key); err != nil {
				t.Fatalf("Remove(first) error=%v want=nil", err)
			}
			if err := client.Remove(tc.key); err != nil {
				t.Fatalf("Remove(second) error=%v want=nil", err)
			}
		})
	}
}

func TestCacheUpdateAndTTLRefreshTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		firstValue       string
		secondValue      string
		name             string
		key              string
		firstTTL         time.Duration
		secondTTL        time.Duration
		waitBeforeUpdate time.Duration
		waitAfterUpdate  time.Duration
		wantFoundAfter   bool
	}{
		{
			name:             "update with zero ttl persists new value",
			key:              "cache::ttl::update::persist",
			firstTTL:         15 * time.Millisecond,
			secondTTL:        0,
			waitBeforeUpdate: 5 * time.Millisecond,
			waitAfterUpdate:  30 * time.Millisecond,
			firstValue:       "v1",
			secondValue:      "v2",
			wantFoundAfter:   true,
		},
		{
			name:             "update with short ttl expires new value",
			key:              "cache::ttl::update::expire",
			firstTTL:         0,
			secondTTL:        20 * time.Millisecond,
			waitBeforeUpdate: 0,
			waitAfterUpdate:  35 * time.Millisecond,
			firstValue:       "base",
			secondValue:      "temp",
			wantFoundAfter:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newClientForCase(t, tc.name, 1024)

			if err := client.Set(tc.key, tc.firstValue, tc.firstTTL); err != nil {
				t.Fatalf("Set(first) error=%v", err)
			}
			assertGetStringValue(t, client, tc.key, tc.firstValue)

			if tc.waitBeforeUpdate > 0 {
				time.Sleep(tc.waitBeforeUpdate)
			}

			if err := client.Set(tc.key, tc.secondValue, tc.secondTTL); err != nil {
				t.Fatalf("Set(second) error=%v", err)
			}
			assertGetStringValue(t, client, tc.key, tc.secondValue)

			if tc.waitAfterUpdate > 0 {
				time.Sleep(tc.waitAfterUpdate)
			}

			var finalOut string
			finalOK, finalErr := client.Get(tc.key, &finalOut)
			if finalErr != nil {
				t.Fatalf("Get(final) error=%v", finalErr)
			}
			if finalOK != tc.wantFoundAfter {
				t.Fatalf("Get(final) ok=%v want=%v", finalOK, tc.wantFoundAfter)
			}
			if finalOK && finalOut != tc.secondValue {
				t.Fatalf("Get(final) value=%q want=%q", finalOut, tc.secondValue)
			}
		})
	}
}
