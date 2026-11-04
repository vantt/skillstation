package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/vantt/skillstation/internal/skillscan"
	"github.com/vantt/skillstation/internal/systemskills"
)

// SkillScanService runs the static security scan on skill folders. It reads
// files only, needs no hub and records nothing: a result is advice for whoever
// holds the folder, and a hub that receives the skill scans it again.
type SkillScanService struct{}

// FolderScan is the scan of one folder.
type FolderScan struct {
	// ID is the folder's name, which is the skill id when the folder is sent.
	ID       string              `json:"id"`
	Folder   string              `json:"folder"`
	Verdict  string              `json:"verdict"`
	Files    []string            `json:"files"`
	Findings []skillscan.Finding `json:"findings"`
}

// ScanResult is the result of scanning folders.
type ScanResult struct {
	Result
	Rules       int          `json:"rules"`
	RulesDigest string       `json:"rules_digest"`
	FailOn      string       `json:"fail_on"`
	Scans       []FolderScan `json:"scans"`
	// Failed is set when a finding reaches FailOn.
	Failed bool `json:"failed"`
}

// SelfTestResult is the result of running every rule's own examples.
type SelfTestResult struct {
	Result
	Rules    int      `json:"rules"`
	Failures []string `json:"failures"`
}

// loadScanRules reads the rules that ship with skill-audit and, when
// extraFile is set, the rules of that file after them.
func loadScanRules(extraFile string) ([]skillscan.Rule, error) {
	shipped, err := skillscan.ParseRules(systemskills.AuditRules(), "rules/rules.txt")
	if err != nil {
		return nil, err
	}
	if extraFile == "" {
		return shipped, nil
	}
	text, err := os.ReadFile(extraFile)
	if err != nil {
		return nil, NewInvalidRequestError(fmt.Sprintf("Cannot read the rules file %s: %v", extraFile, err), "Pass --rules the path of a rules file, or leave it out to use the rules that ship with skillstation.")
	}
	added, err := skillscan.ParseRules(string(text), extraFile)
	if err != nil {
		return nil, NewInvalidRequestError(err.Error(), "Fix the rule in the rules file; patterns are Go regular expressions.")
	}
	rules, err := skillscan.Combine(shipped, added)
	if err != nil {
		return nil, NewInvalidRequestError(err.Error(), "Give each rule its own id.")
	}
	return rules, nil
}

// Scan scans each folder with the shipped rules and the rules of extraFile.
// failOn is the lowest severity that makes the result Failed.
func (SkillScanService) Scan(folders []string, extraFile, failOn string) (ScanResult, error) {
	if skillscan.SeverityRank(failOn) < 0 {
		return ScanResult{}, NewInvalidRequestError(fmt.Sprintf("%q is not a severity.", failOn), "Pass --fail-on info, low, medium, high or critical.")
	}
	if len(folders) == 0 {
		return ScanResult{}, NewInvalidRequestError("Name a skill folder to scan.", "Run `skillstation skill scan <folder>`.")
	}
	rules, err := loadScanRules(extraFile)
	if err != nil {
		return ScanResult{}, err
	}
	result := ScanResult{Rules: len(rules), RulesDigest: skillscan.RulesDigest(rules), FailOn: failOn}
	threshold := skillscan.SeverityRank(failOn)
	findings := 0
	for _, folder := range folders {
		absolute, err := filepath.Abs(folder)
		if err != nil {
			return ScanResult{}, err
		}
		if info, err := os.Stat(absolute); err != nil || !info.IsDir() {
			return ScanResult{}, NewInvalidRequestError(fmt.Sprintf("%s is not a folder.", folder), "Pass the folder that holds the skill's SKILL.md.")
		}
		files, err := skillscan.CollectFiles(absolute)
		if err != nil {
			return ScanResult{}, fmt.Errorf("read %s: %w", folder, err)
		}
		found, err := skillscan.Audit(files, rules)
		if err != nil {
			return ScanResult{}, NewInvalidRequestError(err.Error(), "Fix the rule in the rules file.")
		}
		names := make([]string, len(files))
		for index, file := range files {
			names[index] = file.Rel
		}
		result.Scans = append(result.Scans, FolderScan{ID: filepath.Base(absolute), Folder: absolute, Verdict: skillscan.Verdict(found), Files: names, Findings: found})
		findings += len(found)
		for _, finding := range found {
			if skillscan.SeverityRank(finding.Severity) >= threshold {
				result.Failed = true
			}
		}
	}
	result.Result = NewResult(StatusOK, fmt.Sprintf("Scanned %s with %d rules: %s.", termCount(len(folders), "folder", "folders"), len(rules), termCount(findings, "finding", "findings")))
	return result, nil
}

// SelfTest runs the examples of the shipped rules and of extraFile's rules.
func (SkillScanService) SelfTest(extraFile string) (SelfTestResult, error) {
	rules, err := loadScanRules(extraFile)
	if err != nil {
		return SelfTestResult{}, err
	}
	failures := skillscan.SelfTest(rules)
	result := SelfTestResult{Rules: len(rules), Failures: failures}
	status, summary := StatusOK, fmt.Sprintf("%d rules, all examples pass.", len(rules))
	if len(failures) > 0 {
		status, summary = StatusActionRequired, fmt.Sprintf("%s fail.", termCount(len(failures), "rule example", "rule examples"))
	}
	result.Result = NewResult(status, summary)
	return result, nil
}
