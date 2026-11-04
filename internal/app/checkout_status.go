package app

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vantt/skillstation/internal/digest"
	"github.com/vantt/skillstation/internal/shellquote"
)

// openCheckoutPriority sorts open checkouts below the work that blocks others
// and above reminders about uncommitted Git changes.
const openCheckoutPriority = 30

// openCheckoutActions lists the checkouts of this hub that the project around
// projectDir still holds, each with how many files changed since the checkout
// and whether the hub changed meanwhile. A forgotten checkout is a second copy
// that drifts, so status names it.
func openCheckoutActions(ctx context.Context, root, projectDir string) []ActionItem {
	projectRoot := projectRootOf(ctx, projectDir)
	var markers []string
	for _, parent := range []string{projectSkillsDir, claudeSkillsDir} {
		found, err := filepath.Glob(filepath.Join(projectRoot, filepath.FromSlash(parent), "*", CheckoutMarkerName))
		if err != nil {
			return nil
		}
		markers = append(markers, found...)
	}
	sort.Strings(markers)
	var actions []ActionItem
	var folders []string
	count := map[string]int{}
	for _, markerPath := range markers {
		folder := filepath.Dir(markerPath)
		marker, found, err := ReadCheckoutMarker(folder)
		if err != nil || !found || !sameHub(marker.Hub, root) {
			continue
		}
		if action, ok := describeOpenCheckout(ctx, root, folder, marker); ok {
			rel, _ := filepath.Rel(projectRoot, folder)
			action.Summary += " (in " + filepath.ToSlash(rel) + "/)"
			actions = append(actions, action)
			folders = append(folders, filepath.ToSlash(rel))
			count[marker.SkillID]++
		}
	}
	// A bare id cannot tell two checkouts of one skill apart, so each names
	// its own folder.
	for index, action := range actions {
		if count[action.ID] > 1 {
			actions[index].Summary += fmt.Sprintf("; %s is checked out twice in this project", action.ID)
			actions[index].Command = "skillstation skill commit ./" + folders[index]
		}
	}
	return actions
}

func sameHub(recorded, root string) bool {
	if recorded == root {
		return true
	}
	a, errA := filepath.EvalSymlinks(recorded)
	b, errB := filepath.EvalSymlinks(root)
	return errA == nil && errB == nil && a == b
}

func describeOpenCheckout(ctx context.Context, root, folder string, marker CheckoutMarker) (ActionItem, bool) {
	tree, _, refusal, err := readProjectFolder(folder)
	if err != nil || refusal != nil {
		return ActionItem{}, false
	}
	action := ActionItem{Kind: "open_checkout", ID: marker.SkillID, Priority: openCheckoutPriority, Command: "skillstation skill commit " + marker.SkillID}
	hub, err := loadHubSkill(root, marker.SkillID)
	if err != nil {
		return ActionItem{}, false
	}
	if hub == nil {
		action.Count = len(tree)
		action.Summary = fmt.Sprintf("%s: the hub no longer has this skill; committing adds it as a new skill", marker.SkillID)
		return action, true
	}
	_ = normalizeProjectSkillMD(tree, marker.SkillID, hub)
	base, hubChanged, baseKnown := treeBytes(hub.tree), hub.version != marker.BaseVersion, true
	if hubChanged {
		base, baseKnown = findCheckoutBase(ctx, root, hub.relDir, marker.SkillID, marker)
	}
	action.Count = countTreeDifferences(base, tree)
	action.Summary = checkoutProgress(marker.SkillID, action.Count, hubChanged, baseKnown)
	return action, true
}

// countTreeDifferences counts the files added, changed or removed from base to tree.
func countTreeDifferences(base map[string][]byte, tree map[string]treeFile) int {
	count := 0
	for rel, file := range tree {
		if before, ok := base[rel]; !ok || !bytes.Equal(before, file.Data) {
			count++
		}
	}
	for rel := range base {
		if _, ok := tree[rel]; !ok {
			count++
		}
	}
	return count
}

func checkoutProgress(id string, changed int, hubChanged, baseKnown bool) string {
	summary := fmt.Sprintf("%s: no changes since checkout", id)
	if changed > 0 {
		summary = fmt.Sprintf("%s: %s changed since checkout", id, pluralize(changed, "file"))
	}
	switch {
	case hubChanged && !baseKnown:
		summary += "; the hub changed too and the starting point is gone, so a commit will ask about every differing file"
	case hubChanged:
		summary += "; the hub changed too, which `skillstation skill pull " + id + "` brings in"
	}
	return summary
}

