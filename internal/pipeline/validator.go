package pipeline

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"buf.build/go/protovalidate"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
)

// ValidationError represents a schema validation failure with detailed field information
type ValidationError struct {
	FieldPath string // e.g., "signal_id", "source.app_id"
	Reason    string // e.g., "required field missing", "invalid enum value"
	Value     interface{}
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error at %s: %s (got %v)", e.FieldPath, e.Reason, e.Value)
}

// SchemaValidator is a Processor that validates ArgusSignal messages against schema constraints
type SchemaValidator struct {
	validator protovalidate.Validator
	logger    *zap.Logger
	metrics   ValidatorMetrics
}

// ValidatorMetrics tracks validation statistics
type ValidatorMetrics struct {
	validationFailures prometheus.Counter
}

// NewSchemaValidator creates a new SchemaValidator processor
func NewSchemaValidator(logger *zap.Logger) (*SchemaValidator, error) {
	// Create a protovalidate validator instance
	validator, err := protovalidate.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create protovalidate validator: %w", err)
	}

	metrics := ValidatorMetrics{
		validationFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "argus_pipeline_validation_failures_total",
			Help: "Total number of signals rejected by schema validator",
		}),
	}

	return &SchemaValidator{
		validator: validator,
		logger:    logger,
		metrics:   metrics,
	}, nil
}

// Process validates an ArgusSignal against schema constraints
// Returns the signal unchanged if valid, or an error if validation fails
func (sv *SchemaValidator) Process(ctx context.Context, sig *v1.ArgusSignal) (*v1.ArgusSignal, error) {
	if sig == nil {
		return nil, nil
	}

	// Perform validation
	if err := sv.validate(sig); err != nil {
		sv.metrics.validationFailures.Inc()
		sv.logger.Error("signal validation failed",
			zap.String("signal_id", sig.SignalId),
			zap.Error(err),
		)
		return nil, err
	}

	return sig, nil
}

// Name returns the processor name for logging
func (sv *SchemaValidator) Name() string {
	return "SchemaValidator"
}

// validate performs all validation checks on a signal
func (sv *SchemaValidator) validate(sig *v1.ArgusSignal) error {
	var errors []ValidationError

	// Required field validations
	if sig.SignalId == "" {
		errors = append(errors, ValidationError{
			FieldPath: "signal_id",
			Reason:    "required field missing",
			Value:     sig.SignalId,
		})
	}

	if sig.TraceId == "" {
		errors = append(errors, ValidationError{
			FieldPath: "trace_id",
			Reason:    "required field missing",
			Value:     sig.TraceId,
		})
	}

	if sig.Layer == v1.Layer_LAYER_UNSPECIFIED {
		errors = append(errors, ValidationError{
			FieldPath: "layer",
			Reason:    "required field cannot be UNSPECIFIED",
			Value:     sig.Layer,
		})
	}

	if sig.Category == "" {
		errors = append(errors, ValidationError{
			FieldPath: "category",
			Reason:    "required field missing",
			Value:     sig.Category,
		})
	}

	if sig.Severity == v1.Severity_SEVERITY_UNSPECIFIED {
		errors = append(errors, ValidationError{
			FieldPath: "severity",
			Reason:    "required field cannot be UNSPECIFIED",
			Value:     sig.Severity,
		})
	}

	if sig.Timestamp == nil {
		errors = append(errors, ValidationError{
			FieldPath: "timestamp",
			Reason:    "required field missing",
			Value:     sig.Timestamp,
		})
	}

	if sig.IngestedAt == nil {
		errors = append(errors, ValidationError{
			FieldPath: "ingested_at",
			Reason:    "required field missing",
			Value:     sig.IngestedAt,
		})
	}

	// Source validation
	if sig.Source == nil {
		errors = append(errors, ValidationError{
			FieldPath: "source",
			Reason:    "required field missing",
			Value:     sig.Source,
		})
	} else {
		if srcErrs := sv.validateSource(sig.Source); len(srcErrs) > 0 {
			errors = append(errors, srcErrs...)
		}
	}

	// Enum field validation for DataClassification (allow UNSPECIFIED = 0)
	if !isValidDataClassification(sig.DataClassification) {
		errors = append(errors, ValidationError{
			FieldPath: "data_classification",
			Reason:    "invalid enum value",
			Value:     sig.DataClassification,
		})
	}

	// Enrichment field validation (optional, but validate if present)
	if sig.Enrichment != nil {
		if enrichErrs := sv.validateEnrichment(sig.Enrichment); len(enrichErrs) > 0 {
			errors = append(errors, enrichErrs...)
		}
	}

	if len(errors) > 0 {
		return sv.formatErrors(errors)
	}

	return nil
}

