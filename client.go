package nim

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Client struct {
	rootPath string
	maxBytes int
}

type Config struct {
	RootPath string
	MaxBytes int
}

func New(cfg Config) (*Client, error) {
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = defaultMaxCacheBytes
	}
	if cfg.RootPath == "" {
		return nil, ErrCacheRootPathEmpty
	}

	if err := os.MkdirAll(cfg.RootPath, 0o755); err != nil {
		return nil, err
	}

	return &Client{rootPath: cfg.RootPath, maxBytes: cfg.MaxBytes}, nil
}

func (c *Client) Set(key string, v any, ttl time.Duration) error {
	switch val := v.(type) {
	case []byte:
		return setBytes(c, key, ttl, val)
	case string:
		return setBytes(c, key, ttl, []byte(val))
	default:
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(v); err != nil {
			return fmt.Errorf("failed to encode value for Set: %w", err)
		}
		return setBytes(c, key, ttl, buf.Bytes())
	}
}

func (c *Client) Remove(key string) error {
	dirPath, err := c.keyDir(key)
	if err != nil {
		return err
	}

	if _, err := os.Stat(dirPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	lock, err := c.lockKey(dirPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer func() {
		_ = lock.unlock()
	}()

	if err := os.RemoveAll(dirPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}

func (c *Client) Get(key string, out any) (bool, error) {
	b, ok, err := getBytes(c, key)
	if err != nil || !ok {
		return ok, err
	}

	switch v := out.(type) {
	case *[]byte:
		*v = b
		return true, nil
	case *string:
		*v = string(b)
		return true, nil
	default:
		if err := gob.NewDecoder(bytes.NewReader(b)).Decode(out); err != nil {
			return false, fmt.Errorf("failed to decode cached value into target: %w", err)
		}
		return true, nil
	}
}

func (c *Client) Exists(key string) (bool, error) {
	dirPath, err := c.keyDir(key)
	if err != nil {
		return false, err
	}

	cachePath := filepath.Join(dirPath, cacheFileName)
	info, err := os.Stat(cachePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if info.IsDir() {
		return false, fmt.Errorf("%w: %s", ErrCachePathIsDir, cachePath)
	}

	expired, err := c.isExpired(dirPath)
	if err != nil {
		return false, err
	}
	if expired {
		_ = c.Remove(key)
		return false, nil
	}

	return true, nil
}
