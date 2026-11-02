package schemas

import (
	_ "embed"

	"github.com/google/jsonschema-go/jsonschema"
)

// The telemetry and evaluation schemas are public files the tests check; no
// binary reads them at run time, so they are embedded here, not in embed.go.

//go:embed telemetry-event-v1.schema.json
var telemetryEventJSON []byte

//go:embed evaluation-case-v1.schema.json
var evaluationCaseJSON []byte

//go:embed evaluation-suite-v1.schema.json
var evaluationSuiteJSON []byte

//go:embed experiment-manifest-v1.schema.json
var experimentManifestJSON []byte

func TelemetryEvent() (*jsonschema.Schema, error) {
	return decode("telemetry event", telemetryEventJSON)
}

func EvaluationCase() (*jsonschema.Schema, error) {
	return decode("evaluation case", evaluationCaseJSON)
}

func EvaluationSuite() (*jsonschema.Schema, error) {
	return decode("evaluation suite", evaluationSuiteJSON)
}

func ExperimentManifest() (*jsonschema.Schema, error) {
	return decode("experiment manifest", experimentManifestJSON)
}
