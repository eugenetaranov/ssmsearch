package cache

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Cache handles local caching of SSM parameter keys.
type Cache struct {
	dir string
}

// New creates a new Cache, creating ~/.ssmsearch/ if needed.
func New() (*Cache, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	dir := filepath.Join(home, ".ssmsearch")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &Cache{dir: dir}, nil
}

// Path returns the path to the cache file for the given account ID.
func (c *Cache) Path(accountID string) string {
	return filepath.Join(c.dir, accountID+".cache")
}

// Load reads parameter keys from the cache file.
func (c *Cache) Load(accountID string) ([]string, error) {
	path := c.Path(accountID)

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open cache file: %w", err)
	}
	defer func() { _ = f.Close() }()

	var keys []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			keys = append(keys, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	return keys, nil
}

// Save writes parameter keys to the cache file.
func (c *Cache) Save(accountID string, keys []string) error {
	path := c.Path(accountID)

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create cache file: %w", err)
	}
	defer func() { _ = f.Close() }()

	w := bufio.NewWriter(f)
	for _, key := range keys {
		if _, err := w.WriteString(key + "\n"); err != nil {
			return fmt.Errorf("failed to write to cache file: %w", err)
		}
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush cache file: %w", err)
	}

	return nil
}

// Remove deletes the cache file for the given account ID.
func (c *Cache) Remove(accountID string) error {
	path := c.Path(accountID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove cache file: %w", err)
	}
	return nil
}

// Exists checks if a cache file exists for the given account ID.
func (c *Cache) Exists(accountID string) bool {
	path := c.Path(accountID)
	_, err := os.Stat(path)
	return err == nil
}
