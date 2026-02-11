package tests

import (
	"bytes"
	"testing"
)

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
			client := newClientForCase(t, tc.name, 1024)

			switch tc.kind {
			case kindString:
				if err := client.Set(tc.key, tc.firstString, 0); err != nil {
					t.Fatalf("Set(first) error=%v", err)
				}
				if err := client.Set(tc.key, tc.secondString, 0); err != nil {
					t.Fatalf("Set(second) error=%v", err)
				}

				var out string
				ok, err := client.Get(tc.key, &out)
				if err != nil || !ok {
					t.Fatalf("Get ok=%v err=%v", ok, err)
				}
				if out != tc.secondString {
					t.Fatalf("Get value=%q want=%q", out, tc.secondString)
				}
			case kindBytes:
				if err := client.Set(tc.key, tc.firstBytes, 0); err != nil {
					t.Fatalf("Set(first) error=%v", err)
				}
				if err := client.Set(tc.key, tc.secondBytes, 0); err != nil {
					t.Fatalf("Set(second) error=%v", err)
				}

				var out []byte
				ok, err := client.Get(tc.key, &out)
				if err != nil || !ok {
					t.Fatalf("Get ok=%v err=%v", ok, err)
				}
				if !bytes.Equal(out, tc.secondBytes) {
					t.Fatalf("Get value=%q want=%q", string(out), string(tc.secondBytes))
				}
			case kindStruct:
				if err := client.Set(tc.key, tc.firstStruct, 0); err != nil {
					t.Fatalf("Set(first) error=%v", err)
				}
				if err := client.Set(tc.key, tc.secondStruct, 0); err != nil {
					t.Fatalf("Set(second) error=%v", err)
				}

				var out sampleValue
				ok, err := client.Get(tc.key, &out)
				if err != nil || !ok {
					t.Fatalf("Get ok=%v err=%v", ok, err)
				}
				if out != tc.secondStruct {
					t.Fatalf("Get value=%+v want=%+v", out, tc.secondStruct)
				}
			default:
				t.Fatalf("unknown kind=%d", tc.kind)
			}
		})
	}
}
