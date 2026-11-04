package skillscan

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	maxTextBytes    = 2 * 1024 * 1024
	maxTotalBytes   = 8 * 1024 * 1024
	maxSkillMDLines = 500
	maxEvidence     = 160
)

// Finding is one place where a rule matched, as evidence for a reviewer.
type Finding struct {
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

// hit is what a match or a built-in check found in one file, before it becomes
// a Finding.
type hit struct {
	line     int
	evidence string
	severity string
}

// Audit runs every rule over the files of one folder. The findings come
// highest severity first, then by file and line.
func Audit(files []File, rules []Rule) ([]Finding, error) {
	var findings []Finding
	add := func(rule Rule, file string, found hit) {
		evidence := clipUnits(strings.TrimFunc(found.evidence, isJSSpace), maxEvidence)
		severity := rule.Severity
		if found.severity != "" {
			severity = found.severity
		}
		shown := evidence
		if rule.Redact {
			shown = "[redacted]"
		}
		findings = append(findings, Finding{
			Rule: rule.ID, Category: rule.Category, Severity: severity, File: file, Line: found.line,
			Evidence: shown, Why: rule.Why, Fix: rule.Fix, Fingerprint: fingerprint(rule.ID, file, evidence),
		})
	}
	for _, file := range files {
		for _, rule := range rules {
			if !rule.appliesTo(file.Rel) {
				continue
			}
			if rule.Check != "" {
				run, ok := builtinChecks[rule.Check]
				if !ok {
					return nil, fmt.Errorf("%s:%d (%s): unknown check %q", rule.Origin, rule.Line, rule.ID, rule.Check)
				}
				for _, found := range run(file) {
					add(rule, file.Rel, found)
				}
			}
			if file.HasText {
				for _, found := range rule.matchText(file.Text) {
					add(rule, file.Rel, found)
				}
			}
		}
	}
	var total int64
	for _, file := range files {
		total += file.Size
	}
	if total > maxTotalBytes {
		for _, rule := range rules {
			if rule.Check == "size" {
				evidence := fmt.Sprintf("%d bytes in all, over %d", total, maxTotalBytes)
				findings = append(findings, Finding{
					Rule: rule.ID, Category: rule.Category, Severity: rule.Severity, File: ".", Evidence: evidence,
					Why: rule.Why, Fix: rule.Fix, Fingerprint: fingerprint(rule.ID, ".", fmt.Sprint(total)),
				})
				break
			}
		}
	}
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if ra, rb := SeverityRank(a.Severity), SeverityRank(b.Severity); ra != rb {
			return ra > rb
		}
		if a.File != b.File {
			return localeLess(a.File, b.File)
		}
		return a.Line < b.Line
	})
	return findings, nil
}

// Verdict is the highest severity among the findings, or "clean".
func Verdict(findings []Finding) string {
	if len(findings) == 0 {
		return "clean"
	}
	return findings[0].Severity
}

func (rule Rule) appliesTo(rel string) bool {
	base := rel[strings.LastIndex(rel, "/")+1:]
	for index, glob := range rule.globs {
		target := base
		if rule.globIsPath[index] {
			target = rel
		}
		if glob.MatchString(target) {
			return true
		}
	}
	return false
}

// matchText applies the pattern rules to one text. A rule with several
// patterns needs all of them somewhere in the file, and the first one locates
// the evidence; a single pattern is matched line by line unless the rule's
// scope is the file.
func (rule Rule) matchText(text string) []hit {
	if len(rule.regexes) == 0 {
		return nil
	}
	if len(rule.regexes) > 1 || rule.Scope == "file" {
		var first []int
		for index, re := range rule.regexes {
			located := re.FindStringIndex(text)
			if located == nil {
				return nil
			}
			if index == 0 {
				first = located
			}
		}
		line := 1 + strings.Count(text[:first[0]], "\n")
		evidence, _, _ := strings.Cut(text[first[0]:first[1]], "\n")
		return []hit{{line: line, evidence: evidence}}
	}
	var hits []hit
	for index, content := range strings.Split(text, "\n") {
		if rule.regexes[0].MatchString(content) {
			hits = append(hits, hit{line: index + 1, evidence: content})
		}
	}
	return hits
}

// SelfTest checks each rule's own examples and returns the failures: every
// match line must hit the rule and every clean line must not.
func SelfTest(rules []Rule) []string {
	var failures []string
	for _, rule := range rules {
		if rule.Check == "" && (len(rule.Match) == 0 || len(rule.Clean) == 0) {
			failures = append(failures, rule.ID+": a pattern rule needs at least one match and one clean example")
		}
		hits := func(sample string) bool { return len(rule.matchText(unescapeSample(sample))) > 0 }
		for _, sample := range rule.Match {
			if !hits(sample) {
				failures = append(failures, fmt.Sprintf("%s: should match %q", rule.ID, sample))
			}
		}
		for _, sample := range rule.Clean {
			if hits(sample) {
				failures = append(failures, fmt.Sprintf("%s: should not match %q", rule.ID, sample))
			}
		}
	}
	return failures
}

// unescapeSample turns \n and \u{XXXX} in an example into the characters they name.
func unescapeSample(sample string) string {
	sample = strings.ReplaceAll(sample, `\n`, "\n")
	return unicodeEscape.ReplaceAllStringFunc(sample, func(escape string) string {
		var code int32
		if _, err := fmt.Sscanf(escape, `\u{%x}`, &code); err != nil {
			return escape
		}
		return string(rune(code))
	})
}

func fingerprint(rule, file, evidence string) string {
	normalized := strings.Join(strings.FieldsFunc(evidence, isJSSpace), " ")
	sum := sha256.Sum256([]byte(rule + "\n" + file + "\n" + normalized))
	return hex.EncodeToString(sum[:])[:16]
}

// isJSSpace is the white space a JavaScript trim and \s know, so evidence and
// fingerprints are the same whichever implementation of the scan made them.
func isJSSpace(r rune) bool {
	switch {
	case r == ' ' || (r >= '\t' && r <= '\r') || r == 0xA0 || r == 0x1680 || r == 0x2028 || r == 0x2029 || r == 0x202F || r == 0x205F || r == 0x3000 || r == 0xFEFF:
		return true
	}
	return r >= 0x2000 && r <= 0x200A
}

// clipUnits cuts text to at most max UTF-16 code units, without splitting a
// character.
func clipUnits(text string, max int) string {
	units := 0
	for offset, r := range text {
		width := utf16.RuneLen(r)
		if r == utf8.RuneError {
			width = 1
		}
		if units+width > max {
			return text[:offset]
		}
		units += width
	}
	return text
}

// localeLess orders names ignoring case first, as a locale-aware sort does.
func localeLess(a, b string) bool {
	la, lb := strings.ToLower(a), strings.ToLower(b)
	if la != lb {
		return la < lb
	}
	return a > b
}
