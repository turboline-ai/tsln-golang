package tsln

import (
	"encoding/json"
	"math"
	"reflect"
	"time"
)

// analyzeDataset analyzes the dataset and returns characteristics
func analyzeDataset(dataPoints []BufferedDataPoint) DatasetAnalysis {
	if len(dataPoints) == 0 {
		return DatasetAnalysis{
			TotalPoints:   0,
			FieldAnalyses: make(map[string]FieldTypeAnalysis),
		}
	}

	// Extract and analyze all fields
	fieldData := extractFieldData(dataPoints)
	fieldAnalyses := make(map[string]FieldTypeAnalysis)

	for fieldName, values := range fieldData {
		fieldAnalyses[fieldName] = analyzeField(fieldName, values)
	}

	// Analyze timestamps
	timestampAnalysis := analyzeTimestamps(dataPoints)

	// Calculate overall metrics
	var numericVolatilities []float64
	for _, analysis := range fieldAnalyses {
		if analysis.IsNumeric && analysis.Volatility > 0 {
			numericVolatilities = append(numericVolatilities, analysis.Volatility)
		}
	}

	datasetVolatility := 0.0
	if len(numericVolatilities) > 0 {
		sum := 0.0
		for _, v := range numericVolatilities {
			sum += v
		}
		datasetVolatility = sum / float64(len(numericVolatilities))
	}

	// Compression potential based on repeat rates
	totalRepeatRate := 0.0
	for _, analysis := range fieldAnalyses {
		totalRepeatRate += analysis.RepeatRate
	}
	avgRepeatRate := totalRepeatRate / float64(len(fieldAnalyses))
	compressionPotential := (avgRepeatRate + (1.0 - math.Min(datasetVolatility, 1.0))) / 2.0

	return DatasetAnalysis{
		TotalPoints:          len(dataPoints),
		FieldAnalyses:        fieldAnalyses,
		HasTimestamp:         timestampAnalysis.HasTimestamp,
		TimestampField:       timestampAnalysis.TimestampField,
		TimestampInterval:    timestampAnalysis.TimestampInterval,
		IsRegularInterval:    timestampAnalysis.IsRegularInterval,
		BaseTimestamp:        timestampAnalysis.BaseTimestamp,
		DatasetVolatility:    datasetVolatility,
		CompressionPotential: compressionPotential,
	}
}

// extractFieldData extracts all field values from dataset
func extractFieldData(dataPoints []BufferedDataPoint) map[string][]interface{} {
	fieldData := make(map[string][]interface{})

	for _, point := range dataPoints {
		flattened := flattenObject(point.Data, "")

		for key, value := range flattened {
			if _, exists := fieldData[key]; !exists {
				fieldData[key] = make([]interface{}, 0)
			}
			fieldData[key] = append(fieldData[key], value)
		}
	}

	return fieldData
}

// analyzeField analyzes a single field
func analyzeField(fieldName string, values []interface{}) FieldTypeAnalysis {
	totalCount := len(values)

	// Filter out nil values
	nonNullValues := make([]interface{}, 0)
	for _, v := range values {
		if v != nil {
			nonNullValues = append(nonNullValues, v)
		}
	}

	// Count unique values
	uniqueValues := make(map[interface{}]bool)
	for _, v := range nonNullValues {
		// Check if value is hashable before using as map key
		// Slices, maps, and functions are not hashable in Go
		hashableValue := v
		if v != nil {
			switch reflect.TypeOf(v).Kind() {
			case reflect.Slice, reflect.Map, reflect.Func:
				// Convert unhashable types to JSON string for uniqueness tracking
				jsonBytes, err := json.Marshal(v)
				if err != nil {
					// If marshaling fails, skip this value
					continue
				}
				hashableValue = string(jsonBytes)
			}
		}
		uniqueValues[hashableValue] = true
	}
	uniqueValueCount := len(uniqueValues)

	repeatRate := 1.0
	if totalCount > 0 {
		repeatRate = 1.0 - (float64(uniqueValueCount) / float64(totalCount))
	}

	// Determine type and characteristics
	analysis := FieldTypeAnalysis{
		FieldName:        fieldName,
		UniqueValueCount: uniqueValueCount,
		TotalCount:       totalCount,
		RepeatRate:       repeatRate,
		IsNumeric:        false,
	}

	if len(nonNullValues) == 0 {
		analysis.Type = TypeString
		return analysis
	}

	// Check if numeric
	sampleValue := nonNullValues[0]
	if _, ok := sampleValue.(float64); ok {
		analysis.IsNumeric = true

		// Check if all values are integers
		isInteger := true
		numericValues := make([]float64, 0)
		for _, v := range nonNullValues {
			if num, ok := v.(float64); ok {
				numericValues = append(numericValues, num)
				if num != math.Floor(num) {
					isInteger = false
				}
			}
		}

		analysis.IsInteger = isInteger
		if isInteger {
			analysis.Type = TypeInt
		} else {
			analysis.Type = TypeFloat
		}

		// Calculate volatility
		analysis.Volatility = calculateVolatility(numericValues)
		analysis.Trend = detectTrend(numericValues)

		// Encoding recommendations
		analysis.UseDifferential = analysis.Volatility < 0.5
	} else if _, ok := sampleValue.(bool); ok {
		analysis.Type = TypeBool
	} else if _, ok := sampleValue.(string); ok {
		// Distinguish between symbols and general strings
		avgLength := 0
		for _, v := range nonNullValues {
			if s, ok := v.(string); ok {
				avgLength += len(s)
			}
		}
		avgLength /= len(nonNullValues)

		if avgLength < 15 && repeatRate > 0.3 {
			analysis.Type = TypeSymbol
		} else {
			analysis.Type = TypeString
		}
	} else {
		analysis.Type = TypeObject
	}

	// Encoding recommendations
	analysis.UseRepeatMarkers = repeatRate > 0.4

	return analysis
}

