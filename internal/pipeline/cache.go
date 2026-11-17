package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// Cache manages pipeline step caching.
type Cache struct {
	dir string
}

// CacheKey represents what makes a target unique.
type CacheKey struct {
	Target   string            `json:"target"`
	Image    string            `json:"image"`
	Commands []string          `json:"commands"`
	EnvVars  map[string]string `json:"env_vars"`
	FileHash string            `json:"file_hash"`
}

// NewCache creates a cache rooted at ~/.torresmo/cache/.
func NewCache() (*Cache, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".torresmo", "cache")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Cache{dir: dir}, nil
}

// Key generates a hex-encoded SHA-256 cache key.
func (c *Cache) Key(key CacheKey) string {
	data, _ := json.Marshal(key)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// Has checks if a cache entry exists.
func (c *Cache) Has(key string) bool {
	_, err := os.Stat(filepath.Join(c.dir, key))
	return err == nil
}

// Store saves the contents of artifactDir into the cache under the given key.
func (c *Cache) Store(key string, artifactDir string) error {
	dest := filepath.Join(c.dir, key)
	return copyDir(artifactDir, dest)
}

// Restore copies cached artifacts to the given directory.
func (c *Cache) Restore(key string, artifactDir string) error {
	src := filepath.Join(c.dir, key)
	return copyDir(src, artifactDir)
}

// Clear removes all cache entries.
func (c *Cache) Clear() error {
	return os.RemoveAll(c.dir)
}

// HashDir computes a SHA-256 hash of all files in a directory.
// Hidden directories, node_modules, vendor, and .torresmo are skipped.
func HashDir(dir string) (string, error) {
	h := sha256.New()
	var files []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == ".torresmo" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return "", err
	}

	sort.Strings(files)
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			continue
		}
		h.Write([]byte(f))
		h.Write(data)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// copyDir copies a directory recursively from src to dst.
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcInfo.IsDir() {
		return copyFile(src, dst)
	}

	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file from src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Close()
}
