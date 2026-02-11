package tests

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/brownhounds/nim"
)

type benchUser struct {
	Name   string
	ID     int
	Active bool
}

type benchProfile struct {
	Name    string
	Email   string
	Padding []byte
	Age     int
}

var (
	benchSinkBytes  []byte
	benchSinkString string
	benchSinkBool   bool
	benchSinkUser   benchUser
)

func newBenchmarkClient(b *testing.B, maxBytes int) *nim.Client {
	b.Helper()

	client, err := nim.New(nim.Config{
		RootPath: b.TempDir(),
		MaxBytes: maxBytes,
	})
	if err != nil {
		b.Fatalf("New error=%v", err)
	}

	return client
}

func BenchmarkCacheSetTable(b *testing.B) {
	const (
		setKindBytes = iota
		setKindString
		setKindStruct
	)

	cases := []struct {
		name    string
		size    int
		setKind int
	}{
		{name: "bytes_128b", size: 128, setKind: setKindBytes},
		{name: "bytes_4kb", size: 4 * 1024, setKind: setKindBytes},
		{name: "bytes_64kb", size: 64 * 1024, setKind: setKindBytes},
		{name: "string_128b", size: 128, setKind: setKindString},
		{name: "string_4kb", size: 4 * 1024, setKind: setKindString},
		{name: "struct_256b", size: 256, setKind: setKindStruct},
		{name: "struct_4kb", size: 4 * 1024, setKind: setKindStruct},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			client := newBenchmarkClient(b, tc.size*4+1024)
			key := "bench::set::value"
			payload := bytes.Repeat([]byte("a"), tc.size)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				switch tc.setKind {
				case setKindString:
					if err := client.Set(key, string(payload), 0); err != nil {
						b.Fatalf("Set(string) error=%v", err)
					}
				case setKindBytes:
					if err := client.Set(key, payload, 0); err != nil {
						b.Fatalf("Set(bytes) error=%v", err)
					}
				case setKindStruct:
					profile := benchProfile{
						Name:    "bench-user",
						Email:   "bench@example.com",
						Padding: payload,
						Age:     42,
					}
					if err := client.Set(key, profile, 0); err != nil {
						b.Fatalf("Set(struct) error=%v", err)
					}
				default:
					b.Fatalf("unknown set kind %d", tc.setKind)
				}
			}
		})
	}
}

func BenchmarkCacheGetTable(b *testing.B) {
	cases := []struct {
		setupAny any
		name     string
		key      string
		getKind  int
	}{
		{name: "get_bytes_hit", key: "bench::get::bytes", setupAny: []byte("payload-bytes"), getKind: 0},
		{name: "get_string_hit", key: "bench::get::string", setupAny: "payload-string", getKind: 1},
		{name: "get_struct_hit", key: "bench::get::struct", setupAny: benchUser{ID: 7, Name: "bench", Active: true}, getKind: 2},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			client := newBenchmarkClient(b, 1024*1024)
			if err := client.Set(tc.key, tc.setupAny, 0); err != nil {
				b.Fatalf("Set(setup) error=%v", err)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				switch tc.getKind {
				case 0:
					var out []byte
					ok, err := client.Get(tc.key, &out)
					if err != nil || !ok {
						b.Fatalf("Get(bytes) ok=%v err=%v", ok, err)
					}
					benchSinkBytes = out
				case 1:
					var out string
					ok, err := client.Get(tc.key, &out)
					if err != nil || !ok {
						b.Fatalf("Get(string) ok=%v err=%v", ok, err)
					}
					benchSinkString = out
				case 2:
					var out benchUser
					ok, err := client.Get(tc.key, &out)
					if err != nil || !ok {
						b.Fatalf("Get(struct) ok=%v err=%v", ok, err)
					}
					benchSinkUser = out
				default:
					b.Fatalf("unknown get kind %d", tc.getKind)
				}
			}
		})
	}
}

func BenchmarkCacheExistsTable(b *testing.B) {
	cases := []struct {
		name     string
		key      string
		seedHit  bool
		maxBytes int
	}{
		{name: "exists_hit", key: "bench::exists::hit", seedHit: true, maxBytes: 1024},
		{name: "exists_miss", key: "bench::exists::miss", seedHit: false, maxBytes: 1024},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			client := newBenchmarkClient(b, tc.maxBytes)
			if tc.seedHit {
				if err := client.Set(tc.key, "value", 0); err != nil {
					b.Fatalf("Set(setup) error=%v", err)
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ok, err := client.Exists(tc.key)
				if err != nil {
					b.Fatalf("Exists error=%v", err)
				}
				benchSinkBool = ok
			}
		})
	}
}

func BenchmarkCacheRemoveTable(b *testing.B) {
	cases := []struct {
		name string
	}{
		{name: "remove_hit"},
		{name: "remove_miss"},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			client := newBenchmarkClient(b, 1024)

			switch tc.name {
			case "remove_hit":
				for i := 0; i < b.N; i++ {
					key := fmt.Sprintf("bench::remove::hit::%d", i)
					if err := client.Set(key, "value", 0); err != nil {
						b.Fatalf("Set(setup) error=%v", err)
					}
					b.StartTimer()
					err := client.Remove(key)
					b.StopTimer()
					if err != nil {
						b.Fatalf("Remove error=%v", err)
					}
				}
			case "remove_miss":
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					key := fmt.Sprintf("bench::remove::miss::%d", i)
					if err := client.Remove(key); err != nil {
						b.Fatalf("Remove error=%v", err)
					}
				}
			default:
				b.Fatalf("unknown benchmark case %q", tc.name)
			}
		})
	}
}

func BenchmarkCacheSetParallelSameKey(b *testing.B) {
	client := newBenchmarkClient(b, 1024*1024)
	key := "bench::parallel::set::same-key"
	payload := []byte("parallel-payload")

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := client.Set(key, payload, 0); err != nil {
				b.Fatalf("Set(parallel) error=%v", err)
			}
		}
	})
}
