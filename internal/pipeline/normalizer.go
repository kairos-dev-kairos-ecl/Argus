package pipeline

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
)

// Normalizer is a Processor that performs field canonicalization and validation.
// It runs after SchemaValidator and ensures all signal fields are in canonical form
// before enrichment. Normalizer does not mutate the input signal; it returns a new instance.
type Normalizer struct {
	logger *zap.Logger
}

// NewNormalizer creates a new Normalizer processor.
func NewNormalizer(logger *zap.Logger) *Normalizer {
	return &Normalizer{
		logger: logger,
	}
}

// Process applies normalization to a signal.
// It canonicalizes provider.name to lowercase, normalizes timestamps to UTC,
// validates data_classification and layer enums, and returns a new signal instance.
// Returns (sig, nil) if normalization succeeds, (nil, error) if validation fails.
func (n *Normalizer) Process(ctx context.Context, sig *v1.ArgusSignal) (*v1.ArgusSignal, error) {
	if sig == nil {
		return nil, nil
	}

	// Create a deep copy of the input signal to avoid mutating it
	normalized := proto.Clone(sig).(*v1.ArgusSignal)

	// Canonicalize provider name to lowercase
	if normalized.Provider != nil && normalized.Provider.Name != "" {
		originalName := normalized.Provider.Name
		normalized.Provider.Name = strings.ToLower(originalName)
		n.logger.Debug("normalized provider name",
			zap.String("signal_id", sig.SignalId),
			zap.String("original", originalName),
			zap.String("canonical", normalized.Provider.Name),
		)
	}

	// Validate and normalize timestamps to UTC
	if normalized.Timestamp != nil {
		ts := normalized.Timestamp.AsTime()
		utcTs := ts.UTC()
		normalized.Timestamp = timestamppb.New(utcTs)
		if !ts.Equal(utcTs) {
			n.logger.Debug("normalized timestamp to UTC",
				zap.String("signal_id", sig.SignalId),
				zap.String("original_tz", ts.Location().String()),
			)
		}
	}

	if normalized.IngestedAt != nil {
		ts := normalized.IngestedAt.AsTime()
		utcTs := ts.UTC()
		normalized.IngestedAt = timestamppb.New(utcTs)
		if !ts.Equal(utcTs) {
			n.logger.Debug("normalized ingested_at to UTC",
				zap.String("signal_id", sig.SignalId),
				zap.String("original_tz", ts.Location().String()),
			)
		}
	}

	// Validate data_classification enum (optional field, but if present, must be valid)
	if normalized.DataClassification != v1.DataClassification_CLASSIFICATION_UNSPECIFIED {
		if !isValidDataClassification(normalized.DataClassification) {
			n.logger.Error("invalid data_classification enum value",
				zap.String("signal_id", sig.SignalId),
				zap.Int32("value", int32(normalized.DataClassification)),
			)
			return nil, fmt.Errorf("invalid data_classification enum value: %d", normalized.DataClassification)
		}
	}

	// Validate layer enum (required, but double-check after SchemaValidator)
	if normalized.Layer == v1.Layer_LAYER_UNSPECIFIED {
		n.logger.Error("layer is unspecified",
			zap.String("signal_id", sig.SignalId),
		)
		return nil, errors.New("layer must not be unspecified")
	}

	if !isValidLayer(normalized.Layer) {
		n.logger.Error("invalid layer enum value",
			zap.String("signal_id", sig.SignalId),
			zap.Int32("value", int32(normalized.Layer)),
		)
		return nil, fmt.Errorf("invalid layer enum value: %d", normalized.Layer)
	}

	return normalized, nil
}

// Name returns the processor's name for logging and metrics.
func (n *Normalizer) Name() string {
	return "Normalizer"
}

// isValidLayer checks if a Layer enum value is valid
func isValidLayer(layer v1.Layer) bool {
	switch layer {
	case v1.Layer_LAYER_UNSPECIFIED,
		v1.Layer_L1_HARDWARE,
		v1.Layer_L2_MODEL_WEIGHTS,
		v1.Layer_L3_TOKENIZER,
		v1.Layer_L4_TRANSFORMER,
		v1.Layer_L5_OUTPUT_DECODING,
		v1.Layer_L6_SAFETY,
		v1.Layer_L7_RAG_RETRIEVAL,
		v1.Layer_L8_AGENTS,
		v1.Layer_L9_API_GATEWAY,
		v1.Layer_L10_APPLICATION:
		return true
	default:
		return false
	}
}
