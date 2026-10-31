package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/vantt/skillstation/internal/catalog"
	"github.com/vantt/skillstation/internal/skill"
	"github.com/vantt/skillstation/internal/skillmeta"
)

// SkillCore is the runtime read model of one skill, as MCP skill_get serves
// it: identity, lifecycle, the verified SKILL.md, routing and the resource
// manifest, all read from one pinned catalog generation. It carries no
// curation judgement, so assembling it never inspects canonical state.
type SkillCore struct {
	SkillID         string             `json:"skill_id"`
	Name            string             `json:"name"`
	Description     string             `json:"description"`
	Status          string             `json:"status"`
	Path            string             `json:"path"`
	CatalogSnapshot string             `json:"catalog_snapshot"`
	Content         string             `json:"content"`
	ContentDigest   string             `json:"content_digest"`
	LifecycleState  string             `json:"lifecycle_state"`
	Routing         skill.RoutingInput `json:"routing"`
	Rationale       string             `json:"rationale,omitempty"`
	Resources       []skill.Resource   `json:"resources"`
}

// SkillDetail is the curation read model for the CLI and WebUI: the runtime
// core plus the assessment of canonical state against the served catalog.
type SkillDetail struct {
	SkillID          string             `json:"skill_id"`
	Name             string             `json:"name"`
	Description      string             `json:"description"`
	Status           string             `json:"status"`
	Path             string             `json:"path"`
	CatalogSnapshot  string             `json:"catalog_snapshot"`
	Content          string             `json:"content"`
	ContentDigest    string             `json:"content_digest"`
	StateBasis       string             `json:"state_basis"`
	LifecycleState   string             `json:"lifecycle_state"`
	RoutingEligible  bool               `json:"routing_eligible"`
	Diverged         bool               `json:"diverged"`
	ChangedResources []string           `json:"changed_resources,omitempty"`
	MissingResources []string           `json:"missing_resources,omitempty"`
	Routing          skill.RoutingInput `json:"routing"`
	Rationale        string             `json:"rationale,omitempty"`
	Labels           []string           `json:"labels,omitempty"`
	Topics           []string           `json:"topics,omitempty"`
	Technologies     []string           `json:"technologies,omitempty"`
	Origin           string             `json:"origin,omitempty"`
	Resources        []skill.Resource   `json:"resources"`
}

// ContentTrustFor evaluates the content trust of a skill in any lifecycle
// state from its canonical files: a third-party skill is trusted only when the
// recorded approval matches the current content digest. Callers that hand
// skill content to an agent use it to withhold unreviewed third-party content.
func (SkillService) ContentTrustFor(ctx context.Context, path, id string) (ContentTrust, error) {
	if err := ctx.Err(); err != nil {
		return ContentTrust{}, err
	}
	root, err := skill.ResolveWorkspace(path)
	if err != nil {
		return ContentTrust{}, err
	}
	_, skillRelDir, skillMetaBytes, err := locateSkillDir(root, id)
	if err != nil {
		return ContentTrust{}, err
	}
	entrypointRelPath, entrypointDigest, _ := inspectCanonicalEntrypoint(root, skillRelDir)
	_, resources, _ := inventorySkillResources(root, filepath.Join(root, filepath.FromSlash(skillRelDir)), entrypointRelPath, entrypointDigest)
	return reviewContentTrust(id, skillRelDir, resources, skillMetaBytes), nil
}

// ReadSkillCore reads a skill in any lifecycle state from the pinned catalog
// generation: one catalog open, one manifest query, and the entrypoint read
// once and verified against its pinned digest. Routing and rationale come from
// the catalog's copy of the skill metadata, which the same generation
// published.
func (SkillService) ReadSkillCore(ctx context.Context, path, id string) (core SkillCore, resultErr error) {
	root, view, err := observeSkill(ctx, path, id)
	if err != nil {
		return SkillCore{}, err
	}
	defer closeDistributionHandle(view, &resultErr)
	return readSkillCore(ctx, root, view, id)
}

// observeSkill checks the skill id and observes the current catalog generation
// under the shared workspace lock, which the caller releases by closing the
// view.
func observeSkill(ctx context.Context, path, id string) (string, *catalog.View, error) {
	if !skillmeta.ValidSkillID(id) {
		return "", nil, errors.New("skill id must be a lowercase kebab-case identifier")
	}
	root, err := skill.ResolveWorkspace(path)
	if err != nil {
		return "", nil, err
	}
	view, err := catalog.ObserveWithFallbackLocked(ctx, root)
	if err != nil {
		return "", nil, fmt.Errorf("%w: %v", skill.ErrSnapshotExpired, err)
	}
	return root, view, nil
}

