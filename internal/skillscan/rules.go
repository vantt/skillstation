// Package skillscan is the static security scan of a skill folder: a list of
// rules in a plain text format, applied to every file of the folder. It reads
// files and never runs them, and it has no network. A scan with no findings
// does not mean a skill is safe.
//
// The rules shipped with the skill-audit system skill (rules/rules.txt) are the
// default rule list; the hub's config/audit-rules.txt adds to it. Patterns are
// Go regular expressions (RE2): no lookahead, lookbehind or backreferences.
package skillscan

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Severities lists the severities from the lowest to the highest.
var Severities = []string{"info", "low", "medium", "high", "critical"}

// SeverityRank is the position of severity in Severities, or -1.
func SeverityRank(severity string) int {
	for index, name := range Severities {
		if name == severity {
			return index
		}
	}
	return -1
}

// Rule is one rule of the list.
type Rule struct {
	ID       string
	Category string
	Severity string
	// Files is the comma separated glob list the rule applies to; "*" when unset.
	Files string
	// Scope is "file" to match across lines; empty matches line by line.
	Scope string
	// Flags is the pattern flags as written; nil means the default "iu".
	Flags *string
	// Check names a built-in check on file facts instead of a pattern.
	Check  string
	Why    string
	Fix    string
	Source string
	// Redact hides the evidence in reports, for rules that find secrets.
	Redact   bool
	Patterns []string
	// Match and Clean are the rule's own examples: every Match line must hit the
	// rule and every Clean line must not.
	Match []string
	Clean []string
	// Origin and Line say where the rule was read, for error messages.
	Origin string
	Line   int

	globs      []*regexp.Regexp
	globIsPath []bool
	regexes    []*regexp.Regexp
	// rawRedact is the redact value as written, which the digest keeps.
	rawRedact string
}

var ruleID = regexp.MustCompile(`^[a-z0-9]+(\.[a-z0-9-]+)+$`)

// ParseRules reads the rule format: "## id" starts a rule, "key: value" lines
// follow, blank lines and lines starting with "#" are skipped. origin names the
// text in error messages.
func ParseRules(text, origin string) ([]Rule, error) {
	var rules []Rule
	current := -1
	for index, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		lineNo := index + 1
		fail := func(format string, args ...any) ([]Rule, error) {
			return nil, fmt.Errorf("%s:%d: %s", origin, lineNo, fmt.Sprintf(format, args...))
		}
		if strings.HasPrefix(raw, "## ") {
			rules = append(rules, Rule{ID: strings.TrimSpace(raw[3:]), Origin: origin, Line: lineNo})
			current = len(rules) - 1
			continue
		}
		if strings.TrimSpace(raw) == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		colon := strings.Index(raw, ":")
		if colon < 1 || current < 0 {
			return fail("expected \"key: value\" inside a rule, got %q", clip(raw, 40))
		}
		key, value := strings.TrimSpace(raw[:colon]), strings.TrimSpace(raw[colon+1:])
		rule := &rules[current]
		switch key {
		case "pattern":
			rule.Patterns = append(rule.Patterns, value)
		case "match":
			rule.Match = append(rule.Match, value)
		case "clean":
			rule.Clean = append(rule.Clean, value)
		case "category":
			rule.Category = value
		case "severity":
			rule.Severity = value
		case "files":
			rule.Files = value
		case "scope":
			rule.Scope = value
		case "flags":
			rule.Flags = &value
		case "check":
			rule.Check = value
		case "why":
			rule.Why = value
		case "fix":
			rule.Fix = value
		case "source":
			rule.Source = value
		case "redact":
			rule.rawRedact = value
			rule.Redact = value == "true"
		default:
			return fail("unknown key %q", key)
		}
	}
	for index := range rules {
		if err := rules[index].prepare(); err != nil {
			return nil, err
		}
	}
	return rules, nil
}

func clip(text string, max int) string {
	if len(text) <= max {
		return text
	}
	return text[:max]
}