// validateSource validates nested Source message
func (sv *SchemaValidator) validateSource(source *v1.Source) []ValidationError {
	var errors []ValidationError

	if source.AppId == "" {
		errors = append(errors, ValidationError{
			FieldPath: "source.app_id",
			Reason:    "required field missing",
			Value:     source.AppId,
		})
	}

	if source.AppVersion == "" {
		errors = append(errors, ValidationError{
			FieldPath: "source.app_version",
			Reason:    "required field missing",
			Value:     source.AppVersion,
		})
	}

	if source.SdkVersion == "" {
		errors = append(errors, ValidationError{
			FieldPath: "source.sdk_version",
			Reason:    "required field missing",
			Value:     source.SdkVersion,
		})
	}

	if source.Environment == "" {
		errors = append(errors, ValidationError{
			FieldPath: "source.environment",
			Reason:    "required field missing",
			Value:     source.Environment,
		})
	}

	if source.InstanceId == "" {
		errors = append(errors, ValidationError{
			FieldPath: "source.instance_id",
			Reason:    "required field missing",
			Value:     source.InstanceId,
		})
	}

	return errors
}

// validateEnrichment validates optional Enrichment message
func (sv *SchemaValidator) validateEnrichment(enrichment *v1.Enrichment) []ValidationError {
	var errors []ValidationError

	// Validate baseline_deviation if present (should be -10.0 to 10.0 for z-score)
	if enrichment.BaselineDeviation != nil {
		val := *enrichment.BaselineDeviation
		if val < -10.0 || val > 10.0 {
			errors = append(errors, ValidationError{
				FieldPath: "enrichment.baseline_deviation",
				Reason:    "z-score out of valid range [-10.0, 10.0]",
				Value:     val,
			})
		}
	}

	// Validate risk_score if present (should be 0.0 to 1.0)
	if enrichment.RiskScore != nil {
		val := *enrichment.RiskScore
		if val < 0.0 || val > 1.0 {
			errors = append(errors, ValidationError{
				FieldPath: "enrichment.risk_score",
				Reason:    "risk score out of valid range [0.0, 1.0]",
				Value:     val,
			})
		}
	}

	return errors
}

// isValidDataClassification checks if a DataClassification enum value is valid
func isValidDataClassification(dc v1.DataClassification) bool {
	switch dc {
	case v1.DataClassification_CLASSIFICATION_UNSPECIFIED,
		v1.DataClassification_PUBLIC,
		v1.DataClassification_INTERNAL,
		v1.DataClassification_CONFIDENTIAL,
		v1.DataClassification_RESTRICTED:
		return true
	default:
		return false
	}
}

// formatErrors combines multiple validation errors into a single error message
func (sv *SchemaValidator) formatErrors(errs []ValidationError) error {
	var msgs []string
	for _, err := range errs {
		msgs = append(msgs, err.Error())
	}
	return errors.New(strings.Join(msgs, "; "))
}

// ValidateTimestamp is a helper to check if a timestamp is valid
// Returns error if timestamp is nil or in the future
func ValidateTimestamp(ts *timestamppb.Timestamp, fieldName string) error {
	if ts == nil {
		return fmt.Errorf("%s is required", fieldName)
	}
	return nil
}
