package tsln

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// ConvertToTSLN converts data points to TSLN format
func ConvertToTSLN(dataPoints []BufferedDataPoint, options *TSLNOptions) (*TSLNResult, error) {
	if options == nil {
		opts := DefaultOptions()
		options = &opts
	}

	if len(dataPoints) == 0 {
		return &TSLNResult{
			TSLN:   "# TSLN: No data\n",
			Schema: TSLNSchema{Version: "TSLN/1.0"},
			Analysis: DatasetAnalysis{
				TotalPoints:   0,
				FieldAnalyses: make(map[string]FieldTypeAnalysis),
			},
			Stats: TSLNStatistics{},
		}, nil
	}

	// Analyze dataset
	analysis := analyzeDataset(dataPoints)

	// Generate schema
	schema := generateSchema(analysis, options)

	// Override schema settings with options
	if options.TimestampMode != "" {
		schema.TimestampMode = options.TimestampMode
	}
	if options.BaseTimestamp != nil {
		schema.BaseTimestamp = options.BaseTimestamp
	}
	schema.EnableDifferential = options.EnableDifferential
	schema.EnableRepeatMarkers = options.EnableRepeatMarkers
	schema.EnableRunLength = options.EnableRunLength

	// Convert to TSLN string
	var lines []string

	// Header
	lines = append(lines, generateSchemaHeader(schema)...)
	lines = append(lines, fmt.Sprintf("# Count: %d", len(dataPoints)))
	lines = append(lines, "---")

	// Data rows
	var baseTime time.Time
	if analysis.BaseTimestamp != nil {
		baseTime = *analysis.BaseTimestamp
	} else {
		baseTime = dataPoints[0].Timestamp
	}

	previousValues := make(map[string]interface{})
	previousTimestamp := baseTime

	for i, point := range dataPoints {
		flattened := flattenObject(point.Data, "")
		var rowValues []string

		// Timestamp encoding
		timestampValue := encodeTimestamp(
			point.Timestamp,
			baseTime,
			previousTimestamp,
			schema.TimestampMode,
			schema.ExpectedInterval,
			i,
		)
		rowValues = append(rowValues, timestampValue)

		// Field values
		for _, field := range schema.Fields {
			currentValue := flattened[field.Name]
			previousValue := previousValues[field.Name]

			encodedValue := encodeValue(
				currentValue,
				previousValue,
				&field,
				options,
			)

			rowValues = append(rowValues, encodedValue)
			previousValues[field.Name] = currentValue
		}

		lines = append(lines, strings.Join(rowValues, "|"))
		previousTimestamp = point.Timestamp
	}

	tslnString := strings.Join(lines, "\n")

	// Calculate statistics
	originalSize := estimateJSONSize(dataPoints)
	tslnSize := len(tslnString)
	compressionRatio := float64(originalSize-tslnSize) / float64(originalSize)
	estimatedTokens := int(math.Ceil(float64(tslnSize) / 4.0))
	originalTokens := int(math.Ceil(float64(originalSize) / 4.0))
	estimatedTokenSavings := originalTokens - estimatedTokens

	return &TSLNResult{
		TSLN:     tslnString,
		Schema:   schema,
		Analysis: analysis,
		Stats: TSLNStatistics{
			OriginalSize:          originalSize,
			TSLNSize:              tslnSize,
			CompressionRatio:      compressionRatio,
			EstimatedTokens:       estimatedTokens,
			EstimatedTokenSavings: estimatedTokenSavings,
		},
	}, nil
}

// encodeTimestamp encodes timestamp based on mode
func encodeTimestamp(
	currentTime time.Time,
	baseTime time.Time,
	previousTime time.Time,
	mode string,
	expectedInterval int,
	index int,
) string {
	switch mode {
	case "interval":
		if expectedInterval > 0 {
			expectedTime := baseTime.Add(time.Duration(index*expectedInterval) * time.Millisecond)
			deviation := currentTime.Sub(expectedTime).Milliseconds()

			// If within 5% of expected, just use index
			if math.Abs(float64(deviation)) < float64(expectedInterval)*0.05 {
				return strconv.Itoa(index)
			}

			// Otherwise, show deviation
			if deviation > 0 {
				return fmt.Sprintf("%d+%d", index, deviation)
			}
			return fmt.Sprintf("%d%d", index, deviation)
		}
		return strconv.Itoa(index)

	case "delta":
		delta := currentTime.Sub(baseTime).Milliseconds()
		return strconv.FormatInt(delta, 10)

	case "absolute":
		return currentTime.Format(time.RFC3339Nano)

	default:
		delta := currentTime.Sub(baseTime).Milliseconds()
		return strconv.FormatInt(delta, 10)
	}
}

