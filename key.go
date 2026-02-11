package nim

import (
	"slices"
	"strings"
)

func ValidateKey(key string) error {
	if key == "" {
		return ErrCacheKeyEmpty
	}

	if !strings.Contains(key, "::") {
		return nil
	}

	parts := strings.Split(key, "::")
	if slices.Contains(parts, "") {
		return ErrCacheKeyEmptySegment
	}

	return nil
}

func SplitKey(key string) ([]string, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}

	if !strings.Contains(key, "::") {
		return []string{key}, nil
	}

	return strings.Split(key, "::"), nil
}
