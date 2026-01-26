# TSLN - Go Implementation

**TSLN (Time-Series Lean Notation)** - Ultra-compact serialization format for AI-optimized time-series data

[![Go Reference](https://pkg.go.dev/badge/github.com/turboline-ai/tsln-golang.svg)](https://pkg.go.dev/github.com/turboline-ai/tsln-golang)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Overview

TSLN achieves **74% token reduction** compared to JSON through intelligent compression strategies optimized for time-series data sent to Large Language Models (LLMs).

**[Read our whitepaper](https://github.com/turboline-ai/tsln/blob/main/TSLN_Whitepaper.pdf)**

### Key Benefits

- **74% token reduction** vs JSON
- **40% improvement** over TOON format
- **100% lossless** compression
- **<50ms encoding** for 500 data points
- **LLM-compatible** text-based format
- **Zero dependencies** - uses only Go standard library

## Installation

```bash
go get github.com/turboline-ai/tsln-golang
```

## Quick Start

```go
package main

import (
    "fmt"
    "log"
    tsln "github.com/turboline-ai/tsln-golang"
)

func main() {
    // Your time-series data
    data := []tsln.DataPoint{
        {
            Timestamp: "2025-12-27T10:00:00Z",
            Data: map[string]interface{}{
                "symbol": "BTC",
                "price":  50000.0,
                "volume": 1234567,
            },
        },
        {
            Timestamp: "2025-12-27T10:00:01Z",
            Data: map[string]interface{}{
                "symbol": "BTC",
                "price":  50125.50,
                "volume": 1246907,
            },
        },
    }

    // Convert to TSLN
    result, err := tsln.ConvertToTSLN(data)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result.TSLN)
    // Output:
    // # TSLN/1.0
    // # Schema: t:timestamp s:symbol f:price d:volume
    // # Base: 2025-12-27T10:00:00Z
    // # Interval: 1000ms
    // # Encoding: differential, repeat=
    // ---
    // 0|BTC|50000.00|1234567
    // 1|=|+125.50|+12340

    // Check compression stats
    fmt.Printf("Compression: %.0f%%\n", result.Statistics.CompressionRatio*100)
    fmt.Printf("Original tokens: %d\n", result.Statistics.OriginalSize/4)
    fmt.Printf("TSLN tokens: %d\n", result.Statistics.EstimatedTokens)
    fmt.Printf("Token savings: %d\n", result.Statistics.EstimatedTokenSavings)

    // Decode back to original format
    decoded, err := tsln.DecodeTSLN(result.TSLN)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Decoded %d data points\n", len(decoded))
}
```

## Performance

### Compression Results

| Dataset Type | Data Points | Compression | Token Savings |
|--------------|-------------|-------------|---------------|
| Crypto (volatile) | 500 | 74% | 14,832 tokens |
| Stocks (stable) | 400 | 76% | 11,880 tokens |
| News (text) | 50 | 67% | 2,093 tokens |
| IoT Sensors | 1,000 | 75% | 18,500 tokens |

### Cost Analysis (GPT-4)

**Per-Analysis Cost (500 data points):**

| Format | Tokens | Cost | Annual (1K/day) |
|--------|--------|------|-----------------|
| JSON | 20,029 | $0.0401 | $14,616 |
| CSV | 10,416 | $0.0208 | $7,592 |
| TOON | 9,708 | $0.0194 | $7,081 |
| **TSLN** | **5,197** | **$0.0104** | **$3,796** |

**Annual Savings: $10,820 (74% reduction)**

## API Reference

### `ConvertToTSLN(dataPoints []DataPoint, options ...Config) (*TSLNResult, error)`

Converts time-series data to TSLN format.

**Parameters:**
- `dataPoints`: Slice of `DataPoint` structs
- `options`: Optional `Config` for customization (variadic)

**Returns:** `*TSLNResult` containing:
- `TSLN`: Encoded TSLN string
- `Schema`: Generated schema information
- `Statistics`: Compression statistics
- `Analysis`: Dataset characteristics

**Example:**

```go
result, err := tsln.ConvertToTSLN(data, tsln.Config{
    TimestampMode:       tsln.IntervalMode,
    EnableDifferential:  true,
    EnableRepeatMarkers: true,
    Precision:           2,
})
```

### `DecodeTSLN(tslnString string) ([]DataPoint, error)`

Decodes TSLN format back to original data.

**Parameters:**
- `tslnString`: TSLN formatted string

**Returns:** Slice of `DataPoint` structs

**Example:**

```go
originalData, err := tsln.DecodeTSLN(result.TSLN)
if err != nil {
    log.Fatal(err)
}
```

### `CompareFormats(dataPoints []DataPoint) (*FormatComparison, error)`

Compares TSLN with JSON, CSV, and TOON formats.

**Parameters:**
- `dataPoints`: Slice of `DataPoint` structs

**Returns:** `*FormatComparison` with size and token counts

**Example:**

```go
comparison, err := tsln.CompareFormats(data)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Best format: %s\n", comparison.BestFormat)
fmt.Printf("Savings: %d%%\n", comparison.Savings)
```

## Configuration

```go
type Config struct {
    // Timestamp options
    TimestampMode TimestampMode  // Auto, Delta, Interval, Absolute
    BaseTimestamp string

    // Encoding options
    EnableDifferential  bool  // default: true
    EnableRepeatMarkers bool  // default: true

    // Formatting options
    Precision      int  // default: 2
    MaxStringLength int  // default: 0 (unlimited)

    // Schema options
    MaxFields             int  // default: 50
    PrioritizeCompression bool // default: true
}

// Timestamp modes
const (
    AutoMode     TimestampMode = iota  // Auto-detect best mode
    DeltaMode                           // Milliseconds from base
    IntervalMode                        // Regular interval indices
    AbsoluteMode                        // Full ISO-8601 timestamps
)
```

## How It Works

TSLN uses four key strategies:

1. **Relative Timestamps**: `2025-12-27T10:00:00Z` (24 chars) → `0` (1 char)
2. **Differential Encoding**: `50000, 50125, 50250` → `50000, +125, +125`
3. **Repeat Markers**: `BTC, BTC, BTC` → `BTC, =, =`
4. **Schema-First Design**: Define structure once, not per record

## Use Cases

### Ideal For:
- Real-time cryptocurrency/stock analytics
- IoT sensor data analysis
- High-frequency time-series data
- Cost-sensitive AI applications
- Regular interval data streams

### Not Ideal For:
- Deeply nested JSON documents
- One-off single records
- Human-readable exports
- Legacy system integration (without converters)

## Documentation

- **[Full Specification](./docs/README.md)** - Complete technical specification
- **[Quick Reference](./docs/Reference.md)** - Quick start guide with common patterns
- **[GitHub Repository](https://github.com/turboline-ai/tsln-go)** - Source code and examples
- **[Go Package Docs](https://pkg.go.dev/github.com/turboline-ai/tsln-go)** - API documentation

## Testing

Run the test suite:

```bash
go test -v ./...
```

Run benchmarks:

```bash
go test -bench=. -benchmem
```

Run the demo:

```bash
go run examples/demo.go
```

## Building

```bash
# Build
go build

# Install
go install
```

## Contributing

Contributions are welcome! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

## License

MIT License - see [LICENSE](LICENSE) file for details

## About

Developed by the Turboline team to reduce AI token costs by 74% for real-time analytics applications.

**Contact:** dev@turboline.ai

## Links

- **Go Package**: [pkg.go.dev/github.com/turboline-ai/tsln-go](https://pkg.go.dev/github.com/turboline-ai/tsln-go)
- **GitHub**: [turboline-ai/tsln-go](https://github.com/turboline-ai/tsln-go)
- **Issues**: [Report Issues](https://github.com/turboline-ai/tsln-go/issues)
- **Node.js Implementation**: [turboline-ai/tsln-node](https://github.com/turboline-ai/tsln-node)

---

**Made by Turboline**
