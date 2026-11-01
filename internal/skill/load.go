package skill

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"

	"github.com/vantt/skillstation/internal/skillmeta"
)

// ErrInvalidSkillTree reports an entry under skills/ that a lookup refuses to
// read through: a symbolic link, a collection folder whose name is not an id,
// metadata that is not a regular file, or one id filed in two collections.
var ErrInvalidSkillTree = errors.New("invalid skill tree")

// Loaded is a skill found in the hub by its id.
type Loaded struct {
	// Collection is the name of the folder under skills/ that holds the skill.
	Collection string
	// Dir is the slash-separated skill folder below the hub root,
	// skills/<collection>/<id>.
	Dir string
	// MetadataPath is the slash-separated anchor metadata file below the hub
	// root, such as skills/<collection>/<id>/.meta/hub.yaml.
	MetadataPath string
	// Metadata is the skill's metadata in the pre-split shape, merged from
	// skill.yaml and .meta/hub.yaml when the hub keeps them apart.
	Metadata []byte
}

// Load finds the skill with the given id in the hub at root and reads its
// metadata. Every path is opened below the hub root and none is followed
// through a symbolic link: a link in place of a collection, a skill folder or a
// metadata file is refused with ErrInvalidSkillTree, as is an id filed in two
// collections. Regular files directly under skills/ are not collections and are
// skipped. A hub without a skills/ folder, or without the id, gives
// ErrNotFound.
//
// Load does not parse the metadata beyond merging its files, so a caller can
// still report a skill whose metadata is malformed.
func Load(root, id string) (Loaded, error) {
	if !skillmeta.ValidSkillID(id) {
		return Loaded{}, errors.New("skill id must be a lowercase kebab-case identifier")
	}
	handle, err := os.OpenRoot(root)
	if err != nil {
		return Loaded{}, err
	}
	defer handle.Close()
	collections, err := skillCollections(handle)
	if err != nil {
		return Loaded{}, err
	}
	var found Loaded
	for _, collection := range collections {
		anchor, ok, err := skillAnchor(handle, "skills/"+collection+"/"+id)
		if err != nil {
			return Loaded{}, err
		}
		if !ok {
			continue
		}
		if found.MetadataPath != "" {
			return Loaded{}, fmt.Errorf("%w: skill id %q is in collections %q and %q", ErrInvalidSkillTree, id, found.Collection, collection)
		}
		found = Loaded{Collection: collection, Dir: "skills/" + collection + "/" + id, MetadataPath: anchor}
	}
	if found.MetadataPath == "" {
		return Loaded{}, ErrNotFound
	}
	found.Metadata, _, _, err = skillmeta.Read(rootReader(handle), found.Dir)
	if err != nil {
		return Loaded{}, err
	}
	return found, nil
}

// IDs lists, sorted and once each, the ids of the skill folders under skills/
// that hold metadata. A skill folder that is a symbolic link is listed too, so
// Load refuses it instead of the listing hiding it. The collections follow the
// rules of Load.
func IDs(root string) ([]string, error) {
	handle, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	collections, err := skillCollections(handle)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, collection := range collections {
		entries, err := fs.ReadDir(handle.FS(), "skills/"+collection)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if !skillmeta.ValidSkillID(entry.Name()) {
				continue
			}
			if entry.Type()&fs.ModeSymlink != 0 || (entry.IsDir() && hasAnchor(handle, "skills/"+collection+"/"+entry.Name())) {
				seen[entry.Name()] = true
			}
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

// ReadEntrypoint reads SKILL.md of the skill folder dir, a Dir of Loaded, with
// the rules of Load. It returns nil without an error when the folder has no
// SKILL.md.
func ReadEntrypoint(root, dir string) ([]byte, error) {
	return readOptional(root, dir+"/SKILL.md")
}

// skillCollections lists the collection folders under skills/. It gives
// ErrNotFound when there is no skills/ folder.
func skillCollections(handle *os.Root) ([]string, error) {
	entries, err := fs.ReadDir(handle.FS(), "skills")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	collections := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Type()&fs.ModeSymlink == 0 && !entry.IsDir() {
			continue
		}
		if entry.Type()&fs.ModeSymlink != 0 || !skillmeta.ValidSkillID(entry.Name()) {
			return nil, fmt.Errorf("%w: collection %q is a symbolic link or its name is not an id", ErrInvalidSkillTree, "skills/"+entry.Name())
		}
		collections = append(collections, entry.Name())
	}
	return collections, nil
}

// skillAnchor returns the anchor metadata file of the skill folder directory,
// or false when the folder or its metadata is absent.
func skillAnchor(handle *os.Root, directory string) (string, bool, error) {
	info, err := handle.Lstat(directory)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
		return "", false, fmt.Errorf("%w: skill folder %q is a symbolic link or not a folder", ErrInvalidSkillTree, directory)
	}
	for _, anchor := range skillmeta.AnchorPaths(directory) {
		info, err := handle.Lstat(anchor)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", false, err
		}
		if info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return "", false, fmt.Errorf("%w: skill metadata %q is a symbolic link or not a regular file", ErrInvalidSkillTree, anchor)
		}
		return anchor, true, nil
	}
	return "", false, nil
}

func hasAnchor(handle *os.Root, directory string) bool {
	for _, anchor := range skillmeta.AnchorPaths(directory) {
		if _, err := handle.Lstat(anchor); err == nil {
			return true
		}
	}
	return false
}
