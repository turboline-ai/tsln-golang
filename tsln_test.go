package tsln

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertToTSLN_Empty(t *testing.T) {
	result, err := ConvertToTSLN([]BufferedDataPoint{}, nil)
	require.NoError(t, err)
	assert.Contains(t, result.TSLN, "No data")
}

func TestConvertToTSLN_SimpleData(t *testing.T) {
	baseTime := time.Date(2025, 12, 27, 10, 0, 0, 0, time.UTC)

	data := []BufferedDataPoint{
		{
			Timestamp: baseTime,
			Data: map[string]interface{}{
				"symbol": "BTC",
				"price":  50000.0,
				"volume": 1234567.0,
			},
		},
		{
			Timestamp: baseTime.Add(1 * time.Second),
			Data: map[string]interface{}{
				"symbol": "BTC",
				"price":  50125.50,
				"volume": 1246907.0,
			},
		},
	}

	result, err := ConvertToTSLN(data, nil)
	require.NoError(t, err)

	// Check header
	assert.Contains(t, result.TSLN, "# TSLN/1.0")
	assert.Contains(t, result.TSLN, "# Schema:")
	assert.Contains(t, result.TSLN, "---")

	// Check statistics
	assert.Greater(t, result.Stats.CompressionRatio, 0.0)
	assert.Greater(t, result.Stats.OriginalSize, result.Stats.TSLNSize)
	assert.Greater(t, result.Stats.EstimatedTokenSavings, 0)
}

func TestConvertToTSLN_WithRepeatMarkers(t *testing.T) {
	baseTime := time.Date(2025, 12, 27, 10, 0, 0, 0, time.UTC)

	data := []BufferedDataPoint{
		{
			Timestamp: baseTime,
			Data: map[string]interface{}{
				"symbol": "BTC",
				"value":  100.0,
			},
		},
		{
			Timestamp: baseTime.Add(1 * time.Second),
			Data: map[string]interface{}{
				"symbol": "BTC",
				"value":  100.0,
			},
		},
		{
			Timestamp: baseTime.Add(2 * time.Second),
			Data: map[string]interface{}{
				"symbol": "ETH",
				"value":  100.0,
			},
		},
	}

	opts := DefaultOptions()
	opts.EnableRepeatMarkers = true

	result, err := ConvertToTSLN(data, &opts)
	require.NoError(t, err)

	// Should contain repeat marker
	assert.Contains(t, result.TSLN, "=")
}

func TestConvertToTSLN_WithDifferential(t *testing.T) {
	baseTime := time.Date(2025, 12, 27, 10, 0, 0, 0, time.UTC)

	data := []BufferedDataPoint{
		{
			Timestamp: baseTime,
			Data: map[string]interface{}{
				"value": 1000.0,
			},
		},
		{
			Timestamp: baseTime.Add(1 * time.Second),
			Data: map[string]interface{}{
				"value": 1050.0,
			},
		},
		{
			Timestamp: baseTime.Add(2 * time.Second),
			Data: map[string]interface{}{
				"value": 1025.0,
			},
		},
	}

	opts := DefaultOptions()
	opts.EnableDifferential = true

	result, err := ConvertToTSLN(data, &opts)
	require.NoError(t, err)

	// Should contain differential encoding
	assert.Contains(t, result.TSLN, "+")
}

func TestConvertToTSLN_RegularInterval(t *testing.T) {
	baseTime := time.Date(2025, 12, 27, 10, 0, 0, 0, time.UTC)

	data := []BufferedDataPoint{
		{
			Timestamp: baseTime,
			Data:      map[string]interface{}{"value": 1.0},
		},
		{
			Timestamp: baseTime.Add(1 * time.Second),
			Data:      map[string]interface{}{"value": 2.0},
		},
		{
			Timestamp: baseTime.Add(2 * time.Second),
			Data:      map[string]interface{}{"value": 3.0},
		},
	}

	result, err := ConvertToTSLN(data, nil)
	require.NoError(t, err)

	// Should detect regular interval
	assert.True(t, result.Analysis.IsRegularInterval)
	assert.Equal(t, 1000, result.Analysis.TimestampInterval)
}

func TestDecodeTSLN_Simple(t *testing.T) {
	tslnString := `# TSLN/1.0
# Schema: t:timestamp s:symbol f:price d:volume
# Base: 2025-12-27T10:00:00.000Z
# Interval: 1000ms
# Encoding: differential, repeat=
# Count: 2
---
0|BTC|50000.00|1234567
1|=|+125.50|+12340`

	dataPoints, err := DecodeTSLN(tslnString)
	require.NoError(t, err)
	require.Len(t, dataPoints, 2)

	// Check first point
	assert.Equal(t, "BTC", dataPoints[0].Data["symbol"])
	assert.Equal(t, 50000.0, dataPoints[0].Data["price"])
	assert.Equal(t, 1234567.0, dataPoints[0].Data["volume"])

	// Check second point
	assert.Equal(t, "BTC", dataPoints[1].Data["symbol"]) // Repeat marker
	assert.Equal(t, 50125.50, dataPoints[1].Data["price"]) // Differential
	assert.Equal(t, 1246907.0, dataPoints[1].Data["volume"]) // Differential
}

