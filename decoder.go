package tsln

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// DecodeTSLN decodes a TSLN string back to data points
func DecodeTSLN(tslnString string) ([]BufferedDataPoint, error) {
	lines := strings.Split(tslnString, "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty TSLN string")
	}

	// Parse header
	schema, dataStartLine, err := parseHeader(lines)
	if err != nil {
		return nil, fmt.Errorf("failed to parse header: %w", err)
	}

	// Parse data rows
	dataPoints := make([]BufferedDataPoint, 0)
	var previousValues []string
	var previousTimestamp time.Time

	if schema.BaseTimestamp != nil {
		previousTimestamp = *schema.BaseTimestamp
	}

	for i := dataStartLine; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		values := strings.Split(line, "|")
		if len(values) != len(schema.Fields)+1 {
			return nil, fmt.Errorf("line %d: expected %d fields, got %d", i+1, len(schema.Fields)+1, len(values))
		}

		// Decode timestamp
		timestamp, err := decodeTimestamp(
			values[0],
			schema.BaseTimestamp,
			previousTimestamp,
			schema.TimestampMode,
			schema.ExpectedInterval,
			len(dataPoints),
		)
		if err != nil {
			return nil, fmt.Errorf("line %d: failed to decode timestamp: %w", i+1, err)
		}

		// Decode data fields
		data := make(map[string]interface{})
		for j, field := range schema.Fields {
			value, err := decodeValue(
				values[j+1],
				getPreviousValue(previousValues, j),
				field.Type,
			)
			if err != nil {
				return nil, fmt.Errorf("line %d, field %s: %w", i+1, field.Name, err)
			}

			// Unflatten if nested
			setNestedValue(data, field.Name, value)
		}

		dataPoints = append(dataPoints, BufferedDataPoint{
			Timestamp: timestamp,
			Data:      data,
		})

		previousValues = values[1:]
		previousTimestamp = timestamp
	}

	return dataPoints, nil
}

// parseHeader parses TSLN header and returns schema
func parseHeader(lines []string) (*TSLNSchema, int, error) {
	schema := &TSLNSchema{
		Version:       "TSLN/1.0",
		Fields:        make([]TSLNSchemaField, 0),
		TimestampMode: "delta", // Default timestamp mode
	}

	dataStartLine := 0
	for i, line := range lines {
		line = strings.TrimSpace(line)

		if line == "---" {
			dataStartLine = i + 1
			break
		}

		if !strings.HasPrefix(line, "#") {
			continue
		}

		// Remove # and trim
		content := strings.TrimSpace(strings.TrimPrefix(line, "#"))

		// Parse different header lines
		if strings.HasPrefix(content, "TSLN/") {
			schema.Version = content
		} else if strings.HasPrefix(content, "Schema:") {
			schemaContent := strings.TrimPrefix(content, "Schema:")
			schemaContent = strings.TrimSpace(schemaContent)
			fields := strings.Fields(schemaContent)

			position := 0
			for _, field := range fields {
				if strings.HasPrefix(field, "t:") {
					// Skip timestamp field in schema
					continue
				}

				parts := strings.SplitN(field, ":", 2)
				if len(parts) != 2 {
					continue
				}

				fieldType := parseFieldType(parts[0])
				fieldName := parts[1]

				schema.Fields = append(schema.Fields, TSLNSchemaField{
					Name:     fieldName,
					Type:     fieldType,
					Position: position,
				})
				position++
			}
		} else if strings.HasPrefix(content, "Base:") {
			baseStr := strings.TrimSpace(strings.TrimPrefix(content, "Base:"))
			baseTime, err := time.Parse("2006-01-02T15:04:05.000Z07:00", baseStr)
			if err != nil {
				baseTime, err = time.Parse(time.RFC3339, baseStr)
				if err != nil {
					return nil, 0, fmt.Errorf("failed to parse base timestamp: %w", err)
				}
			}
			schema.BaseTimestamp = &baseTime
		} else if strings.HasPrefix(content, "Interval:") {
			intervalStr := strings.TrimSpace(strings.TrimPrefix(content, "Interval:"))
			intervalStr = strings.TrimSuffix(intervalStr, "ms")
			interval, err := strconv.Atoi(intervalStr)
			if err != nil {
				return nil, 0, fmt.Errorf("failed to parse interval: %w", err)
			}
			schema.ExpectedInterval = interval
			schema.TimestampMode = "interval"
		} else if strings.HasPrefix(content, "Encoding:") {
			encodingStr := strings.TrimSpace(strings.TrimPrefix(content, "Encoding:"))
			if strings.Contains(encodingStr, "differential") {
				schema.EnableDifferential = true
			}
			if strings.Contains(encodingStr, "repeat=") {
				schema.EnableRepeatMarkers = true
			}
			if strings.Contains(encodingStr, "run-length") {
				schema.EnableRunLength = true
			}
		}
	}

	if dataStartLine == 0 {
		return nil, 0, fmt.Errorf("no data separator (---) found")
	}

	return schema, dataStartLine, nil
}

