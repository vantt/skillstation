package evaluation

import (
	"errors"
	"fmt"
	"reflect"
)

// calibrationPolicy is the recorded calibration of the resolver thresholds.
// The tests check the production policy against its selected values.
type calibrationPolicy struct {
	SchemaVersion    int    `json:"schema_version" yaml:"schema_version"`
	Corpus           string `json:"corpus" yaml:"corpus"`
	CalibrationSplit string `json:"calibration_split" yaml:"calibration_split"`
	HeldOutSplit     string `json:"held_out_split" yaml:"held_out_split"`
	CalibrationGrid  struct {
		ApplicabilityFloor []float64 `json:"applicability_floor" yaml:"applicability_floor"`
		MinimumMargin      []float64 `json:"minimum_margin" yaml:"minimum_margin"`
	} `json:"calibration_grid" yaml:"calibration_grid"`
	Selected struct {
		ApplicabilityFloor float64 `json:"applicability_floor" yaml:"applicability_floor"`
		MinimumMargin      float64 `json:"minimum_margin" yaml:"minimum_margin"`
	} `json:"selected" yaml:"selected"`
	HeldOutGates struct {
		ResolvedPrecision float64 `json:"resolved_precision_min" yaml:"resolved_precision_min"`
		NoSkillRecall     float64 `json:"no_skill_abstention_recall_min" yaml:"no_skill_abstention_recall_min"`
		AmbiguityRecall   float64 `json:"ambiguity_recall_min" yaml:"ambiguity_recall_min"`
		OverallAccuracy   float64 `json:"overall_accuracy_min" yaml:"overall_accuracy_min"`
	} `json:"held_out_gates" yaml:"held_out_gates"`
}

// loadCalibrationPolicy reads and strictly parses a recorded calibration.
func loadCalibrationPolicy(path string) (calibrationPolicy, error) {
	data, err := readBounded(path)
	if err != nil {
		return calibrationPolicy{}, fmt.Errorf("read evaluation policy: %w", err)
	}
	return parseCalibrationPolicy(data, formatForPath(path))
}

func parseCalibrationPolicy(data []byte, format Format) (calibrationPolicy, error) {
	if err := validateDocumentSize(data); err != nil {
		return calibrationPolicy{}, err
	}
	var value calibrationPolicy
	if err := strictDecode(data, format, &value); err != nil {
		return calibrationPolicy{}, fmt.Errorf("parse evaluation policy: %w", err)
	}
	if value.SchemaVersion != SchemaVersion || value.Corpus == "" || value.CalibrationSplit == "" || value.HeldOutSplit == "" || len(value.CalibrationGrid.ApplicabilityFloor) == 0 || len(value.CalibrationGrid.MinimumMargin) == 0 {
		return calibrationPolicy{}, errors.New("evaluation policy is incomplete or unsupported")
	}
	if err := validateBounds(reflect.ValueOf(value), "policy"); err != nil {
		return calibrationPolicy{}, err
	}
	return value, nil
}
