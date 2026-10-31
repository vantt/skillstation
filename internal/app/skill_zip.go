package app

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	sourcepkg "github.com/vantt/skillstation/internal/source"
	"github.com/vantt/skillstation/internal/workspace"
)

const (
	// MaxSkillZipBytes bounds an uploaded archive. The skill content inside is
	// held to the import limits, which are smaller than this for any real skill.
	MaxSkillZipBytes = 10 << 20
	// maxZipEntries bounds the entries read from the archive, files left out
	// included, so a crafted central directory cannot keep the server busy.
	maxZipEntries = 10_000
)

func zipRejected(reason, fix string) error {
	return NewInvalidRequestError(reason, fix)
}

// PreviewSkillZip previews adding the skills in an uploaded zip archive. The
// archive is unpacked into a private temporary folder and goes through the same
// capture, discovery and validation as any other skill source; nothing is
// written to the hub until the returned proposal is confirmed. The temporary
// folder is removed before returning.
func (service SkillAddService) PreviewSkillZip(ctx context.Context, hubPath string, archive []byte, filename string, input SkillAddInput) (SkillAddProposal, error) {
	root, err := workspace.Discover(hubPath)
	if err != nil {
		return SkillAddProposal{}, err
	}
	stage, cleanup, err := newZipStage(root, filename)
	if err != nil {
		return SkillAddProposal{}, err
	}
	defer cleanup()

	if err := extractSkillZip(archive, stage); err != nil {
		var appErr *Error
		if errors.As(err, &appErr) {
			return SkillAddProposal{Result: ErrorResult(appErr)}, nil
		}
		return SkillAddProposal{}, err
	}
	input.Locator = stage
	input.OriginKind = SkillOriginArchive
	return service.PreviewSkillAdd(ctx, hubPath, input)
}

// newZipStage makes a private folder under the hub's runtime/tmp whose name
// derives from the archive name, so a skill without its own name keeps it.
func newZipStage(root, filename string) (string, func(), error) {
	holder, removeHolder, err := newRuntimeTemp(root, "skill-zip-")
	if err != nil {
		return "", nil, err
	}
	name := sanitizeSkillID(strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)))
	if name == "" {
		name = "skill"
	}
	stage := filepath.Join(holder, name)
	if err := os.Mkdir(stage, 0o700); err != nil {
		removeHolder()
		return "", nil, err
	}
	return stage, removeHolder, nil
}

// extractSkillZip unpacks archive into the empty folder dest. It refuses the
// archive as a whole on anything unsafe or over the import limits, and skips
// files an import never copies (build output, secrets, system files).
func extractSkillZip(archive []byte, dest string) error {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return zipRejected("The file is not a readable zip archive.", "Upload a .zip made from the skill folder.")
	}
	if len(reader.File) > maxZipEntries {
		return zipRejected(fmt.Sprintf("The archive has more than %d entries.", maxZipEntries), "Zip only the skill's own folder.")
	}
	out, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	seen := map[string]string{}
	var files int
	var total int64
	for _, entry := range reader.File {
		rel, skip, err := checkZipEntry(entry)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		if prior, clash := seen[strings.ToLower(rel)]; clash {
			return zipRejected(fmt.Sprintf("The archive lists %q and %q, which are the same file on some systems.", prior, rel), "Remove the duplicate and zip again.")
		}
		seen[strings.ToLower(rel)] = rel
		if entry.FileInfo().IsDir() {
			if err := out.MkdirAll(rel, 0o700); err != nil {
				return err
			}
			continue
		}
		if _, left := sourcepkg.ExcludedFromImport(rel); left {
			continue
		}
		files++
		if files > sourcepkg.DefaultMaxFiles {
			return zipRejected(fmt.Sprintf("The archive has more than %d files to import.", sourcepkg.DefaultMaxFiles), "Zip only the skill's own folder.")
		}
		written, err := writeZipFile(out, entry, rel, sourcepkg.DefaultMaxBytes-total)
		if err != nil {
			return err
		}
		total += written
	}
	return nil
}

