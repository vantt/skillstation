package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/vantt/skillstation/internal/userconfig"
)

func TestConnectFromASubfolderConnectsTheGitProjectRoot(t *testing.T) {
	t.Parallel()
	hub := healthHub(t)
	project := t.TempDir()
	if output, err := exec.Command("git", "init", "-q", project).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	subfolder := filepath.Join(project, "skills", "tool")
	if err := os.MkdirAll(subfolder, 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := (ConnectService{Store: userconfig.NewStore(t.TempDir())}).Connect(t.Context(), ConnectRequest{Project: subfolder, Workspace: hub})
	if err != nil || result.Error != nil {
		t.Fatalf("connect: %+v %v", result.Error, err)
	}
	if _, err := os.Stat(filepath.Join(project, ".claude", "settings.local.json")); err != nil {
		t.Fatalf("project root not connected: %v", err)
	}
	entries, err := os.ReadDir(subfolder)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("connect wrote into the subfolder: %v", entries)
	}
}

func TestConnectOutsideGitConnectsTheGivenFolder(t *testing.T) {
	t.Parallel()
	hub := healthHub(t)
	folder := filepath.Join(t.TempDir(), "plain")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := (ConnectService{Store: userconfig.NewStore(t.TempDir())}).Connect(t.Context(), ConnectRequest{Project: folder, Workspace: hub})
	if err != nil || result.Error != nil {
		t.Fatalf("connect: %+v %v", result.Error, err)
	}
	if _, err := os.Stat(filepath.Join(folder, ".claude", "settings.local.json")); err != nil {
		t.Fatalf("folder not connected: %v", err)
	}
}
