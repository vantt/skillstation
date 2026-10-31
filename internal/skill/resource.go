package skill

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vantt/skillstation/internal/catalog"
)

// MaxEditableResourceBytes caps the editable size of a companion file and the
// number of bytes returned when a larger text file is viewed.
const MaxEditableResourceBytes = 1 << 20

// Resource describes a digest-pinned skill file in one catalog snapshot.
type Resource = catalog.Resource

// Manifest is the immutable resource view returned before content loading.
type Manifest struct {
	SkillID         string     `json:"skill_id"`
	Name            string     `json:"name"`
	Description     string     `json:"description"`
	Status          string     `json:"status"`
	CatalogSnapshot string     `json:"catalog_snapshot"`
	Resources       []Resource `json:"resources"`
}

// GetManifestAnyState is the curator read surface: it returns a skill in any
// lifecycle state, with the state recorded in the manifest. Distribution reads
// the active skills through the generation instead.
func GetManifestAnyState(ctx context.Context, root, id string) (Manifest, error) {
	if !idPattern.MatchString(id) {
		return Manifest{}, errors.New("skill id must be a lowercase kebab-case identifier")
	}
	handle, err := catalog.OpenWithFallback(ctx, root)
	if err != nil {
		return Manifest{}, fmt.Errorf("%w: %v", ErrSnapshotExpired, err)
	}
	defer handle.Close()
	manifest := Manifest{SkillID: id, CatalogSnapshot: handle.Pointer.CatalogSnapshot, Resources: []Resource{}}
	if err := handle.DB.QueryRowContext(ctx, `SELECT name,description,status FROM skills WHERE id=?`, id).Scan(&manifest.Name, &manifest.Description, &manifest.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Manifest{}, ErrNotFound
		}
		return Manifest{}, err
	}
	if manifest.Resources, err = catalog.SkillResources(ctx, handle.DB, id); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// ReadResource serves canonical bytes only when the current generation, path,
// and digest all match the caller's manifest pins. Old bytes are never guessed.
func ReadResource(ctx context.Context, root, snapshot, path, expectedDigest string) ([]byte, error) {
	if snapshot == "" || expectedDigest == "" {
		return nil, errors.New("catalog snapshot and resource digest pins are required")
	}
	if !safeResourcePath(path) {
		return nil, errors.New("resource path is invalid")
	}
	handle, err := catalog.OpenWithFallback(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSnapshotExpired, err)
	}
	defer handle.Close()
	if handle.Pointer.CatalogSnapshot != snapshot {
		return nil, ErrSnapshotExpired
	}
	var catalogDigest string
	if err := handle.DB.QueryRowContext(ctx, `SELECT digest FROM resources WHERE path=?`, path).Scan(&catalogDigest); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSnapshotExpired
		}
		return nil, err
	}
	if catalogDigest != expectedDigest {
		return nil, ErrSnapshotExpired
	}
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer rootHandle.Close()
	info, err := rootHandle.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, ErrSnapshotExpired
	}
	contents, err := rootHandle.ReadFile(path)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(contents)
	actual := "sha256:" + hex.EncodeToString(sum[:])
	if actual != expectedDigest {
		return nil, ErrResourceDigestMismatch
	}
	return contents, nil
}

func safeResourcePath(path string) bool {
	if path == "" || filepath.IsAbs(filepath.FromSlash(path)) || strings.Contains(path, `\`) || !strings.HasPrefix(path, "skills/") {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	return clean == path && !strings.Contains(path, "../")
}
