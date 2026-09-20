// Package imagecatalog loads the on-disk image library (vms/images/*.json)
// and exposes it to the API for the dashboard's image picker.
package imagecatalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"porter/internal/types"
)

// Catalog is an immutable snapshot of the image library at startup.
type Catalog struct {
	images []types.ImageManifest
}

// New scans dir for *.json image manifests. A missing or empty dir yields
// an empty catalog (the API stays usable, just with no quick-deploy picker).
func New(dir string) *Catalog {
	c := &Catalog{}
	if dir == "" {
		return c
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return c
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var m types.ImageManifest
		if err := json.Unmarshal(b, &m); err != nil {
			continue
		}
		if m.ID == "" {
			m.ID = strings.TrimSuffix(e.Name(), ".json")
		}
		c.images = append(c.images, m)
	}
	sort.Slice(c.images, func(i, j int) bool { return c.images[i].Name < c.images[j].Name })
	return c
}

// All returns the catalog entries.
func (c *Catalog) All() []types.ImageManifest {
	return c.images
}
