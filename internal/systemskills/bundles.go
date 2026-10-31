package systemskills

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

const (
	// DistillSkillID is the stable identity of the bundled distill-lab skill.
	DistillSkillID = "distill-lab"
	// DistillSkillVersion versions the bundled distill-lab files independently
	// of the binary.
	DistillSkillVersion = "0.1.0"

	// AuditSkillID is the stable identity of the bundled skill-audit skill.
	AuditSkillID = "skill-audit"
	// AuditSkillVersion versions the bundled skill-audit files independently
	// of the binary.
	AuditSkillVersion = "0.1.0"

	skillEntryFile = "SKILL.md"
	distillRoot    = "distill-lab"
	auditRoot      = "skill-audit"
)

// distillLabFS holds the distill-lab bundle. It mirrors the editable source in
// .agents/skills/distill-lab/ of this repository byte for byte, because an
// embed directive cannot reach outside the package folder. Like every directory embed,
// it skips names that start with "." or "_".
//
//go:embed distill-lab
var distillLabFS embed.FS

// skillAuditFS holds the skill-audit bundle. It mirrors the editable source in
// .agents/skills/skill-audit/ of this repository byte for byte, for the same
// reason as the distill-lab mirror.
//
//go:embed skill-audit
var skillAuditFS embed.FS

// SkillFile is one file of a system skill bundle. Path is slash-separated and
// relative to the skill folder.
type SkillFile struct {
	Path    string
	Content []byte
}

// SystemSkill is a system skill that Skillstation installs as files on disk.
type SystemSkill struct {
	ID      string
	Version string
	// Files lists SKILL.md first, then every other file sorted by path.
	Files []SkillFile
	// HubOnly marks a skill installed only in the hub folder itself.
	HubOnly bool
	// ServedOverMCP marks a skill listed by skills/list and skills/get.
	ServedOverMCP bool
	// FormerIDs are ids earlier releases installed the skill under. Their files
	// are removed when Skillstation wrote them, and never written again.
	FormerIDs []string
}

var systemSkills = mustBuildSystemSkills()

func mustBuildSystemSkills() []SystemSkill {
	distillFiles, err := embeddedFiles(distillLabFS, distillRoot)
	if err != nil {
		panic(fmt.Sprintf("systemskills: read embedded %s bundle: %v", distillRoot, err))
	}
	auditFiles, err := embeddedFiles(skillAuditFS, auditRoot)
	if err != nil {
		panic(fmt.Sprintf("systemskills: read embedded %s bundle: %v", auditRoot, err))
	}
	return []SystemSkill{
		{
			ID:            CuratorSkillID,
			Version:       CuratorSkillVersion,
			Files:         []SkillFile{{Path: skillEntryFile, Content: []byte(CuratorSkill)}},
			ServedOverMCP: true,
			FormerIDs:     []string{CuratorFormerSkillID},
		},
		{
			ID:      DistillSkillID,
			Version: DistillSkillVersion,
			Files:   distillFiles,
			HubOnly: true,
		},
		{
			ID:      AuditSkillID,
			Version: AuditSkillVersion,
			Files:   auditFiles,
			HubOnly: true,
		},
	}
}

func embeddedFiles(fsys fs.FS, root string) ([]SkillFile, error) {
	var files []SkillFile
	err := fs.WalkDir(fsys, root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		content, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		files = append(files, SkillFile{Path: strings.TrimPrefix(name, root+"/"), Content: content})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool {
		if (files[i].Path == skillEntryFile) != (files[j].Path == skillEntryFile) {
			return files[i].Path == skillEntryFile
		}
		return files[i].Path < files[j].Path
	})
	if len(files) == 0 || files[0].Path != skillEntryFile {
		return nil, fmt.Errorf("%s has no %s", root, skillEntryFile)
	}
	return files, nil
}

// SystemSkills returns every system skill in a fixed order: skill-curator,
// distill-lab, then skill-audit. The result is a deep copy, so callers may modify it.
func SystemSkills() []SystemSkill {
	out := make([]SystemSkill, len(systemSkills))
	for i, skill := range systemSkills {
		out[i] = skill
		out[i].FormerIDs = append([]string(nil), skill.FormerIDs...)
		out[i].Files = make([]SkillFile, len(skill.Files))
		for j, file := range skill.Files {
			out[i].Files[j] = SkillFile{Path: file.Path, Content: append([]byte(nil), file.Content...)}
		}
	}
	return out
}

// FileDigest returns the digest form used by KnownFileDigest and
// CuratorBundle: "sha256:" followed by lowercase hex.
func FileDigest(content []byte) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256(content))
}

// KnownFileDigest reports whether digest identifies bytes that Skillstation wrote
// for the file at path (slash-separated, relative to the skill folder) of the
// system skill id: either the current bundle or a previously shipped version.
func KnownFileDigest(id, path, digest string) bool {
	for _, skill := range systemSkills {
		if skill.ID != id {
			continue
		}
		for _, file := range skill.Files {
			if file.Path == path && FileDigest(file.Content) == digest {
				return true
			}
		}
	}
	for _, known := range knownDigests[id][path] {
		if known == digest {
			return true
		}
	}
	return false
}

// KnownFileContent is KnownFileDigest for the bytes of a file on disk. A copy
// whose line endings were turned into CRLF, as Git does on checkout when
// core.autocrlf is true on Windows, counts as the same file: every bundled
// file and every recorded digest uses LF line endings.
func KnownFileContent(id, path string, raw []byte) bool {
	if KnownFileDigest(id, path, FileDigest(raw)) {
		return true
	}
	normalized := NormalizeLineEndings(raw)
	return len(normalized) != len(raw) && KnownFileDigest(id, path, FileDigest(normalized))
}

// SameContent reports whether a and b hold the same text once CRLF line
// endings are read as LF.
func SameContent(a, b []byte) bool {
	return bytes.Equal(NormalizeLineEndings(a), NormalizeLineEndings(b))
}

// NormalizeLineEndings returns content with every CRLF line ending replaced
// by LF. Content without CRLF is returned as it is.
func NormalizeLineEndings(content []byte) []byte {
	if !bytes.Contains(content, []byte("\r\n")) {
		return content
	}
	return bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
}

// IsSystemSkillID reports whether id names a system skill, under its current
// id or one an earlier release used. A hub skill may not take such an id.
func IsSystemSkillID(id string) bool {
	for _, skill := range systemSkills {
		if skill.ID == id {
			return true
		}
		for _, former := range skill.FormerIDs {
			if former == id {
				return true
			}
		}
	}
	return false
}
