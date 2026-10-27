package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vantt/skillstation/internal/catalog"
	resolverpkg "github.com/vantt/skillstation/internal/resolver"
	"github.com/vantt/skillstation/internal/skill"
	"github.com/vantt/skillstation/internal/skillmeta"
	"github.com/vantt/skillstation/internal/skillruntime"
	sourcepkg "github.com/vantt/skillstation/internal/source"
	"github.com/vantt/skillstation/internal/workspace"
)

// An agent in a project that holds a draft of a skill gets the draft, through
// skill_resolve and skill_get as through its own skill folder: the draft's
// skill.yaml routes the skill for that project, and its files are what the
// agent reads. Every other project gets the official version.
//
// The asking project is the folder the agent runs in. The MCP server learns
// it from the client's roots, or from its own working directory, and passes it
// in the context.

type projectDirKey struct{}

// WithProjectDir records the folder the asking agent runs in.
func WithProjectDir(ctx context.Context, dir string) context.Context {
	if dir == "" {
		return ctx
	}
	return context.WithValue(ctx, projectDirKey{}, filepath.Clean(dir))
}

// ProjectDirFrom returns the folder the asking agent runs in, or "".
func ProjectDirFrom(ctx context.Context) string {
	dir, _ := ctx.Value(projectDirKey{}).(string)
	return dir
}

// DraftServed tells an agent that it got a project's draft, not the official
// version.
type DraftServed struct {
	// Folder is the draft folder, which the agent may read and the person
	// edits.
	Folder string `json:"folder"`
	// Project is the project that holds the draft.
	Project string `json:"project"`
}

// projectDrafts returns the drafts the asking project holds, by skill id: for
// each skill, the record with the deepest project folder that contains the
// asking folder and still has a SKILL.md. A folder inside the hub asks for no
// draft.
func projectDrafts(ctx context.Context, root string) map[string]DraftRecord {
	dir := ProjectDirFrom(ctx)
	if dir == "" || withinFolder(dir, root) {
		return nil
	}
	drafts, err := loadDrafts(root)
	if err != nil || len(drafts) == 0 {
		return nil
	}
	found := map[string]DraftRecord{}
	for _, record := range drafts {
		if _, seen := found[record.SkillID]; seen {
			continue
		}
		best, ok := draftForFolder(drafts, record.SkillID, dir)
		if !ok {
			continue
		}
		if info, err := os.Stat(filepath.Join(best.FolderPath(), "SKILL.md")); err != nil || !info.Mode().IsRegular() {
			continue
		}
		found[record.SkillID] = best
	}
	return found
}

// draftDocument is a draft's metadata as the catalog would hold it once the
// draft is official: the hub's own record of the skill (or a new one) with
// the draft's skill.yaml, and the draft's SKILL.md for name and description.
// The draft is served as active to its project.
func draftDocument(root string, record DraftRecord) (document []byte, tree map[string]treeFile, merged []byte, err error) {
	tree, _, refusal, err := readProjectFolder(record.FolderPath())
	if err != nil {
		return nil, nil, nil, err
	}
	if refusal != nil {
		return nil, nil, nil, errors.New(refusal.Render.Error)
	}
	hub := []byte("schema_version: 1\nstatus: active\n")
	if _, _, current, locateErr := locateSkillDir(root, record.SkillID); locateErr == nil {
		if _, hubPart, splitErr := skillmeta.Split(current); splitErr == nil {
			hub = hubPart
		}
	}
	merged, err = skillmeta.Merge(tree[skillmeta.SettingsFile].Data, hub, record.SkillID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read skill.yaml of the draft: %w", err)
	}
	document, err = catalog.NormalizeSkillDocument(merged, tree["SKILL.md"].Data, record.SkillID)
	if err != nil {
		return nil, nil, nil, err
	}
	return document, tree, merged, nil
}

// draftContentDigest identifies a draft's content the way an approval would.
func draftContentDigest(tree map[string]treeFile, merged []byte) string {
	files := make([]skillruntime.ResourceDigest, 0, len(tree))
	for rel, file := range tree {
		files = append(files, skillruntime.ResourceDigest{Path: rel, Digest: sourcepkg.Digest(file.Data)})
	}
	spec, hasSpec := reviewRuntimeSpec(merged)
	return skillruntime.ContentDigest(files, spec, hasSpec)
}

// draftSkillURI is the skill URI of a draft's content. No snapshot holds it:
// an agent reads the draft from its folder.
func draftSkillURI(id, version string) string {
	return "skill://skillstation/" + strings.TrimPrefix(version, "sha256:") + "/" + id + "/SKILL.md"
}

// draftCatalog routes the drafts of the asking project with their own
// skill.yaml, in place of the official version, for one request.
type draftCatalog struct {
	resolverpkg.Catalog
	drafts map[string]resolverpkg.Skill
}