func readSkillCore(ctx context.Context, root string, view *catalog.View, id string) (SkillCore, error) {
	row, err := loadSkillRow(ctx, view, id)
	if err != nil {
		return SkillCore{}, err
	}
	core := SkillCore{SkillID: id, Name: row.Name, Description: row.Description, Status: row.Status, LifecycleState: row.Status, CatalogSnapshot: view.Pointer.CatalogSnapshot, Resources: slices.Clone(row.Resources)}
	if row.Manifest == nil {
		return SkillCore{}, fmt.Errorf("%w: skill %s has no catalog manifest", skill.ErrSnapshotExpired, id)
	}
	routing, rationale, err := row.routingAndRationale()
	if err != nil {
		return SkillCore{}, fmt.Errorf("decode skill %s manifest: %w", id, err)
	}
	core.Routing, core.Rationale = routing, rationale
	var entrypoint *skill.Resource
	for index := range core.Resources {
		if strings.HasSuffix(core.Resources[index].Path, "/SKILL.md") {
			entrypoint = &core.Resources[index]
			break
		}
	}
	if entrypoint == nil {
		return SkillCore{}, fmt.Errorf("%w: skill has no SKILL.md manifest entry", skill.ErrSnapshotExpired)
	}
	contents, err := readPinnedResource(root, entrypoint.Path, entrypoint.Digest)
	if err != nil {
		if errors.Is(err, skill.ErrResourceDigestMismatch) {
			why := fmt.Sprintf("Current canonical content for %s no longer matches published manifest digest %s.", entrypoint.Path, entrypoint.Digest)
			fix := fmt.Sprintf("Run `skillstation maintain rebuild` to publish current canonical files, or inspect differences with `skillstation skill review %s`.", id)
			return SkillCore{}, NewResourceContentUnavailableError(why, fix)
		}
		return SkillCore{}, err
	}
	core.Path = entrypoint.Path
	core.Content = string(contents)
	core.ContentDigest = entrypoint.Digest
	return core, nil
}

// GetSkillDetail returns the full detail and read model for a single skill:
// the runtime core plus the curation assessment of its canonical state. It
// serves the CLI and WebUI; MCP skill_get serves the core alone. The core, the
// assessment and the classification come from one observation of the catalog,
// so the canonical tree is listed once per detail.
func (service SkillService) GetSkillDetail(ctx context.Context, path, id string) (SkillDetail, error) {
	root, observed, err := observeSkillDetail(ctx, path, id)
	if err != nil {
		return SkillDetail{}, err
	}
	core, assessment, facet := observed.core, observed.assessment, observed.facet
	lifecycleState := core.LifecycleState
	if lifecycleState == "" {
		lifecycleState = assessment.Canonical.Status
	}
	routingEligible := (lifecycleState == "active") && (!assessment.Served.Known || assessment.Served.Servable)

	// The upstream status only tells an adopted skill from a forked one, so
	// a skill without an upstream does not inspect every tracked skill.
	upstreamStatus := ""
	if facet.hasUpstream {
		upstreamStatus = skillUpstreamStatus(ctx, root, id)
	}
	return SkillDetail{
		Labels:           facet.labels,
		Topics:           facet.topics,
		Technologies:     facet.technologies,
		Origin:           deriveOrigin(facet.hasUpstream, upstreamStatus, facet.hasLearning),
		SkillID:          core.SkillID,
		Name:             core.Name,
		Description:      core.Description,
		Status:           core.Status,
		Path:             core.Path,
		CatalogSnapshot:  core.CatalogSnapshot,
		Content:          core.Content,
		ContentDigest:    core.ContentDigest,
		StateBasis:       string(catalog.BasisCanonical),
		LifecycleState:   lifecycleState,
		RoutingEligible:  routingEligible,
		Diverged:         assessment.Diverged,
		ChangedResources: assessment.ChangedResources,
		MissingResources: assessment.MissingResources,
		Routing:          core.Routing,
		Rationale:        core.Rationale,
		Resources:        core.Resources,
	}, nil
}

// observedSkillDetail is what a skill's detail reads under one observation of
// the catalog.
type observedSkillDetail struct {
	core       SkillCore
	assessment catalog.SkillStateAssessment
	facet      skillFacets
}

func observeSkillDetail(ctx context.Context, path, id string) (root string, observed observedSkillDetail, resultErr error) {
	root, view, err := observeSkill(ctx, path, id)
	if err != nil {
		return "", observedSkillDetail{}, err
	}
	defer closeDistributionHandle(view, &resultErr)
	if observed.core, err = readSkillCore(ctx, root, view, id); err != nil {
		return "", observedSkillDetail{}, err
	}
	if observed.assessment, err = catalog.AssessSkillStateInView(ctx, view, id); err != nil {
		return "", observedSkillDetail{}, err
	}
	observed.facet = skillDetailFacet(ctx, view, id)
	return root, observed, nil
}

// skillDetailFacet reads the classification of one skill from the observed
// generation the way the list does. A failure leaves the facets empty: they
// are extra to the detail.
func skillDetailFacet(ctx context.Context, view *catalog.View, id string) skillFacets {
	database, err := view.DB(ctx)
	if err != nil {
		return skillFacets{}
	}
	facets, err := skillFacetRows(ctx, database)
	if err != nil || facets[id] == nil {
		return skillFacets{}
	}
	if err := markSourceLinks(ctx, database, facets); err != nil {
		return skillFacets{}
	}
	return *facets[id]
}

// skillUpstreamStatus returns the upstream status of one tracked skill, or
// nothing when it cannot be read.
func skillUpstreamStatus(ctx context.Context, root, id string) string {
	upstreams, err := ListSkillUpstream(ctx, root)
	if err != nil {
		return ""
	}
	status := ""
	for _, upstream := range upstreams {
		if upstream.SkillID == id {
			status = upstream.Status
		}
	}
	return status
}
