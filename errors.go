package nim

import "errors"

var (
	ErrCacheRootPathEmpty   = errors.New("cache root path cannot be empty")
	ErrCacheKeyEmpty        = errors.New("cache key cannot be empty")
	ErrCacheKeyEmptySegment = errors.New("cache key contains empty segment")
	ErrCachePathIsDir       = errors.New("cache path is a directory")
	ErrCacheValueTooLarge   = errors.New("cache value exceeds max bytes")
)