// checkZipEntry validates one entry's name and kind. skip is true for entries
// that are only archive noise.
func checkZipEntry(entry *zip.File) (rel string, skip bool, err error) {
	name := entry.Name
	switch {
	case name == "" || strings.ContainsRune(name, 0):
		return "", false, zipRejected("An entry in the archive has an invalid name.", "Zip the skill folder again.")
	case strings.Contains(name, `\`):
		return "", false, zipRejected(fmt.Sprintf("The entry %q uses a backslash in its path.", name), "Zip the skill folder again with a standard tool.")
	case strings.HasPrefix(name, "/") || (len(name) > 1 && name[1] == ':'):
		return "", false, zipRejected(fmt.Sprintf("The entry %q has an absolute path.", name), "Zip the skill folder again with relative paths.")
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return "", false, zipRejected(fmt.Sprintf("The entry %q leaves the archive folder.", name), "Zip the skill folder again.")
		}
	}
	rel = strings.TrimSuffix(path.Clean(name), "/")
	if rel == "." || rel == "" {
		return "", true, nil
	}
	if rel == "__MACOSX" || strings.HasPrefix(rel, "__MACOSX/") {
		return "", true, nil
	}
	mode := entry.Mode()
	switch {
	case entry.Flags&0x1 != 0:
		return "", false, zipRejected(fmt.Sprintf("The entry %q is encrypted.", name), "Zip the skill folder again without a password.")
	case mode&fs.ModeSymlink != 0:
		return "", false, zipRejected(fmt.Sprintf("The entry %q is a symbolic link.", name), "Replace links with the files they point to and zip again.")
	case !mode.IsRegular() && !mode.IsDir():
		return "", false, zipRejected(fmt.Sprintf("The entry %q is not a regular file.", name), "Zip only regular files and folders.")
	}
	return rel, false, nil
}

// writeZipFile copies one entry, counting the bytes actually produced rather
// than the size the archive declares. remaining is what the whole archive may
// still write.
func writeZipFile(out *os.Root, entry *zip.File, rel string, remaining int64) (int64, error) {
	limit := min(sourcepkg.DefaultMaxFileSize, remaining)
	if entry.UncompressedSize64 > uint64(sourcepkg.DefaultMaxFileSize) {
		return 0, zipRejected(fmt.Sprintf("The file %q is larger than %s.", rel, megabytes(sourcepkg.DefaultMaxFileSize)), "Remove or shrink the file and zip again.")
	}
	if dir := path.Dir(rel); dir != "." {
		if err := out.MkdirAll(dir, 0o700); err != nil {
			return 0, err
		}
	}
	perm := fs.FileMode(0o600)
	if entry.Mode()&0o111 != 0 {
		perm = 0o700
	}
	source, err := entry.Open()
	if err != nil {
		return 0, zipRejected(fmt.Sprintf("The file %q cannot be read from the archive.", rel), "Zip the skill folder again.")
	}
	defer source.Close()
	target, err := out.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return 0, fmt.Errorf("create %q from the archive: %w", rel, err)
	}
	written, copyErr := io.Copy(target, io.LimitReader(source, limit+1))
	closeErr := target.Close()
	if copyErr != nil {
		return 0, zipRejected(fmt.Sprintf("The file %q cannot be unpacked.", rel), "Zip the skill folder again.")
	}
	if closeErr != nil {
		return 0, closeErr
	}
	if written > limit && limit == sourcepkg.DefaultMaxFileSize {
		return 0, zipRejected(fmt.Sprintf("The file %q is larger than %s.", rel, megabytes(sourcepkg.DefaultMaxFileSize)), "Remove or shrink the file and zip again.")
	}
	if written > limit {
		return 0, zipRejected(fmt.Sprintf("The archive holds more than %s of files to import.", megabytes(sourcepkg.DefaultMaxBytes)), "Zip only the skill's own folder.")
	}
	return written, nil
}

func megabytes(size int64) string {
	return fmt.Sprintf("%d MiB", size>>20)
}
