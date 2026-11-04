package systemskills_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/vantt/skillstation/internal/skillscan"
	"github.com/vantt/skillstation/internal/systemskills"
)

// The folder scan in Go and the script's own scan read the same rules.txt. These
// tests hold them to the same findings, for the shipped rules over folders that
// trip every rule and over the skill folders of this repository, and to the
// same rules digest, which recorded audits keep.

type scriptFinding struct {
	Rule        string `json:"rule"`
	Category    string `json:"category"`
	Severity    string `json:"severity"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Evidence    string `json:"evidence"`
	Why         string `json:"why"`
	Fix         string `json:"fix"`
	Fingerprint string `json:"fingerprint"`
}

func shippedRules(t *testing.T) []skillscan.Rule {
	t.Helper()
	rules, err := skillscan.ParseRules(systemskills.AuditRules(), "rules/rules.txt")
	if err != nil {
		t.Fatal(err)
	}
	return rules
}

func TestShippedRulesPassTheirOwnExamples(t *testing.T) {
	t.Parallel()
	rules := shippedRules(t)
	if len(rules) == 0 {
		t.Fatal("no rules shipped")
	}
	if failures := skillscan.SelfTest(rules); len(failures) > 0 {
		t.Fatalf("rule examples fail:\n%s", strings.Join(failures, "\n"))
	}
}

func goFindings(t *testing.T, folder string, rules []skillscan.Rule) ([]string, []string) {
	t.Helper()
	files, err := skillscan.CollectFiles(folder)
	if err != nil {
		t.Fatal(err)
	}
	findings, err := skillscan.Audit(files, rules)
	if err != nil {
		t.Fatal(err)
	}
	var lines, names []string
	for _, f := range findings {
		lines = append(lines, describeFinding(scriptFinding(f)))
	}
	for _, f := range files {
		names = append(names, f.Rel)
	}
	sort.Strings(lines)
	sort.Strings(names)
	return lines, names
}

func scriptFindings(t *testing.T, folder string) ([]string, []string) {
	t.Helper()
	stdout, _ := runAudit(t, filepath.Dir(folder), nil, folder, "--json", "--fail-on", "critical")
	var report struct {
		Skills []struct {
			Files    []string        `json:"files"`
			Findings []scriptFinding `json:"findings"`
		} `json:"skills"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil || len(report.Skills) != 1 {
		t.Fatalf("script output for %s: %v\n%s", folder, err, stdout)
	}
	var lines []string
	for _, f := range report.Skills[0].Findings {
		lines = append(lines, describeFinding(f))
	}
	names := append([]string(nil), report.Skills[0].Files...)
	sort.Strings(lines)
	sort.Strings(names)
	return lines, names
}

func describeFinding(f scriptFinding) string {
	return fmt.Sprintf("%s|%s|%s|%s:%d|%q|%s", f.Rule, f.Category, f.Severity, f.File, f.Line, f.Evidence, f.Fingerprint)
}

func compareScans(t *testing.T, folder string, rules []skillscan.Rule) {
	t.Helper()
	wantLines, wantNames := scriptFindings(t, folder)
	gotLines, gotNames := goFindings(t, folder, rules)
	if strings.Join(gotNames, "\n") != strings.Join(wantNames, "\n") {
		t.Errorf("%s: files differ\ngo:     %v\nscript: %v", folder, gotNames, wantNames)
	}
	if strings.Join(gotLines, "\n") != strings.Join(wantLines, "\n") {
		t.Errorf("%s: findings differ\ngo (%d):\n%s\nscript (%d):\n%s", folder, len(gotLines), strings.Join(gotLines, "\n"), len(wantLines), strings.Join(wantLines, "\n"))
	}
}

func TestFolderScanFindsWhatTheScriptFinds(t *testing.T) {
	t.Parallel()
	nodePath(t)
	rules := shippedRules(t)

	t.Run("a folder that trips every rule", func(t *testing.T) {
		t.Parallel()
		folder := filepath.Join(t.TempDir(), "everything")
		writeParityFolder(t, folder, rules)
		compareScans(t, folder, rules)
		lines, _ := goFindings(t, folder, rules)
		hit := map[string]bool{}
		for _, line := range lines {
			hit[strings.SplitN(line, "|", 2)[0]] = true
		}
		for _, rule := range rules {
			if rule.ID == "fs.exec-bit" && runtime.GOOS == "windows" {
				continue // Windows has no executable bit to set
			}
			if !hit[rule.ID] {
				t.Errorf("rule %s found nothing in the folder built to trip it", rule.ID)
			}
		}
	})

	for _, root := range []string{filepath.Join("..", "..", ".agents", "skills"), filepath.Join("..", "..", "system-skills"), "skill-audit", "distill-lab", "curator"} {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		subjects := []string{root}
		if _, err := os.Stat(filepath.Join(root, "SKILL.md")); err != nil {
			subjects = nil
			for _, entry := range entries {
				if entry.IsDir() {
					subjects = append(subjects, filepath.Join(root, entry.Name()))
				}
			}
		}
		for _, subject := range subjects {
			absolute, err := filepath.Abs(subject)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(absolute, "SKILL.md")); err != nil {
				continue
			}
			t.Run(filepath.Base(absolute), func(t *testing.T) {
				t.Parallel()
				compareScans(t, absolute, rules)
			})
		}
	}
}