func TestRoundTrip(t *testing.T) {
	baseTime := time.Date(2025, 12, 27, 10, 0, 0, 0, time.UTC)

	originalData := []BufferedDataPoint{
		{
			Timestamp: baseTime,
			Data: map[string]interface{}{
				"symbol": "BTC",
				"price":  50000.0,
				"volume": 1234567.0,
				"active": true,
			},
		},
		{
			Timestamp: baseTime.Add(1 * time.Second),
			Data: map[string]interface{}{
				"symbol": "BTC",
				"price":  50125.50,
				"volume": 1246907.0,
				"active": true,
			},
		},
		{
			Timestamp: baseTime.Add(2 * time.Second),
			Data: map[string]interface{}{
				"symbol": "ETH",
				"price":  3000.0,
				"volume": 789012.0,
				"active": false,
			},
		},
	}

	// Encode
	result, err := ConvertToTSLN(originalData, nil)
	require.NoError(t, err)

	// Decode
	decodedData, err := DecodeTSLN(result.TSLN)
	require.NoError(t, err)
	require.Len(t, decodedData, len(originalData))

	// Compare timestamps (within millisecond precision)
	for i := range originalData {
		assert.WithinDuration(t, originalData[i].Timestamp, decodedData[i].Timestamp, time.Millisecond)
		
		// Compare data fields
		for key, originalValue := range originalData[i].Data {
			decodedValue, exists := decodedData[i].Data[key]
			assert.True(t, exists, "field %s should exist", key)
			
			// Handle float comparison
			if origFloat, ok := originalValue.(float64); ok {
				if decFloat, ok := decodedValue.(float64); ok {
					assert.InDelta(t, origFloat, decFloat, 0.01)
				}
			} else {
				assert.Equal(t, originalValue, decodedValue, "field %s should match", key)
			}
		}
	}
}

func TestAnalyzeDataset(t *testing.T) {
	baseTime := time.Date(2025, 12, 27, 10, 0, 0, 0, time.UTC)

	data := []BufferedDataPoint{
		{
			Timestamp: baseTime,
			Data: map[string]interface{}{
				"symbol": "BTC",
				"price":  50000.0,
			},
		},
		{
			Timestamp: baseTime.Add(1 * time.Second),
			Data: map[string]interface{}{
				"symbol": "BTC",
				"price":  50050.0,
			},
		},
		{
			Timestamp: baseTime.Add(2 * time.Second),
			Data: map[string]interface{}{
				"symbol": "ETH",
				"price":  3000.0,
			},
		},
	}

	analysis := analyzeDataset(data)
	
	assert.Equal(t, 3, analysis.TotalPoints)
	assert.True(t, analysis.HasTimestamp)
	assert.True(t, analysis.IsRegularInterval)
	assert.Equal(t, 1000, analysis.TimestampInterval) // 1 second in ms
	assert.NotNil(t, analysis.BaseTimestamp)
	
	// Check field analysis
	assert.Contains(t, analysis.FieldAnalyses, "symbol")
	assert.Contains(t, analysis.FieldAnalyses, "price")
	
	symbolAnalysis := analysis.FieldAnalyses["symbol"]
	assert.Equal(t, TypeSymbol, symbolAnalysis.Type)
	assert.True(t, symbolAnalysis.RepeatRate > 0)
	
	priceAnalysis := analysis.FieldAnalyses["price"]
	assert.True(t, priceAnalysis.IsNumeric)
	// Price could be int or float depending on values
	assert.Contains(t, []TSLNFieldType{TypeInt, TypeFloat}, priceAnalysis.Type)
}

func TestCompareFormats(t *testing.T) {
	baseTime := time.Date(2025, 12, 27, 10, 0, 0, 0, time.UTC)

	data := []BufferedDataPoint{
		{
			Timestamp: baseTime,
			Data: map[string]interface{}{
				"symbol": "BTC",
				"price":  50000.0,
			},
		},
		{
			Timestamp: baseTime.Add(1 * time.Second),
			Data: map[string]interface{}{
				"symbol": "BTC",
				"price":  50100.0,
			},
		},
	}

	comparison, err := CompareFormats(data)
	require.NoError(t, err)
	
	assert.Greater(t, comparison.JSON.Size, comparison.TSLN.Size)
	assert.Greater(t, comparison.JSON.Tokens, comparison.TSLN.Tokens)
	assert.Greater(t, comparison.Savings, 0)
	assert.NotEmpty(t, comparison.BestFormat)
}

func TestEstimateTokens(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected int
	}{
		{"Empty", "", 0},
		{"Short", "test", 1},
		{"Medium", "test string here", 4},
		{"Long", "this is a longer test string that should be multiple tokens", 15},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tokens := EstimateTokens(tc.input)
			assert.Equal(t, tc.expected, tokens)
		})
	}
}

