package skillscan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf16"
)

// File is one file of a skill folder with the facts the rules need.
type File struct {
	// Rel is the slash-separated path inside the folder.
	Rel        string
	Size       int64
	Executable bool
	// HasText is set for a file read as text; Text is then its content without
	// a leading byte order mark.
	HasText bool
	Text    string
	// Binary names the kind of a file that is not text: ELF, Mach-O, PE,
	// "zip archive" or "binary".
	Binary         string
	Symlink        bool
	SymlinkTarget  string
	SymlinkOutside bool
}

var (
	skippedDirs    = map[string]bool{".git": true, ".meta": true}
	mediaExt       = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".ico": true, ".ttf": true, ".otf": true, ".woff": true, ".woff2": true, ".mp4": true, ".mov": true, ".webm": true, ".mp3": true, ".wav": true, ".pdf": true}
	scriptExt      = map[string]bool{".sh": true, ".bash": true, ".py": true, ".mjs": true, ".js": true, ".cjs": true, ".ts": true, ".rb": true, ".pl": true, ".ps1": true}
	nativeLibrary  = regexp.MustCompile(`(?i)\.(so|dylib|dll|exe|pyc|class)$`)
	executableMark = map[string]string{"7f454c46": "ELF", "feedface": "Mach-O", "feedfacf": "Mach-O", "cafebabe": "Mach-O", "cffaedfe": "Mach-O", "cefaedfe": "Mach-O", "504b0304": "zip archive"}
)

// CollectFiles lists every file of a skill folder, skipping .git and .meta. A
// symbolic link is listed, never followed.
func CollectFiles(dir string) ([]File, error) {
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	var files []File
	var walk func(current string) error
	walk = func(current string) error {
		entries, err := os.ReadDir(current)
		if err != nil {
			return err
		}
		sort.SliceStable(entries, func(i, j int) bool { return localeLess(entries[i].Name(), entries[j].Name()) })
		for _, entry := range entries {
			full := filepath.Join(current, entry.Name())
			if entry.IsDir() {
				if !skippedDirs[entry.Name()] {
					if err := walk(full); err != nil {
						return err
					}
				}
				continue
			}
			rel, err := filepath.Rel(root, full)
			if err != nil {
				return err
			}
			info, err := os.Lstat(full)
			if err != nil {
				return err
			}
			file := File{Rel: filepath.ToSlash(rel), Size: info.Size(), Executable: info.Mode().Perm()&0o111 != 0}
			switch {
			case info.Mode()&os.ModeSymlink != 0:
				file.Symlink = true
				file.SymlinkTarget, _ = os.Readlink(full)
				resolved, err := filepath.EvalSymlinks(full)
				file.SymlinkOutside = err != nil || (resolved != root && !strings.HasPrefix(resolved, root+string(filepath.Separator)))
			case info.Mode().IsRegular() && info.Size() <= maxTextBytes:
				data, err := os.ReadFile(full)
				if err != nil {
					return err
				}
				file.Binary = binaryKind(data)
				if file.Binary == "" && nativeLibrary.MatchString(file.Rel) {
					file.Binary = "binary"
				}
				if file.Binary == "" {
					file.HasText = true
					file.Text = strings.TrimPrefix(string(data), "\ufeff")
				}
			}
			files = append(files, file)
		}
		return nil
	}
	if err := walk(root); err != nil {
		return nil, err
	}
	return files, nil
}

func binaryKind(data []byte) string {
	if len(data) >= 4 {
		if kind, ok := executableMark[fmt.Sprintf("%x", data[:4])]; ok {
			return kind
		}
		if data[0] == 0x4d && data[1] == 0x5a {
			return "PE"
		}
	}
	head := data
	if len(head) > 8000 {
		head = head[:8000]
	}
	if bytes.IndexByte(head, 0) >= 0 {
		return "binary"
	}
	return ""
}

