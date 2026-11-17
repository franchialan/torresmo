package community

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// Store manages community addon manifests on disk.
type Store struct {
	dir string // ~/.torresmo/addons/
}

func NewStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".torresmo", "addons")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// Install saves an addon manifest to the local store.
func (s *Store) Install(manifest *AddonManifest) error {
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("serializing manifest: %w", err)
	}
	path := filepath.Join(s.dir, manifest.Metadata.Name+".yaml")
	return os.WriteFile(path, data, 0o644)
}

// Remove deletes an addon manifest from the local store.
func (s *Store) Remove(name string) error {
	path := filepath.Join(s.dir, name+".yaml")
	return os.Remove(path)
}

// Get loads a single addon manifest.
func (s *Store) Get(name string) (*AddonManifest, error) {
	path := filepath.Join(s.dir, name+".yaml")
	return LoadManifest(path)
}

// List returns all installed community addon manifests.
func (s *Store) List() ([]*AddonManifest, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var manifests []*AddonManifest
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		m, err := LoadManifest(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			continue
		}
		manifests = append(manifests, m)
	}

	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].Metadata.Name < manifests[j].Metadata.Name
	})

	return manifests, nil
}

// Has checks if a community addon is installed locally.
func (s *Store) Has(name string) bool {
	_, err := os.Stat(filepath.Join(s.dir, name+".yaml"))
	return err == nil
}
