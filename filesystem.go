package nim

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func getBytes(c *Client, key string) (data []byte, ok bool, err error) {
	ok, err = c.Exists(key)
	if err != nil || !ok {
		return nil, ok, err
	}

	dirPath, err := c.keyDir(key)
	if err != nil {
		return nil, false, err
	}

	cachePath := filepath.Join(dirPath, cacheFileName)
	b, err := os.ReadFile(cachePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}

	return b, true, nil
}

func setBytes(c *Client, key string, ttl time.Duration, data []byte) error {
	if err := c.validateCacheSize(len(data)); err != nil {
		return err
	}

	dirPath, err := c.keyDir(key)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dirPath), 0o755); err != nil {
		return err
	}

	lock, err := c.lockKey(dirPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = lock.unlock()
	}()

	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dirPath, cacheTempPattern)
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	cachePath := filepath.Join(dirPath, cacheFileName)
	if err := os.Rename(tmpPath, cachePath); err != nil {
		return err
	}

	return c.writeTTLSymlink(dirPath, ttl)
}

func (c *Client) keyDir(key string) (string, error) {
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	parts, err := SplitKey(key)
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{c.rootPath}, parts...)...), nil
}

func (c *Client) isExpired(dirPath string) (bool, error) {
	expiry, ok, err := readExpiryFromSymlink(dirPath)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}

	return time.Now().After(expiry), nil
}

func (c *Client) writeTTLSymlink(dirPath string, ttl time.Duration) error {
	if ttl <= 0 {
		return c.removeTTLSymlinks(dirPath)
	}

	if err := c.removeTTLSymlinks(dirPath); err != nil {
		return err
	}

	expiry := time.Now().Add(ttl).UnixNano()
	finalName := strconv.FormatInt(expiry, 10)
	finalPath := filepath.Join(dirPath, finalName)
	tmpName := cacheTTLTempPref + finalName
	tmpPath := filepath.Join(dirPath, tmpName)

	if err := os.Symlink(cacheFileName, tmpPath); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	return nil
}

func (c *Client) validateCacheSize(dataLen int) error {
	if dataLen > c.maxBytes {
		return fmt.Errorf("%w: got %d bytes, max %d bytes", ErrCacheValueTooLarge, dataLen, c.maxBytes)
	}
	return nil
}

func (c *Client) removeTTLSymlinks(dirPath string) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			_ = os.Remove(filepath.Join(dirPath, entry.Name()))
		}
	}
	return nil
}

func readExpiryFromSymlink(dirPath string) (time.Time, bool, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink == 0 {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, cacheTTLTempPref) {
			continue
		}
		nanos, err := strconv.ParseInt(name, 10, 64)
		if err != nil {
			continue
		}
		return time.Unix(0, nanos), true, nil
	}

	return time.Time{}, false, nil
}

type keyLock struct {
	file *os.File
}

func (c *Client) lockKey(dirPath string) (*keyLock, error) {
	lockPath := dirPath + cacheLockSuffix
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &keyLock{file: f}, nil
}

func (l *keyLock) unlock() error {
	if l == nil || l.file == nil {
		return nil
	}
	_ = syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	return l.file.Close()
}