// parseFieldType converts type code to TSLNFieldType
func parseFieldType(code string) TSLNFieldType {
	switch code {
	case "t":
		return TypeTimestampDelta
	case "s":
		return TypeSymbol
	case "i":
		return TypeString
	case "f":
		return TypeFloat
	case "d":
		return TypeInt
	case "b":
		return TypeBool
	case "e":
		return TypeEnum
	case "a":
		return TypeArray
	case "o":
		return TypeObject
	default:
		return TypeString
	}
}

// decodeTimestamp decodes timestamp value
func decodeTimestamp(
	value string,
	baseTime *time.Time,
	previousTime time.Time,
	mode string,
	expectedInterval int,
	index int,
) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("empty timestamp value")
	}

	switch mode {
	case "interval":
		if baseTime == nil {
			return time.Time{}, fmt.Errorf("base timestamp required for interval mode")
		}

		// Parse index and deviation
		if strings.Contains(value, "+") || (strings.Contains(value, "-") && value != "0") {
			// Has deviation
			var idx, deviation int
			if strings.Contains(value, "+") {
				parts := strings.SplitN(value, "+", 2)
				idx, _ = strconv.Atoi(parts[0])
				deviation, _ = strconv.Atoi(parts[1])
			} else {
				// Negative deviation
				for i := len(value) - 1; i > 0; i-- {
					if value[i] == '-' {
						idx, _ = strconv.Atoi(value[:i])
						deviation, _ = strconv.Atoi(value[i:])
						break
					}
				}
			}

			expectedTime := baseTime.Add(time.Duration(idx*expectedInterval) * time.Millisecond)
			return expectedTime.Add(time.Duration(deviation) * time.Millisecond), nil
		}

		// Just index
		idx, _ := strconv.Atoi(value)
		return baseTime.Add(time.Duration(idx*expectedInterval) * time.Millisecond), nil

	case "delta":
		if baseTime == nil {
			return time.Time{}, fmt.Errorf("base timestamp required for delta mode")
		}

		delta, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to parse delta: %w", err)
		}

		return baseTime.Add(time.Duration(delta) * time.Millisecond), nil

	case "absolute":
		// Parse absolute timestamp
		timestamp, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to parse absolute timestamp: %w", err)
		}
		return timestamp, nil

	default:
		return time.Time{}, fmt.Errorf("unknown timestamp mode: %s", mode)
	}
}

// decodeValue decodes a field value
func decodeValue(value string, previousValue string, fieldType TSLNFieldType) (interface{}, error) {
	// Handle null
	if value == "∅" || value == "" {
		return nil, nil
	}

	// Handle repeat marker
	if value == "=" {
		if previousValue == "" || previousValue == "∅" {
			return nil, nil
		}
		// Decode previous value recursively
		return decodeValue(previousValue, "", fieldType)
	}

	// Handle boolean
	if fieldType == TypeBool {
		if value == "1" {
			return true, nil
		}
		if value == "0" {
			return false, nil
		}
		return strconv.ParseBool(value)
	}

	// Handle numeric types
	if fieldType == TypeFloat || fieldType == TypeInt {
		// Check for differential encoding
		if strings.HasPrefix(value, "+") || (strings.HasPrefix(value, "-") && value != "0") {
			if previousValue == "" {
				return nil, fmt.Errorf("differential value without previous value")
			}

			prevNum, err := strconv.ParseFloat(previousValue, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse previous value: %w", err)
			}

			diff, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse differential: %w", err)
			}

			return prevNum + diff, nil
		}

		// Absolute value
		num, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse number: %w", err)
		}

		return num, nil
	}

	// String types - restore pipe character
	return strings.ReplaceAll(value, "¦", "|"), nil
}

// getPreviousValue gets previous value safely
func getPreviousValue(previousValues []string, index int) string {
	if index < 0 || index >= len(previousValues) {
		return ""
	}
	return previousValues[index]
}

// setNestedValue sets a value in a nested map using dot notation
func setNestedValue(data map[string]interface{}, key string, value interface{}) {
	if !strings.Contains(key, ".") {
		data[key] = value
		return
	}

	parts := strings.SplitN(key, ".", 2)
	firstKey := parts[0]
	restKey := parts[1]

	if _, exists := data[firstKey]; !exists {
		data[firstKey] = make(map[string]interface{})
	}

	if nested, ok := data[firstKey].(map[string]interface{}); ok {
		setNestedValue(nested, restKey, value)
	}
}