func TestGetExplanation(t *testing.T) {
	baseTime := time.Date(2025, 12, 27, 10, 0, 0, 0, time.UTC)
	
	schema := TSLNSchema{
		Version:             "TSLN/1.0",
		TimestampMode:       "interval",
		BaseTimestamp:       &baseTime,
		EnableDifferential:  true,
		EnableRepeatMarkers: true,
		Fields: []TSLNSchemaField{
			{Name: "symbol", Type: TypeSymbol},
			{Name: "price", Type: TypeFloat},
		},
		EstimatedCompression: 0.74,
	}

	explanation := GetExplanation(schema)
	
	assert.Contains(t, explanation, "TSLN")
	assert.Contains(t, explanation, "differential")
	assert.Contains(t, explanation, "repeat")
	assert.Contains(t, explanation, "74%")
}

func TestNestedObjects(t *testing.T) {
	baseTime := time.Date(2025, 12, 27, 10, 0, 0, 0, time.UTC)

	data := []BufferedDataPoint{
		{
			Timestamp: baseTime,
			Data: map[string]interface{}{
				"user": map[string]interface{}{
					"name": "Alice",
					"age":  30.0,
				},
				"status": "active",
			},
		},
	}

	result, err := ConvertToTSLN(data, nil)
	require.NoError(t, err)
	
	// Should flatten nested objects
	assert.Contains(t, result.TSLN, "user.name")
	assert.Contains(t, result.TSLN, "user.age")
	assert.Contains(t, result.TSLN, "status")
}

func TestNullValues(t *testing.T) {
	baseTime := time.Date(2025, 12, 27, 10, 0, 0, 0, time.UTC)

	data := []BufferedDataPoint{
		{
			Timestamp: baseTime,
			Data: map[string]interface{}{
				"value":   100.0,
				"optional": nil,
			},
		},
		{
			Timestamp: baseTime.Add(1 * time.Second),
			Data: map[string]interface{}{
				"value":   200.0,
				"optional": "present",
			},
		},
	}

	result, err := ConvertToTSLN(data, nil)
	require.NoError(t, err)
	assert.Contains(t, result.TSLN, "∅")
}

func TestConvertToTSLN_WithArrayFields(t *testing.T) {
	baseTime := time.Date(2025, 12, 27, 10, 0, 0, 0, time.UTC)

	// Test with array/slice fields that previously caused panic
	data := []BufferedDataPoint{
		{
			Timestamp: baseTime,
			Data: map[string]interface{}{
				"name":        "test1",
				"tags":        []string{"tag1", "tag2"},
				"items":       []interface{}{1, 2, 3},
				"product_ids": []string{"BTC-USD", "ETH-USD"},
			},
		},
		{
			Timestamp: baseTime.Add(1 * time.Second),
			Data: map[string]interface{}{
				"name":        "test2",
				"tags":        []string{"tag3", "tag4"},
				"items":       []interface{}{4, 5, 6},
				"product_ids": []string{"BTC-USD", "ETH-USD"},
			},
		},
	}

	// This should not panic
	result, err := ConvertToTSLN(data, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, result.TSLN)
	assert.Contains(t, result.TSLN, "# TSLN/1.0")
}

func TestConvertToTSLN_WithMapFields(t *testing.T) {
	baseTime := time.Date(2025, 12, 27, 10, 0, 0, 0, time.UTC)

	// Test with map fields that are also unhashable
	data := []BufferedDataPoint{
		{
			Timestamp: baseTime,
			Data: map[string]interface{}{
				"name": "test",
				"metadata": map[string]interface{}{
					"key1": "value1",
				},
			},
		},
	}

	// This should not panic
	result, err := ConvertToTSLN(data, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, result.TSLN)
}

func BenchmarkConvertToTSLN(b *testing.B) {
	baseTime := time.Date(2025, 12, 27, 10, 0, 0, 0, time.UTC)
	data := make([]BufferedDataPoint, 500)

	for i := 0; i < 500; i++ {
		data[i] = BufferedDataPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Data: map[string]interface{}{
				"symbol": "BTC",
				"price":  50000.0 + float64(i)*10.0,
				"volume": 1234567.0 + float64(i)*1000.0,
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ConvertToTSLN(data, nil)
	}
}

func BenchmarkDecodeTSLN(b *testing.B) {
	baseTime := time.Date(2025, 12, 27, 10, 0, 0, 0, time.UTC)
	data := make([]BufferedDataPoint, 100)

	for i := 0; i < 100; i++ {
		data[i] = BufferedDataPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Data: map[string]interface{}{
				"symbol": "BTC",
				"price":  50000.0 + float64(i)*10.0,
				"volume": 1234567.0 + float64(i)*1000.0,
			},
		}
	}

	result, _ := ConvertToTSLN(data, nil)
	tslnString := result.TSLN

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DecodeTSLN(tslnString)
	}
}