func TestRulesDigestMatchesTheScript(t *testing.T) {
	t.Parallel()
	node := nodePath(t)
	script, err := filepath.Abs(filepath.Join("skill-audit", "scripts", "audit.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	extra := filepath.Join(t.TempDir(), "audit-rules.txt")
	extraText := "## custom.marker\nseverity: low\nfiles: scripts/**, *.md\nscope: file\nflags: u\nredact: true\npattern: zzz-<marker>&\\u{2028}\nwhy: A rule added by a hub.\nmatch: zzz-<marker>&\nclean: nothing\n"
	if err := os.WriteFile(extra, []byte(extraText), 0o644); err != nil {
		t.Fatal(err)
	}
	program := `import { loadRules } from ` + fmt.Sprintf("%q", "file://"+script) + `; console.log(loadRules({ hub: '/nonexistent', rules: process.argv[1] }).digest)`
	command := exec.Command(node, "--input-type=module", "-e", program, extra)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("%v\n%s", err, stderr.String())
	}
	shipped, err := skillscan.ParseRules(systemskills.AuditRules(), "rules/rules.txt")
	if err != nil {
		t.Fatal(err)
	}
	added, err := skillscan.ParseRules(extraText, extra)
	if err != nil {
		t.Fatal(err)
	}
	all, err := skillscan.Combine(shipped, added)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := skillscan.RulesDigest(all), strings.TrimSpace(stdout.String()); got != want {
		t.Fatalf("rules digest = %s, the script computes %s", got, want)
	}
}

var exampleEscape = regexp.MustCompile(`\\u\{([0-9a-fA-F]+)\}`)

// unescapeExample turns \n and \u{XXXX} in a rule example into the characters
// they name.
func unescapeExample(sample string) string {
	sample = strings.ReplaceAll(sample, `\n`, "\n")
	return exampleEscape.ReplaceAllStringFunc(sample, func(escape string) string {
		code, err := strconv.ParseInt(escape[3:len(escape)-1], 16, 32)
		if err != nil {
			return escape
		}
		return string(rune(code))
	})
}

// writeParityFolder builds a skill folder with every shipped rule's match
// examples in files of several kinds, plus the file facts the built-in checks
// read: a symbolic link, a binary, an executable without an extension, a
// lifecycle script, a broad tool grant and a file that uses CRLF line ends.
func writeParityFolder(t *testing.T, folder string, rules []skillscan.Rule) {
	t.Helper()
	var samples []string
	for _, rule := range rules {
		for _, sample := range rule.Match {
			samples = append(samples, unescapeExample(sample))
		}
	}
	write := func(rel, content string, mode os.FileMode) {
		t.Helper()
		path := filepath.Join(folder, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
	}
	body := strings.Join(samples, "\n")
	write("SKILL.md", "---\nname: claude-helper\ndescription: <b>bold</b> helper\nallowed-tools: Bash(*), Read\n---\n"+body+"\n", 0o644)
	write("scripts/run.sh", body+"\n", 0o755)
	write("scripts/run.js", body+"\n", 0o644)
	write("references/notes.md", body+"\n", 0o644)
	write("references/crlf.md", strings.ReplaceAll(body, "\n", "\r\n")+"\r\n", 0o644)
	write("references/bom.txt", "\ufeff"+body+"\n", 0o644)
	write("package.json", `{"name":"x","scripts":{"postinstall":"node setup.js","test":"echo","prepare":{"a":1}}}`, 0o644)
	write("bin/tool", "#!/bin/sh\necho hi\n", 0o755)
	write("assets/logo.png", "\x89PNG\x00\x00data", 0o644)
	write("lib/native.so", "fake", 0o644)
	write("lib/elf", "\x7fELF\x02\x01", 0o644)
	write("lib/data.bin", "abc\x00def", 0o644)
	write("big/blob.txt", strings.Repeat("a", 2*1024*1024+1), 0o644)
	write(".git/config", body, 0o644)
	if err := os.Symlink("/etc/passwd", filepath.Join(folder, "outside-link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("SKILL.md", filepath.Join(folder, "inside-link")); err != nil {
		t.Fatal(err)
	}
	write("long/SKILL.md", strings.Repeat("line\n", 600), 0o644)
	write("Zebra.txt", body, 0o644)
	write("apple.txt", body, 0o644)
}