func (rule *Rule) prepare() error {
	where := fmt.Sprintf("%s:%d (%s)", rule.Origin, rule.Line, rule.ID)
	if !ruleID.MatchString(rule.ID) {
		return fmt.Errorf("%s: id must look like category.name", where)
	}
	if SeverityRank(rule.Severity) < 0 {
		return fmt.Errorf("%s: severity must be one of %s", where, strings.Join(Severities, ", "))
	}
	if rule.Why == "" {
		return fmt.Errorf("%s: a rule needs a \"why\" line", where)
	}
	if rule.Check == "" && len(rule.Patterns) == 0 {
		return fmt.Errorf("%s: a rule needs a pattern or a check", where)
	}
	if rule.Category == "" {
		rule.Category = rule.ID[:strings.Index(rule.ID, ".")]
	}
	globs := rule.Files
	if globs == "" {
		globs = "*"
	}
	for _, glob := range strings.Split(globs, ",") {
		glob = strings.TrimSpace(glob)
		if glob == "" {
			continue
		}
		rule.globs = append(rule.globs, globRegexp(glob))
		rule.globIsPath = append(rule.globIsPath, strings.Contains(glob, "/"))
	}
	flags := "iu"
	if rule.Flags != nil {
		flags = *rule.Flags
	}
	for _, pattern := range rule.Patterns {
		compiled, err := compilePattern(pattern, flags)
		if err != nil {
			return fmt.Errorf("%s: bad pattern: %v", where, err)
		}
		rule.regexes = append(rule.regexes, compiled)
	}
	return nil
}

var unicodeEscape = regexp.MustCompile(`\\u\{([0-9a-fA-F]+)\}`)

// compilePattern turns a pattern and its flags into a Go regular expression.
// Of the flags, i, m and s carry over and u is implied; \u{XXXX} names a
// character as it does in the rules file.
func compilePattern(pattern, flags string) (*regexp.Regexp, error) {
	prefix := ""
	for _, flag := range flags {
		if flag == 'i' || flag == 'm' || flag == 's' {
			prefix += string(flag)
		}
	}
	if prefix != "" {
		prefix = "(?" + prefix + ")"
	}
	return regexp.Compile(prefix + unicodeEscape.ReplaceAllString(pattern, `\x{$1}`))
}

// globRegexp turns a file glob into an expression over a slash-separated path:
// "*" stops at a slash, "**" does not, and a "/" after "**" is optional.
func globRegexp(glob string) *regexp.Regexp {
	var source strings.Builder
	source.WriteString("^")
	for index := 0; index < len(glob); index++ {
		switch {
		case glob[index] == '*' && index+1 < len(glob) && glob[index+1] == '*':
			source.WriteString(".*")
			index++
			if index+1 < len(glob) && glob[index+1] == '/' {
				index++
			}
		case glob[index] == '*':
			source.WriteString("[^/]*")
		default:
			source.WriteString(regexp.QuoteMeta(string(glob[index])))
		}
	}
	source.WriteString("$")
	return regexp.MustCompile(source.String())
}

// RulesDigest identifies a rule list: it changes when a rule's pattern, files,
// severity, scope, flags, check or redaction does, and not when only its
// wording does. The skill-audit record keeps it, so a skill audited under other
// rules shows as rules-changed.
func RulesDigest(rules []Rule) string {
	var lines []string
	for _, rule := range rules {
		fields := []any{rule.Severity, optional(rule.Files), optional(rule.Scope), nil, optional(rule.Check), optional(rule.rawRedact), rule.Patterns}
		if rule.Flags != nil {
			fields[3] = *rule.Flags
		}
		if rule.Patterns == nil {
			fields[6] = []string{}
		}
		var out bytes.Buffer
		encoder := json.NewEncoder(&out)
		encoder.SetEscapeHTML(false)
		_ = encoder.Encode(fields)
		encoded := strings.TrimSuffix(out.String(), "\n")
		// A JSON text keeps U+2028 and U+2029 as they are.
		encoded = strings.NewReplacer(` `, " ", ` `, " ").Replace(encoded)
		lines = append(lines, rule.Origin+"\n"+rule.ID+"\n"+encoded)
	}
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// optional is nil for an empty value, so an unset key digests as null.
func optional(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// Combine joins rule lists and refuses an id that two rules share.
func Combine(lists ...[]Rule) ([]Rule, error) {
	var all []Rule
	seen := map[string]bool{}
	for _, list := range lists {
		for _, rule := range list {
			if seen[rule.ID] {
				return nil, fmt.Errorf("rule id %s is defined twice (%s:%d)", rule.ID, rule.Origin, rule.Line)
			}
			seen[rule.ID] = true
			all = append(all, rule)
		}
	}
	return all, nil
}
