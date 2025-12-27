package tsln

import (
	"fmt"
	"sort"
	"strings"
)

// generateSchema generates optimal TSLN schema from dataset analysis
func generateSchema(analysis DatasetAnalysis, options *TSLNOptions) TSLNSchema {
	maxFields := options.MaxFields
	prioritizeCompression := options.PrioritizeCompression

	schema := TSLNSchema{
		Version: "TSLN/1.0",
		Fields:  make([]TSLNSchemaField, 0),
	}

	// Sort fields for optimal ordering
	sortedFields := sortFieldsForSchema(analysis.FieldAnalyses, prioritizeCompression)

	// Limit fields if necessary
	fieldsToInclude := sortedFields
	if len(sortedFields) > maxFields {
		fieldsToInclude = sortedFields[:maxFields]
	}

	position := 0
	for _, fieldAnalysis := range fieldsToInclude {
		schema.Fields = append(schema.Fields, TSLNSchemaField{
			Name:             fieldAnalysis.FieldName,
			Type:             fieldAnalysis.Type,
			Position:         position,
			UseDifferential:  fieldAnalysis.UseDifferential,
			UseRepeatMarkers: fieldAnalysis.UseRepeatMarkers,
			RepeatRate:       fieldAnalysis.RepeatRate,
			Volatility:       fieldAnalysis.Volatility,
		})
		position++
	}

	// Determine timestamp mode
	timestampMode := "delta"
	if analysis.IsRegularInterval && analysis.TimestampInterval > 0 {
		timestampMode = "interval"
	}

	schema.TimestampMode = timestampMode
	schema.BaseTimestamp = analysis.BaseTimestamp
	schema.ExpectedInterval = analysis.TimestampInterval

	// Set encoding flags
	schema.EnableDifferential = false
	schema.EnableRepeatMarkers = false
	for _, field := range schema.Fields {
		if field.UseDifferential {
			schema.EnableDifferential = true
		}
		if field.UseRepeatMarkers {
			schema.EnableRepeatMarkers = true
		}
	}

	// Calculate overall compression estimate
	compressionEstimates := make([]float64, 0)
	for _, field := range schema.Fields {
		estimate := 0.0
		if field.UseDifferential {
			estimate += 0.3
		}
		if field.UseRepeatMarkers && field.RepeatRate > 0 {
			estimate += field.RepeatRate * 0.4
		}
		compressionEstimates = append(compressionEstimates, estimate)
	}

	estimatedCompression := 0.0
	if len(compressionEstimates) > 0 {
		sum := 0.0
		for _, e := range compressionEstimates {
			sum += e
		}
		estimatedCompression = sum / float64(len(compressionEstimates))
	}

	schema.TotalFields = len(schema.Fields)
	schema.EstimatedCompression = estimatedCompression

	return schema
}

// sortFieldsForSchema sorts fields for optimal schema ordering
func sortFieldsForSchema(
	fieldAnalyses map[string]FieldTypeAnalysis,
	prioritizeCompression bool,
) []FieldTypeAnalysis {
	fields := make([]FieldTypeAnalysis, 0, len(fieldAnalyses))
	for _, analysis := range fieldAnalyses {
		fields = append(fields, analysis)
	}

	if !prioritizeCompression {
		sort.Slice(fields, func(i, j int) bool {
			return fields[i].FieldName < fields[j].FieldName
		})
		return fields
	}

	// Sort by compression score
	sort.Slice(fields, func(i, j int) bool {
		scoreA := calculateCompressionScore(fields[i])
		scoreB := calculateCompressionScore(fields[j])
		return scoreB < scoreA // Higher score first
	})

	return fields
}

// calculateCompressionScore calculates compression score for field ordering
func calculateCompressionScore(field FieldTypeAnalysis) float64 {
	score := 0.0

	// High repeat rate = better compression
	score += field.RepeatRate * 50

	// Differential encoding potential
	if field.UseDifferential {
		score += 30
	}

	// Low volatility = better differential compression
	if field.Volatility > 0 {
		score += (1.0 - field.Volatility) * 20
	}

	return score
}

// generateSchemaHeader generates schema header string for TSLN output
func generateSchemaHeader(schema TSLNSchema) []string {
	lines := make([]string, 0)

	lines = append(lines, fmt.Sprintf("# %s", schema.Version))

	// Schema definition
	schemaFields := make([]string, 0)
	for _, field := range schema.Fields {
		// Format: typeCode:fieldName
		typeCode := strings.Split(string(field.Type), ":")[0]
		schemaFields = append(schemaFields, fmt.Sprintf("%s:%s", typeCode, field.Name))
	}

	lines = append(lines, fmt.Sprintf("# Schema: t:timestamp %s", strings.Join(schemaFields, " ")))

	// Timestamp configuration
	if schema.BaseTimestamp != nil {
		lines = append(lines, fmt.Sprintf("# Base: %s", schema.BaseTimestamp.Format("2006-01-02T15:04:05.000Z07:00")))
	}
	if schema.TimestampMode == "interval" && schema.ExpectedInterval > 0 {
		lines = append(lines, fmt.Sprintf("# Interval: %dms", schema.ExpectedInterval))
	}

	// Encoding strategies
	strategies := make([]string, 0)
	if schema.EnableDifferential {
		strategies = append(strategies, "differential")
	}
	if schema.EnableRepeatMarkers {
		strategies = append(strategies, "repeat=")
	}
	if schema.EnableRunLength {
		strategies = append(strategies, "run-length")
	}

	if len(strategies) > 0 {
		lines = append(lines, fmt.Sprintf("# Encoding: %s", strings.Join(strategies, ", ")))
	}

	return lines
}

// GetExplanation returns TSLN format explanation for AI
func GetExplanation(schema TSLNSchema) string {
	strategies := make([]string, 0)
	if schema.EnableDifferential {
		strategies = append(strategies, "differential encoding (±values)")
	}
	if schema.EnableRepeatMarkers {
		strategies = append(strategies, "repeat markers (=)")
	}
	if schema.EnableRunLength {
		strategies = append(strategies, "run-length (*N)")
	}

	baseTime := "N/A"
	if schema.BaseTimestamp != nil {
		baseTime = schema.BaseTimestamp.Format("2006-01-02T15:04:05Z")
	}

	strategiesStr := "standard"
	if len(strategies) > 0 {
		strategiesStr = strings.Join(strategies, ", ")
	}

	return fmt.Sprintf(`Data Format: TSLN (Time-Series Lean Notation)
Version: %s
Structure: Schema-first, pipe-delimited positional values
Timestamp Mode: %s (base: %s)
Fields: %d columns
Encoding: %s
Symbols: ∅=null, 1/0=boolean, +=increase, -=decrease, ==repeat
Benefits: ~75%% more compact than JSON, ~40%% more compact than TOON
Estimated Compression: %d%%`,
		schema.Version,
		schema.TimestampMode,
		baseTime,
		len(schema.Fields),
		strategiesStr,
		int(schema.EstimatedCompression*100),
	)
}
