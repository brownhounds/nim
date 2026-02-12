package tests

import (
	"bytes"
	"testing"
)

func assertLastWriterWinsString(t *testing.T, clientKey, first, second string) {
	t.Helper()

	client := newClientForCase(t, t.Name(), 1024)
	if err := client.Set(clientKey, first, 0); err != nil {
		t.Fatalf("Set(first) error=%v", err)
	}
	if err := client.Set(clientKey, second, 0); err != nil {
		t.Fatalf("Set(second) error=%v", err)
	}

	var out string
	ok, err := client.Get(clientKey, &out)
	if err != nil || !ok {
		t.Fatalf("Get ok=%v err=%v", ok, err)
	}
	if out != second {
		t.Fatalf("Get value=%q want=%q", out, second)
	}
}

func assertLastWriterWinsBytes(t *testing.T, clientKey string, first, second []byte) {
	t.Helper()

	client := newClientForCase(t, t.Name(), 1024)
	if err := client.Set(clientKey, first, 0); err != nil {
		t.Fatalf("Set(first) error=%v", err)
	}
	if err := client.Set(clientKey, second, 0); err != nil {
		t.Fatalf("Set(second) error=%v", err)
	}

	var out []byte
	ok, err := client.Get(clientKey, &out)
	if err != nil || !ok {
		t.Fatalf("Get ok=%v err=%v", ok, err)
	}
	if !bytes.Equal(out, second) {
		t.Fatalf("Get value=%q want=%q", string(out), string(second))
	}
}

func assertLastWriterWinsStruct(t *testing.T, clientKey string, first, second sampleValue) {
	t.Helper()

	client := newClientForCase(t, t.Name(), 1024)
	if err := client.Set(clientKey, first, 0); err != nil {
		t.Fatalf("Set(first) error=%v", err)
	}
	if err := client.Set(clientKey, second, 0); err != nil {
		t.Fatalf("Set(second) error=%v", err)
	}

	var out sampleValue
	ok, err := client.Get(clientKey, &out)
	if err != nil || !ok {
		t.Fatalf("Get ok=%v err=%v", ok, err)
	}
	if out != second {
		t.Fatalf("Get value=%+v want=%+v", out, second)
	}
}

func TestLastWriterWinsTable(t *testing.T) {
	t.Parallel()

	const (
		kindString = iota
		kindBytes
		kindStruct
	)

	cases := []struct {
		firstString  string
		secondString string
		name         string
		key          string
		firstBytes   []byte
		secondBytes  []byte
		firstStruct  sampleValue
		secondStruct sampleValue
		kind         int
	}{
		{
			name:         "string last write wins",
			key:          "lww::string",
			firstString:  "first",
			secondString: "second",
			kind:         kindString,
		},
		{
			name:        "bytes last write wins",
			key:         "lww::bytes",
			firstBytes:  []byte("first-bytes"),
			secondBytes: []byte("second-bytes"),
			kind:        kindBytes,
		},
		{
			name:         "struct last write wins",
			key:          "lww::struct",
			firstStruct:  sampleValue{Name: "first", Count: 1},
			secondStruct: sampleValue{Name: "second", Count: 2},
			kind:         kindStruct,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			switch tc.kind {
			case kindString:
				assertLastWriterWinsString(t, tc.key, tc.firstString, tc.secondString)
			case kindBytes:
				assertLastWriterWinsBytes(t, tc.key, tc.firstBytes, tc.secondBytes)
			case kindStruct:
				assertLastWriterWinsStruct(t, tc.key, tc.firstStruct, tc.secondStruct)
			default:
				t.Fatalf("unknown kind=%d", tc.kind)
			}
		})
	}
}