// analyzeTimestamps analyzes timestamp patterns
func analyzeTimestamps(dataPoints []BufferedDataPoint) struct {
	HasTimestamp      bool
	TimestampField    string
	TimestampInterval int
	IsRegularInterval bool
	BaseTimestamp     *time.Time
} {
	if len(dataPoints) == 0 {
		return struct {
			HasTimestamp      bool
			TimestampField    string
			TimestampInterval int
			IsRegularInterval bool
			BaseTimestamp     *time.Time
		}{HasTimestamp: false}
	}

	baseTimestamp := dataPoints[0].Timestamp

	// Calculate intervals
	intervals := make([]int64, 0)
	for i := 1; i < len(dataPoints); i++ {
		interval := dataPoints[i].Timestamp.Sub(dataPoints[i-1].Timestamp).Milliseconds()
		intervals = append(intervals, interval)
	}

	if len(intervals) == 0 {
		return struct {
			HasTimestamp      bool
			TimestampField    string
			TimestampInterval int
			IsRegularInterval bool
			BaseTimestamp     *time.Time
		}{
			HasTimestamp:      true,
			TimestampField:    "timestamp",
			BaseTimestamp:     &baseTimestamp,
			IsRegularInterval: true,
			TimestampInterval: 0,
		}
	}

	// Calculate average interval
	var sum int64
	for _, interval := range intervals {
		sum += interval
	}
	avgInterval := sum / int64(len(intervals))

	// Check if intervals are regular (within 10% variance)
	var varianceSum float64
	for _, interval := range intervals {
		varianceSum += math.Abs(float64(interval) - float64(avgInterval))
	}
	intervalVariance := varianceSum / float64(len(intervals))
	isRegularInterval := intervalVariance < float64(avgInterval)*0.1

	return struct {
		HasTimestamp      bool
		TimestampField    string
		TimestampInterval int
		IsRegularInterval bool
		BaseTimestamp     *time.Time
	}{
		HasTimestamp:      true,
		TimestampField:    "timestamp",
		TimestampInterval: int(avgInterval),
		IsRegularInterval: isRegularInterval,
		BaseTimestamp:     &baseTimestamp,
	}
}

// calculateVolatility calculates coefficient of variation
func calculateVolatility(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Calculate mean
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	if mean == 0 {
		return 0
	}

	// Calculate variance
	varianceSum := 0.0
	for _, v := range values {
		diff := v - mean
		varianceSum += diff * diff
	}
	variance := varianceSum / float64(len(values))
	stdDev := math.Sqrt(variance)

	return stdDev / math.Abs(mean)
}

// detectTrend detects trend in numeric sequence
func detectTrend(values []float64) string {
	if len(values) < 2 {
		return "stable"
	}

	increases := 0
	decreases := 0

	for i := 1; i < len(values); i++ {
		if values[i] > values[i-1] {
			increases++
		} else if values[i] < values[i-1] {
			decreases++
		}
	}

	increaseRate := float64(increases) / float64(len(values)-1)
	decreaseRate := float64(decreases) / float64(len(values)-1)

	if increaseRate > 0.6 {
		return "increasing"
	}
	if decreaseRate > 0.6 {
		return "decreasing"
	}
	return "stable"
}

// flattenObject flattens nested objects with dot notation
func flattenObject(obj map[string]interface{}, prefix string) map[string]interface{} {
	flattened := make(map[string]interface{})

	for key, value := range obj {
		newKey := key
		if prefix != "" {
			newKey = prefix + "." + key
		}

		if value == nil {
			flattened[newKey] = value
		} else if nested, ok := value.(map[string]interface{}); ok {
			// Recursively flatten nested objects
			for k, v := range flattenObject(nested, newKey) {
				flattened[k] = v
			}
		} else {
			flattened[newKey] = value
		}
	}

	return flattened
}
