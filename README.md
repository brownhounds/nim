[![Go Reference](https://pkg.go.dev/badge/github.com/brownhounds/nim.svg)](https://pkg.go.dev/github.com/brownhounds/nim)
[![CI](https://github.com/brownhounds/nim/actions/workflows/ci.yml/badge.svg)](https://github.com/brownhounds/nim/actions/workflows/ci.yml)
[![Release](https://github.com/brownhounds/nim/actions/workflows/release.yml/badge.svg)](https://github.com/brownhounds/nim/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/brownhounds/nim)](https://goreportcard.com/report/github.com/brownhounds/nim)
[![Latest Release](https://img.shields.io/github/v/release/brownhounds/nim)](https://github.com/brownhounds/nim/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/brownhounds/nim)](https://github.com/brownhounds/nim/blob/main/go.mod)
[![License](https://img.shields.io/github/license/brownhounds/nim)](https://github.com/brownhounds/nim/blob/main/LICENSE)

# nim (not in memory)

Simple file-backed key/value cache for Go.

It stores values on disk, supports TTL expiration, and is safe for concurrent access with per-key file locks.

Note: Values are read/written as whole byte payloads, so this is not optimized for very large payloads.

## Installation

```bash
go get github.com/brownhounds/nim
```

## Usage

### Client

```go
client, err := nim.New(nim.Config{
	RootPath: "./.cache",
	MaxBytes: 10 * 1024 * 1024, // optional
})

```

### Operations

```go
// []byte
err = client.Set("bin::payload", []byte("hello-bytes"), time.Minute)
var b []byte
ok, err := client.Get("bin::payload", &b)
exists, err := client.Exists("bin::payload")
err = client.Remove("bin::payload")
```

```go
// string
err = client.Set("str::greeting", "hello", time.Minute)
var s string
ok, err := client.Get("str::greeting", &s)
exists, err := client.Exists("str::greeting")
err = client.Remove("str::greeting")
```

```go
// struct
type User struct {
	ID   int
	Name string
}

err = client.Set("user::1", User{ID: 1, Name: "Alice"}, time.Minute)
var u User
ok, err := client.Get("user::1", &u)
exists, err := client.Exists("user::1")
err = client.Remove("user::1")
```

`Get` performs existence and TTL checks internally before reading cache file bytes.

## Features

- File-backed cache (not in-memory)
- Stores `string`, `[]byte`, and `structs`
- Automatic serialization/deserialization for structs
- Atomic writes (temp file + rename)
- TTL expiration per key
- Namespace-style keys with `::` segments
- Concurrent-safe per-key operations via file locks

## How It Works

Keys are split by `::` and mapped to nested directories under `RootPath`, so a key like `user::123::profile` becomes a deterministic path on disk. Each key directory stores its value in a `cache` file as binary. Strings and raw bytes are written directly, and structs are serialized before being written.

TTL is tracked with symlinks in the key directory. The symlink name is a Unix-nano expiry timestamp, and the symlink target points to `cache`. TTL is resolved from filesystem metadata (`stat`/directory entries), so the cache can decide expiry without reading cache file bytes.

## Concurrency

Writes are lock-protected per key to avoid partial/corrupt data writes.

If multiple writers concurrently write different values to the same key, behavior is **last-writer-wins**.  
The final stored value is whichever write acquires the key lock last.

## Benchmarks

CPU: AMD Ryzen 9 7950X 16-Core Processor

| Benchmark | Iterations | ns/op | B/op | allocs/op |
|---|---:|---:|---:|---:|
| `BenchmarkCacheSetTable/bytes_128b-32` | 2661 | 447047 | 2633 | 36 |
| `BenchmarkCacheSetTable/bytes_4kb-32` | 2647 | 448795 | 2640 | 36 |
| `BenchmarkCacheSetTable/bytes_64kb-32` | 2199 | 540923 | 2643 | 36 |
| `BenchmarkCacheSetTable/string_128b-32` | 2613 | 448058 | 2895 | 38 |
| `BenchmarkCacheSetTable/string_4kb-32` | 2749 | 454486 | 10872 | 38 |
| `BenchmarkCacheGetTable/get_bytes_hit-32` | 102332 | 11598 | 2259 | 27 |
| `BenchmarkCacheGetTable/get_string_hit-32` | 100696 | 11728 | 2267 | 28 |
| `BenchmarkCacheGetTable/get_struct_hit-32` | 58972 | 20338 | 9363 | 186 |
| `BenchmarkCacheExistsTable/exists_hit-32` | 197569 | 5900 | 960 | 15 |
| `BenchmarkCacheExistsTable/exists_miss-32` | 984931 | 1249 | 704 | 9 |
| `BenchmarkCacheRemoveTable/remove_hit-32` | 22923 | 52384 | 1535 | 27 |
| `BenchmarkCacheRemoveTable/remove_miss-32` | 934452 | 1383 | 729 | 10 |
| `BenchmarkCacheSetParallelSameKey-32` | 2419 | 487504 | 3012 | 36 |
