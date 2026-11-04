package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/vantt/skillstation/internal/app"
	"github.com/vantt/skillstation/internal/delivery/cli/termui"
)

const scanUsage = "Run `skillstation skill scan <folder>... [--fail-on high] [--rules <file>] [--json]` or `skillstation skill scan --self-test`."

type scanArgs struct {
	folders    []string
	rulesFile  string
	failOn     string
	selfTest   bool
	jsonOutput bool
}

// runSkillScan is `skill scan`: the static security scan of a skill folder. It
// needs no hub, so the author of a skill can run it where the skill is made.
// The exit code is 0 when nothing reaches --fail-on, 1 when a finding does.
func runSkillScan(args []string, stdout, stderr io.Writer) int {
	parsed, err := scanFlags(args)
	if err != nil {
		return writeInvalidRequest(stdout, stderr, hasJSONFlag(args), err.Error(), scanUsage)
	}
	service := app.SkillScanService{}
	if parsed.selfTest {
		result, err := service.SelfTest(parsed.rulesFile)
		if err != nil {
			return writeScanError(stdout, stderr, parsed.jsonOutput, err)
		}
		code := writeResult(stdout, stderr, parsed.jsonOutput, result, func(p *termui.Printer) {
			for _, failure := range result.Failures {
				p.Line(failure)
			}
			p.Line(result.Summary)
		})
		if code == 0 && len(result.Failures) > 0 {
			return 1
		}
		return code
	}
	result, err := service.Scan(parsed.folders, parsed.rulesFile, parsed.failOn)
	if err != nil {
		return writeScanError(stdout, stderr, parsed.jsonOutput, err)
	}
	code := writeResult(stdout, stderr, parsed.jsonOutput, result, func(p *termui.Printer) { renderScan(p, result) })
	if code == 0 && result.Failed {
		return 1
	}
	return code
}

func writeScanError(stdout, stderr io.Writer, jsonOutput bool, err error) int {
	var structured *app.Error
	if errors.As(err, &structured) {
		writeStructuredSkillError(stdout, stderr, jsonOutput, structured)
		return 2
	}
	return writeInvalidRequest(stdout, stderr, jsonOutput, err.Error(), scanUsage)
}

func renderScan(p *termui.Printer, result app.ScanResult) {
	for _, scan := range result.Scans {
		p.Heading(fmt.Sprintf("%s: %s (%d files, %d findings)", scan.ID, scan.Verdict, len(scan.Files), len(scan.Findings)))
		for _, finding := range scan.Findings {
			location := finding.File
			if finding.Line > 0 {
				location += fmt.Sprintf(":%d", finding.Line)
			}
			p.Blank()
			p.Line(fmt.Sprintf("[%s] %s  %s", finding.Severity, finding.Rule, location))
			p.Fields(termui.Field{Label: "Found", Value: finding.Evidence}, termui.Field{Label: "Why", Value: finding.Why}, termui.Field{Label: "Fix", Value: finding.Fix})
		}
	}
	p.Blank()
	p.Line("Static rules catch known patterns only. No findings does not mean the skill is safe.")
}

func scanFlags(args []string) (scanArgs, error) {
	parsed := scanArgs{failOn: "high"}
	for index := 0; index < len(args); index++ {
		switch arg := args[index]; arg {
		case "--rules", "--fail-on":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "--") {
				return parsed, fmt.Errorf("%s requires a value", arg)
			}
			index++
			if arg == "--rules" {
				parsed.rulesFile = args[index]
			} else {
				parsed.failOn = args[index]
			}
		case "--self-test":
			parsed.selfTest = true
		case "--json":
			parsed.jsonOutput = true
		default:
			if strings.HasPrefix(arg, "-") {
				return parsed, fmt.Errorf("unknown argument %q", arg)
			}
			parsed.folders = append(parsed.folders, arg)
		}
	}
	if parsed.selfTest && len(parsed.folders) > 0 {
		return parsed, errors.New("--self-test takes no folder")
	}
	if !parsed.selfTest && len(parsed.folders) == 0 {
		return parsed, errors.New("scan needs a skill folder")
	}
	return parsed, nil
}