// builtinChecks are the rules that look at file facts instead of text; a rule
// names one with its check key.
var builtinChecks = map[string]func(File) []hit{
	"symlink": func(file File) []hit {
		if !file.Symlink {
			return nil
		}
		severity := "high"
		if file.SymlinkOutside {
			severity = "critical"
		}
		return []hit{{evidence: "symbolic link to " + file.SymlinkTarget, severity: severity}}
	},
	// An image, font or media file with no executable magic is data: shown, never failing.
	"binary": func(file File) []hit {
		if file.Binary == "" {
			return nil
		}
		if file.Binary == "binary" && mediaExt[strings.ToLower(filepath.Ext(file.Rel))] {
			return []hit{{evidence: fmt.Sprintf("media or font asset, %d bytes", file.Size), severity: "info"}}
		}
		return []hit{{evidence: fmt.Sprintf("%s file, %d bytes", file.Binary, file.Size)}}
	},
	"size": func(file File) []hit {
		if file.Size > maxTextBytes {
			return []hit{{evidence: fmt.Sprintf("%d bytes, over %d", file.Size, maxTextBytes)}}
		}
		if file.Rel == "SKILL.md" && file.HasText && strings.Count(file.Text, "\n")+1 > maxSkillMDLines {
			return []hit{{evidence: fmt.Sprintf("SKILL.md has more than %d lines", maxSkillMDLines)}}
		}
		return nil
	},
	"exec-bit": func(file File) []hit {
		if file.Executable && !scriptExt[strings.ToLower(filepath.Ext(file.Rel))] {
			return []hit{{evidence: "executable bit on a file that is not a script"}}
		}
		return nil
	},
	"allowed-tools": func(file File) []hit {
		if file.Rel != "SKILL.md" || !file.HasText {
			return nil
		}
		front, ok := readFrontmatter(file.Text)
		if !ok {
			return nil
		}
		value, found := frontmatterValue(front, "allowed-tools")
		if !found || value == "" || !broadTools.MatchString(value) {
			return nil
		}
		return []hit{{evidence: clipUnits("allowed-tools: "+value, maxEvidence)}}
	},
	"lifecycle": func(file File) []hit {
		if filepath.Base(file.Rel) != "package.json" || !file.HasText {
			return nil
		}
		var manifest struct {
			Scripts map[string]any `json:"scripts"`
		}
		if json.Unmarshal([]byte(file.Text), &manifest) != nil {
			return nil
		}
		var hits []hit
		for _, key := range []string{"preinstall", "install", "postinstall", "prepare"} {
			if value, ok := manifest.Scripts[key]; ok {
				text := "[object Object]"
				if s, isString := value.(string); isString {
					text = s
				}
				hits = append(hits, hit{evidence: fmt.Sprintf("scripts.%s: %s", key, clipUnits(text, 120))})
			}
		}
		return hits
	},
	"meta": func(file File) []hit {
		if file.Rel != "SKILL.md" || !file.HasText {
			return nil
		}
		front, ok := readFrontmatter(file.Text)
		if !ok {
			return []hit{{line: 1, evidence: "SKILL.md has no frontmatter"}}
		}
		name, _ := frontmatterValue(front, "name")
		description, _ := frontmatterValue(front, "description")
		var hits []hit
		if name == "" || description == "" {
			hits = append(hits, hit{line: 1, evidence: "frontmatter lacks a name or a description"})
		}
		if reservedName.MatchString(name) {
			hits = append(hits, hit{line: 1, evidence: fmt.Sprintf("name %q uses a reserved word", name)})
		}
		if length := len(utf16.Encode([]rune(description))); length > 1024 {
			hits = append(hits, hit{line: 1, evidence: fmt.Sprintf("description is %d characters", length)})
		}
		if markup.MatchString(description) {
			hits = append(hits, hit{line: 1, evidence: "description holds markup"})
		}
		return hits
	},
}

var (
	frontmatterBlock = regexp.MustCompile(`(?s)^---\r?\n(.*?)\r?\n---`)
	// broadTools is a Bash grant with no limit, or limited to curl or wget. The
	// character after the grant is part of the match.
	broadTools   = regexp.MustCompile(`(?i)(^|[\s,\[])Bash(\(\*\)|\(curl[^)]*\)|\(wget[^)]*\))?([\s,\]]|$)`)
	reservedName = regexp.MustCompile(`(?i)anthropic|claude`)
	markup       = regexp.MustCompile(`(?i)<[a-z][^>]*>`)
	indentedLine = regexp.MustCompile(`^\s+\S`)
	listLine     = regexp.MustCompile(`^\s*-\s`)
)

func readFrontmatter(text string) (string, bool) {
	match := frontmatterBlock.FindStringSubmatch(text)
	if match == nil {
		return "", false
	}
	return match[1], true
}

// frontmatterValue reads a top-level key of the frontmatter, including a value
// folded or listed on the lines under it. It is not a YAML parser: it is the
// reading the scan has always made.
func frontmatterValue(front, key string) (string, bool) {
	line := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `:[ \t]*(.*)$`)
	located := line.FindStringSubmatchIndex(front)
	if located == nil {
		return "", false
	}
	value := strings.TrimFunc(front[located[2]:located[3]], isJSSpace)
	if value == "" || value == ">-" || value == ">" || value == "|" {
		rest := strings.Split(front[located[1]:], "\n")[1:]
		var block []string
		for _, text := range rest {
			if !indentedLine.MatchString(text) && !listLine.MatchString(text) {
				break
			}
			block = append(block, strings.TrimFunc(text, isJSSpace))
		}
		value = strings.Join(block, " ")
	}
	return value, true
}
