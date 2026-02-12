package tests

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/brownhounds/nim"
)

var errUnknownOpKind = errors.New("unknown op kind")

type mixedOpsCase struct {
	name          string
	setWorkers    int
	removeWorkers int
	opsPerWorker  int
	seed          bool
}

func launchSetWorkers(
	client *nim.Client,
	key string,
	workers int,
	opsPerWorker int,
	errCh chan<- error,
	panicCh chan<- struct{},
	wg *sync.WaitGroup,
) {
	for worker := 0; worker < workers; worker++ {
		worker := worker
		wg.Go(func() {
			defer func() {
				if recover() != nil {
					panicCh <- struct{}{}
				}
			}()
			for op := 0; op < opsPerWorker; op++ {
				payload := fmt.Sprintf("set-worker-%d-op-%d", worker, op)
				if err := client.Set(key, payload, 0); err != nil {
					errCh <- err
				}
			}
		})
	}
}

func launchRemoveWorkers(
	client *nim.Client,
	key string,
	workers int,
	opsPerWorker int,
	errCh chan<- error,
	panicCh chan<- struct{},
	wg *sync.WaitGroup,
) {
	for worker := 0; worker < workers; worker++ {
		wg.Go(func() {
			defer func() {
				if recover() != nil {
					panicCh <- struct{}{}
				}
			}()
			for op := 0; op < opsPerWorker; op++ {
				if err := client.Remove(key); err != nil {
					errCh <- err
				}
			}
		})
	}
}

func assertNoPanicsOrErrors(t *testing.T, errCh <-chan error, panicCh <-chan struct{}) {
	t.Helper()

	if len(panicCh) > 0 {
		t.Fatalf("mixed operations caused panic in %d goroutine(s)", len(panicCh))
	}
	for err := range errCh {
		if err != nil {
			t.Fatalf("mixed concurrent operation error=%v", err)
		}
	}
}

func assertCoherentFinalState(t *testing.T, client *nim.Client, key string) {
	t.Helper()

	exists, err := client.Exists(key)
	if err != nil {
		t.Fatalf("Exists error=%v", err)
	}

	var out string
	ok, err := client.Get(key, &out)
	if err != nil {
		t.Fatalf("Get error=%v", err)
	}
	if exists {
		if !ok {
			t.Fatalf("Exists=true but Get ok=%v want=true", ok)
		}
		if out == "" {
			t.Fatalf("Exists=true but Get returned empty value")
		}
		return
	}
	if ok {
		t.Fatalf("Exists=false but Get ok=%v want=false", ok)
	}
}

func runMixedOpsCase(t *testing.T, tc mixedOpsCase) {
	t.Helper()

	client := newClientForCase(t, tc.name, 1024)
	key := "lock::mixed::key"

	if tc.seed {
		if err := client.Set(key, "seed", 0); err != nil {
			t.Fatalf("Set(seed) error=%v", err)
		}
	}

	totalOps := (tc.setWorkers + tc.removeWorkers) * tc.opsPerWorker
	errCh := make(chan error, totalOps)
	panicCh := make(chan struct{}, tc.setWorkers+tc.removeWorkers)
	var wg sync.WaitGroup

	launchSetWorkers(client, key, tc.setWorkers, tc.opsPerWorker, errCh, panicCh, &wg)
	launchRemoveWorkers(client, key, tc.removeWorkers, tc.opsPerWorker, errCh, panicCh, &wg)

	wg.Wait()
	close(errCh)
	close(panicCh)

	assertNoPanicsOrErrors(t, errCh, panicCh)
	assertCoherentFinalState(t, client, key)
}

func TestLockContentionTable(t *testing.T) {
	t.Parallel()

	const (
		opSet = iota
		opRemove
	)

	cases := []struct {
		name     string
		opKind   int
		preCache bool
	}{
		{
			name:     "set waits for external flock",
			opKind:   opSet,
			preCache: false,
		},
		{
			name:     "remove waits for external flock",
			opKind:   opRemove,
			preCache: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newClientForCase(t, tc.name, 1024)
			key := "lock::item"

			if tc.preCache {
				if err := client.Set(key, "seed", 0); err != nil {
					t.Fatalf("Set(seed) error=%v", err)
				}
			}

			rootPath := caseRootPath(t, tc.name)
			dirPath := cacheKeyDir(rootPath, key)
			if err := os.MkdirAll(dirPath, 0o755); err != nil {
				t.Fatalf("MkdirAll(%s) error=%v", dirPath, err)
			}

			lockPath := dirPath + ".lock"
			lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
			if err != nil {
				t.Fatalf("OpenFile(lock) error=%v", err)
			}
			defer func() {
				_ = lockFile.Close()
			}()

			if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
				t.Fatalf("Flock(lock) error=%v", err)
			}

			done := make(chan error, 1)
			go func() {
				switch tc.opKind {
				case opSet:
					done <- client.Set(key, "value", 0)
				case opRemove:
					done <- client.Remove(key)
				default:
					done <- errUnknownOpKind
				}
			}()

			select {
			case opErr := <-done:
				t.Fatalf("operation completed while lock held, err=%v", opErr)
			case <-time.After(50 * time.Millisecond):
			}

			if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN); err != nil {
				t.Fatalf("Flock(unlock) error=%v", err)
			}

			select {
			case opErr := <-done:
				if opErr != nil {
					t.Fatalf("operation error=%v", opErr)
				}
			case <-time.After(500 * time.Millisecond):
				t.Fatalf("operation did not complete after lock release")
			}
		})
	}
}

func TestLockConcurrentSetTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		workers int
		writes  int
	}{
		{
			name:    "many writers one key",
			workers: 8,
			writes:  20,
		},
		{
			name:    "few writers one key",
			workers: 3,
			writes:  15,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newClientForCase(t, tc.name, 1024)
			key := "lock::contention::key"

			errCh := make(chan error, tc.workers*tc.writes)
			var wg sync.WaitGroup

			for worker := 0; worker < tc.workers; worker++ {
				worker := worker
				wg.Go(func() {
					for write := 0; write < tc.writes; write++ {
						payload := fmt.Sprintf("worker-%d-write-%d", worker, write)
						if err := client.Set(key, payload, 0); err != nil {
							errCh <- err
						}
					}
				})
			}

			wg.Wait()
			close(errCh)

			for err := range errCh {
				if err != nil {
					t.Fatalf("concurrent Set error=%v", err)
				}
			}

			var out string
			ok, err := client.Get(key, &out)
			if err != nil {
				t.Fatalf("Get error=%v", err)
			}
			if !ok {
				t.Fatalf("Get ok=%v want=true", ok)
			}
			if out == "" {
				t.Fatalf("Get value empty after concurrent writes")
			}
		})
	}
}

func TestLockConcurrentMixedOpsTable(t *testing.T) {
	t.Parallel()

	cases := []mixedOpsCase{
		{
			name:          "balanced set remove",
			setWorkers:    4,
			removeWorkers: 4,
			opsPerWorker:  30,
			seed:          true,
		},
		{
			name:          "set heavy",
			setWorkers:    8,
			removeWorkers: 2,
			opsPerWorker:  25,
			seed:          false,
		},
		{
			name:          "remove heavy",
			setWorkers:    2,
			removeWorkers: 8,
			opsPerWorker:  25,
			seed:          true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runMixedOpsCase(t, tc)
		})
	}
}