// encodeValue encodes a field value
func encodeValue(
	currentValue interface{},
	previousValue interface{},
	field *TSLNSchemaField,
	options *TSLNOptions,
) string {
	// Null/undefined
	if currentValue == nil {
		return "∅"
	}

	// Repeat marker
	if options.EnableRepeatMarkers && isEqual(currentValue, previousValue) && previousValue != nil {
		return "="
	}

	// Boolean
	if b, ok := currentValue.(bool); ok {
		if b {
			return "1"
		}
		return "0"
	}

	// Number (float64 in Go)
	if num, ok := currentValue.(float64); ok {
		// Differential encoding
		if options.EnableDifferential && field.UseDifferential {
			if prevNum, ok := previousValue.(float64); ok {
				diff := num - prevNum

				if math.Abs(diff) < math.Abs(num)*0.5 {
					if diff == 0 {
						return "="
					} else if diff > 0 {
						return "+" + formatNumber(diff, options.Precision)
					}
					return formatNumber(diff, options.Precision)
				}
			}
		}

		return formatNumber(num, options.Precision)
	}

	// String
	if str, ok := currentValue.(string); ok {
		if options.MaxStringLength > 0 && len(str) > options.MaxStringLength {
			str = str[:options.MaxStringLength] + "…"
		}
		return strings.ReplaceAll(str, "|", "¦")
	}

	// Default: string representation
	return fmt.Sprintf("%v", currentValue)
}

// formatNumber formats a number with appropriate precision
func formatNumber(value float64, precision int) string {
	if value == math.Floor(value) {
		return strconv.FormatInt(int64(value), 10)
	}
	return strconv.FormatFloat(value, 'f', precision, 64)
}

// isEqual checks if two values are equal
func isEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// EstimateTokens estimates token count for TSLN string
func EstimateTokens(tslnString string) int {
	return int(math.Ceil(float64(len(tslnString)) / 4.0))
}

// CompareFormats compares different serialization formats
func CompareFormats(dataPoints []BufferedDataPoint) (*FormatComparison, error) {
	jsonSize := estimateJSONSize(dataPoints)
	jsonTokens := int(math.Ceil(float64(jsonSize) / 4.0))

	// For now, we'll estimate CSV and TOON sizes
	// Full implementation would require actual converters
	csvSize := int(float64(jsonSize) * 0.52)
	csvTokens := int(math.Ceil(float64(csvSize) / 4.0))

	toonSize := int(float64(jsonSize) * 0.48)
	toonTokens := int(math.Ceil(float64(toonSize) / 4.0))

	result, err := ConvertToTSLN(dataPoints, nil)
	if err != nil {
		return nil, err
	}

	tslnTokens := result.Stats.EstimatedTokens

	// Determine best format
	formats := []struct {
		name   string
		tokens int
	}{
		{"json", jsonTokens},
		{"csv", csvTokens},
		{"toon", toonTokens},
		{"tsln", tslnTokens},
	}

	best := formats[0]
	worst := formats[0]
	for _, f := range formats {
		if f.tokens < best.tokens {
			best = f
		}
		if f.tokens > worst.tokens {
			worst = f
		}
	}

	savings := int(math.Round(float64(worst.tokens-best.tokens) / float64(worst.tokens) * 100))

	return &FormatComparison{
		JSON:       FormatStats{Size: jsonSize, Tokens: jsonTokens},
		CSV:        FormatStats{Size: csvSize, Tokens: csvTokens},
		TOON:       FormatStats{Size: toonSize, Tokens: toonTokens},
		TSLN:       FormatStats{Size: result.Stats.TSLNSize, Tokens: tslnTokens},
		BestFormat: best.name,
		Savings:    savings,
	}, nil
}

// estimateJSONSize estimates the JSON size of data points
func estimateJSONSize(dataPoints []BufferedDataPoint) int {
	// Simplified estimation
	// In a real implementation, we would marshal to JSON and measure
	estimatedSize := 50 // Base array brackets and formatting

	for _, point := range dataPoints {
		estimatedSize += 100 // Timestamp and wrapper
		estimatedSize += estimateObjectSize(point.Data)
	}

	return estimatedSize
}

// estimateObjectSize estimates the size of an object
func estimateObjectSize(obj map[string]interface{}) int {
	size := 10 // Braces and basic formatting

	for key, value := range obj {
		size += len(key) + 5 // Key and quotes/colon

		switch v := value.(type) {
		case string:
			size += len(v) + 2 // Value and quotes
		case float64:
			size += 15 // Approximate number size
		case bool:
			size += 5
		case nil:
			size += 4
		default:
			size += 20 // Estimate for other types
		}
	}

	return size
}
