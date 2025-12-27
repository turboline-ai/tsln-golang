package tsln

import "time"

// BufferedDataPoint represents a single time-series data point
type BufferedDataPoint struct {
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// TSLNFieldType represents the type of a field in the schema
type TSLNFieldType string

const (
	TypeTimestampDelta    TSLNFieldType = "t:delta"
	TypeTimestampInterval TSLNFieldType = "t:interval"
	TypeTimestampAbsolute TSLNFieldType = "t:absolute"
	TypeString            TSLNFieldType = "i:string"
	TypeSymbol            TSLNFieldType = "s:symbol"
	TypeFloat             TSLNFieldType = "f:float"
	TypeInt               TSLNFieldType = "d:int"
	TypeBool              TSLNFieldType = "b:bool"
	TypeEnum              TSLNFieldType = "e:enum"
	TypeArray             TSLNFieldType = "a:array"
	TypeObject            TSLNFieldType = "o:object"
)

// TSLNOptions configuration for TSLN encoding
type TSLNOptions struct {
	// Timestamp options
	TimestampMode  string // "delta", "interval", or "absolute"
	BaseTimestamp  *time.Time

	// Encoding options
	EnableDifferential  bool // default: true
	EnableRepeatMarkers bool // default: true
	EnableRunLength     bool // default: false

	// Formatting options
	Precision       int // default: 2
	MaxStringLength int // default: 0 (unlimited)

	// Schema options
	MaxFields             int  // default: 50
	PrioritizeCompression bool // default: true

	// Performance options
	MinRepeatForRLE int // default: 3
}

// DefaultOptions returns default TSLN options
func DefaultOptions() TSLNOptions {
	return TSLNOptions{
		TimestampMode:         "delta",
		EnableDifferential:    true,
		EnableRepeatMarkers:   true,
		EnableRunLength:       false,
		Precision:             2,
		MaxStringLength:       0,
		MaxFields:             50,
		PrioritizeCompression: true,
		MinRepeatForRLE:       3,
	}
}

// TSLNSchema represents the schema definition
type TSLNSchema struct {
	Version    string
	Fields     []TSLNSchemaField

	// Timestamp configuration
	TimestampMode    string
	BaseTimestamp    *time.Time
	ExpectedInterval int // milliseconds

	// Encoding configuration
	EnableDifferential  bool
	EnableRepeatMarkers bool
	EnableRunLength     bool

	// Metadata
	TotalFields          int
	EstimatedCompression float64
}

// TSLNSchemaField represents a single field in the schema
type TSLNSchemaField struct {
	Name     string
	Type     TSLNFieldType
	Position int

	// Encoding strategies
	UseDifferential  bool
	UseRepeatMarkers bool

	// For enum types
	EnumValues []interface{}

	// Statistics
	RepeatRate float64
	Volatility float64
}

// TSLNResult contains the encoding result and statistics
type TSLNResult struct {
	TSLN     string
	Schema   TSLNSchema
	Analysis DatasetAnalysis
	Stats    TSLNStatistics
}

// TSLNStatistics contains compression statistics
type TSLNStatistics struct {
	OriginalSize         int
	TSLNSize             int
	CompressionRatio     float64
	EstimatedTokens      int
	EstimatedTokenSavings int
}

// DatasetAnalysis contains analysis of the dataset
type DatasetAnalysis struct {
	TotalPoints          int
	FieldAnalyses        map[string]FieldTypeAnalysis

	// Timestamp analysis
	HasTimestamp      bool
	TimestampField    string
	TimestampInterval int // milliseconds
	IsRegularInterval bool
	BaseTimestamp     *time.Time

	// Overall characteristics
	DatasetVolatility      float64
	CompressionPotential   float64
}

// FieldTypeAnalysis contains analysis of a single field
type FieldTypeAnalysis struct {
	FieldName        string
	Type             TSLNFieldType

	// Statistical properties
	UniqueValueCount int
	TotalCount       int
	RepeatRate       float64

	// For numeric types
	IsNumeric  bool
	IsInteger  bool
	Volatility float64
	Trend      string // "increasing", "decreasing", "stable"

	// For categorical types
	TopValues []TopValue

	// Encoding recommendations
	UseDifferential  bool
	UseRepeatMarkers bool
	UseRunLength     bool
}

// TopValue represents a frequently occurring value
type TopValue struct {
	Value interface{}
	Count int
}

// FormatComparison compares different serialization formats
type FormatComparison struct {
	JSON       FormatStats
	CSV        FormatStats
	TOON       FormatStats
	TSLN       FormatStats
	BestFormat string
	Savings    int // percentage
}

// FormatStats contains size and token statistics for a format
type FormatStats struct {
	Size   int
	Tokens int
}