func (view draftCatalog) Skills(ctx context.Context) ([]resolverpkg.Skill, error) {
	skills, err := view.Catalog.Skills(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]resolverpkg.Skill, 0, len(skills)+len(view.drafts))
	for _, item := range skills {
		if _, replaced := view.drafts[item.ID]; !replaced {
			out = append(out, item)
		}
	}
	for _, item := range view.drafts {
		out = append(out, item)
	}
	return out, nil
}

// draftOverlay prepares the routing of the asking project's drafts. It
// returns nil when the project holds none. A draft that cannot be read is
// left out, so the official version routes instead.
func draftOverlay(ctx context.Context, root string) (map[string]DraftRecord, map[string]resolverpkg.Skill, map[string]string) {
	records := projectDrafts(ctx, root)
	if len(records) == 0 {
		return nil, nil, nil
	}
	skills := map[string]resolverpkg.Skill{}
	versions := map[string]string{}
	for id, record := range records {
		document, tree, merged, err := draftDocument(root, record)
		if err != nil {
			delete(records, id)
			continue
		}
		decoded, err := resolverpkg.DecodeSkillDocument(id, document)
		if err != nil {
			delete(records, id)
			continue
		}
		decoded.Status = "active"
		skills[id] = decoded
		versions[id] = draftContentDigest(tree, merged)
	}
	if len(skills) == 0 {
		return nil, nil, nil
	}
	return records, skills, versions
}

// ReadDraftSkill returns the asking project's draft of id for skill_get, with
// where the agent reads and runs it: the draft folder, and the state and
// config folders the skill's own dependencies and settings use. found is
// false when the project holds no draft of it.
func (service SkillService) ReadDraftSkill(ctx context.Context, path, id string) (SkillCore, DraftServed, *LocalSkill, bool, error) {
	root, err := workspace.Discover(path)
	if err != nil {
		return SkillCore{}, DraftServed{}, nil, false, err
	}
	record, ok := projectDrafts(ctx, root)[id]
	if !ok {
		return SkillCore{}, DraftServed{}, nil, false, nil
	}
	document, tree, merged, err := draftDocument(root, record)
	if err != nil {
		return SkillCore{}, DraftServed{}, nil, false, err
	}
	decoded, err := resolverpkg.DecodeSkillDocument(id, document)
	if err != nil {
		return SkillCore{}, DraftServed{}, nil, false, err
	}
	core := SkillCore{
		SkillID: id, Name: decoded.Name, Description: decoded.Description, Status: "checked out in this project",
		Path: record.FolderPath(), Content: string(tree["SKILL.md"].Data), ContentDigest: draftContentDigest(tree, merged),
		LifecycleState: "active", Routing: skill.RoutingFromDocument(decodeMetaOrEmpty(merged)), Resources: []skill.Resource{},
	}
	files := make([]skillruntime.ResourceDigest, 0, len(tree))
	for rel, file := range tree {
		core.Resources = append(core.Resources, skill.Resource{Path: rel, Kind: draftResourceKind(rel), Digest: sourcepkg.Digest(file.Data), SizeBytes: int64(len(file.Data))})
		files = append(files, skillruntime.ResourceDigest{Path: rel, Digest: sourcepkg.Digest(file.Data)})
	}
	folder := record.FolderPath()
	local := &LocalSkill{Path: folder, Status: LocalStatusReady}
	spec, hasSpec := reviewRuntimeSpec(merged)
	var dependencies []distributedSkillFile
	for _, file := range skillruntime.DependencyFiles(files) {
		data := tree[file.Path].Data
		dependencies = append(dependencies, distributedSkillFile{Path: file.Path, Kind: draftResourceKind(file.Path), Digest: file.Digest, Size: int64(len(data)), Bytes: data, Loaded: true})
	}
	stateDirectory, err := ensureStateDir(root, id, skillruntime.DepsKey(files, spec, hasSpec), dependencies)
	if err != nil {
		return SkillCore{}, DraftServed{}, nil, false, err
	}
	configDirectory, err := ensureSnapshotDirectory(root, skillruntime.ConfigDirRoot+"/"+id, true)
	if err != nil {
		return SkillCore{}, DraftServed{}, nil, false, err
	}
	local.StateDirectory = stateDirectory
	local.Env = skillExportEnv(folder, stateDirectory, configDirectory)
	return core, DraftServed{Folder: folder, Project: record.ProjectRoot}, local, true, nil
}

func decodeMetaOrEmpty(meta []byte) map[string]any {
	document, err := decodeMeta(meta)
	if err != nil || document == nil {
		return map[string]any{}
	}
	return document
}

// draftResourceKind names a draft file's kind as the catalog does.
func draftResourceKind(rel string) string {
	if rel == "SKILL.md" {
		return "instructions"
	}
	for _, kind := range []string{"references", "scripts", "assets"} {
		for _, part := range strings.Split(rel, "/") {
			if part == kind {
				return strings.TrimSuffix(kind, "s")
			}
		}
	}
	return "resource"
}