// openDraftActions lists, for status in the hub folder, the drafts that
// projects on this machine hold. A record whose folder is gone is named so the
// person can remove it.
func openDraftActions(root string) []ActionItem {
	drafts, err := loadDrafts(root)
	if err != nil {
		return nil
	}
	actions := make([]ActionItem, 0, len(drafts))
	for _, record := range drafts {
		action := ActionItem{Kind: "open_draft", ID: record.SkillID, Priority: openCheckoutPriority}
		folder := record.FolderPath()
		if _, found, err := ReadCheckoutMarker(folder); err != nil || !found {
			if _, statErr := os.Stat(filepath.Join(folder, "SKILL.md")); statErr != nil {
				action.Summary = fmt.Sprintf("%s: the hub records a checkout in %s, but that folder is gone; remove the record on the Checkouts page of the web UI (`skillstation serve web`)", record.SkillID, folder)
				actions = append(actions, action)
				continue
			}
		}
		action.Summary = fmt.Sprintf("%s: checked out in %s since %s", record.SkillID, folder, record.StartedAt.Format("2006-01-02"))
		action.Command = "cd " + shellquote.Arg(record.ProjectRoot) + " && skillstation skill commit " + record.SkillID
		actions = append(actions, action)
	}
	return actions
}

// RemoteOpenCheckouts lists the checkouts this project holds from a hub on
// another machine, for the status of a project that has no hub of its own. It
// reads only this machine: how many files changed since the checkout comes from
// the digests the marker kept, and the hub is never asked.
func RemoteOpenCheckouts(ctx context.Context, projectDir string, now time.Time) []ActionItem {
	projectRoot := projectRootOf(ctx, projectDir)
	var markers []string
	for _, parent := range []string{projectSkillsDir, claudeSkillsDir} {
		found, err := filepath.Glob(filepath.Join(projectRoot, filepath.FromSlash(parent), "*", CheckoutMarkerName))
		if err != nil {
			return nil
		}
		markers = append(markers, found...)
	}
	sort.Strings(markers)
	actions := []ActionItem{}
	for _, markerPath := range markers {
		folder := filepath.Dir(markerPath)
		marker, found, err := ReadCheckoutMarker(folder)
		if err != nil || !found || !strings.HasPrefix(marker.Hub, "http") || marker.BaseFiles == nil {
			continue
		}
		tree, _, refusal, err := readProjectFolder(folder)
		if err != nil || refusal != nil {
			continue
		}
		changed := 0
		for rel, was := range marker.BaseFiles {
			file, there := tree[rel]
			if !there || digest.Bytes(file.Data) != was {
				changed++
			}
		}
		for rel := range tree {
			if _, known := marker.BaseFiles[rel]; !known {
				changed++
			}
		}
		rel, _ := filepath.Rel(projectRoot, folder)
		summary := fmt.Sprintf("%s: %s since checkout%s (in %s/); this project uses its own copy of the skill until it is sent to the hub (`skillstation skill commit %s`) or the checkout is removed",
			marker.SkillID, changedFiles(changed), checkoutAge(marker.CheckedOutAt, now), filepath.ToSlash(rel), marker.SkillID)
		actions = append(actions, ActionItem{Kind: "open_checkout", ID: marker.SkillID, Count: changed, Priority: openCheckoutPriority, Summary: summary, Command: "skillstation skill commit " + marker.SkillID})
	}
	return actions
}

func changedFiles(count int) string {
	if count == 0 {
		return "no changes"
	}
	return pluralize(count, "file") + " changed"
}

// checkoutAge says how long ago a checkout was made, or nothing when the
// marker holds no readable time.
func checkoutAge(checkedOutAt string, now time.Time) string {
	at, err := time.Parse("2006-01-02T15:04:05Z", checkedOutAt)
	if err != nil || now.Before(at) {
		return ""
	}
	days := int(now.Sub(at).Hours() / 24)
	if days < 1 {
		return ", checked out today"
	}
	return ", checked out " + pluralize(days, "day") + " ago"
}
