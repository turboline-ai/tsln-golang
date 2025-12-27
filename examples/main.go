package main

import (
	"fmt"
	"log"
	"time"

	tsln "github.com/turboline-ai/tsln-golang"
)

func main() {
	fmt.Println("=== TSLN Golang Example ===\n")

	// Example 1: Basic usage
	basicExample()
	fmt.Println("\n" + string(rune(0x2500)*50) + "\n")

	// Example 2: Cryptocurrency data
	cryptoExample()
	fmt.Println("\n" + string(rune(0x2500)*50) + "\n")

	// Example 3: IoT sensor data
	iotExample()
	fmt.Println("\n" + string(rune(0x2500)*50) + "\n")

	// Example 4: Format comparison
	formatComparisonExample()
}

func basicExample() {
	fmt.Println("📊 Example 1: Basic Time-Series Data")
	fmt.Println("=====================================")

	baseTime := time.Date(2025, 12, 27, 10, 0, 0, 0, time.UTC)

	data := []tsln.BufferedDataPoint{
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
		{
			Timestamp: baseTime.Add(2 * time.Second),
			Data: map[string]interface{}{
				"symbol": "BTC",
				"price":  50100.25,
				"volume": 1240000.0,
			},
		},
	}

	result, err := tsln.ConvertToTSLN(data, nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n🔹 TSLN Output:")
	fmt.Println(result.TSLN)

	fmt.Printf("\n📈 Statistics:\n")
	fmt.Printf("  • Compression: %.1f%%\n", result.Stats.CompressionRatio*100)
	fmt.Printf("  • Original size: %d bytes (%d tokens)\n",
		result.Stats.OriginalSize, result.Stats.OriginalSize/4)
	fmt.Printf("  • TSLN size: %d bytes (%d tokens)\n",
		result.Stats.TSLNSize, result.Stats.EstimatedTokens)
	fmt.Printf("  • Token savings: %d tokens\n", result.Stats.EstimatedTokenSavings)

	// Decode back
	decoded, err := tsln.DecodeTSLN(result.TSLN)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\n✅ Successfully decoded %d data points\n", len(decoded))
}

func cryptoExample() {
	fmt.Println("💰 Example 2: Cryptocurrency Trading Data")
	fmt.Println("==========================================")

	baseTime := time.Now().Truncate(time.Second)

	// Generate volatile crypto data
	data := make([]tsln.BufferedDataPoint, 10)
	prices := []float64{50000, 50125, 49950, 50200, 50175, 50050, 50300, 50250, 50100, 50400}

	for i := 0; i < 10; i++ {
		data[i] = tsln.BufferedDataPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Data: map[string]interface{}{
				"symbol":    "BTC",
				"price":     prices[i],
				"volume":    1234567.0 + float64(i)*12340,
				"marketCap": 985000000000.0,
				"change24h": 2.5,
			},
		}
	}

	opts := tsln.DefaultOptions()
	opts.EnableDifferential = true
	opts.EnableRepeatMarkers = true

	result, err := tsln.ConvertToTSLN(data, &opts)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n🔹 TSLN Output (first 10 lines):")
	lines := splitLines(result.TSLN, 10)
	for _, line := range lines {
		fmt.Println(line)
	}

	fmt.Printf("\n📈 Compression Analysis:\n")
	fmt.Printf("  • Data points: %d\n", len(data))
	fmt.Printf("  • Fields per point: %d\n", len(result.Schema.Fields))
	fmt.Printf("  • Compression ratio: %.1f%%\n", result.Stats.CompressionRatio*100)
	fmt.Printf("  • Timestamp mode: %s\n", result.Schema.TimestampMode)
	fmt.Printf("  • Differential encoding: %v\n", result.Schema.EnableDifferential)
	fmt.Printf("  • Repeat markers: %v\n", result.Schema.EnableRepeatMarkers)

	fmt.Println("\n💡 Schema Explanation:")
	fmt.Println(tsln.GetExplanation(result.Schema))
}

func iotExample() {
	fmt.Println("🌡️ Example 3: IoT Sensor Data")
	fmt.Println("==============================")

	baseTime := time.Now().Truncate(time.Minute)

	// Generate sensor data with regular intervals
	data := make([]tsln.BufferedDataPoint, 20)
	for i := 0; i < 20; i++ {
		data[i] = tsln.BufferedDataPoint{
			Timestamp: baseTime.Add(time.Duration(i*5) * time.Second),
			Data: map[string]interface{}{
				"sensor":      "TEMP-001",
				"temperature": 22.5 + float64(i)*0.1,
				"humidity":    45.0 + float64(i)*0.5,
				"battery":     98.0 - float64(i)*0.2,
				"status":      "online",
			},
		}
	}

	result, err := tsln.ConvertToTSLN(data, nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\n📊 Dataset Analysis:\n")
	fmt.Printf("  • Total points: %d\n", result.Analysis.TotalPoints)
	fmt.Printf("  • Regular interval: %v\n", result.Analysis.IsRegularInterval)
	fmt.Printf("  • Interval: %dms\n", result.Analysis.TimestampInterval)
	fmt.Printf("  • Dataset volatility: %.3f\n", result.Analysis.DatasetVolatility)
	fmt.Printf("  • Compression potential: %.1f%%\n", result.Analysis.CompressionPotential*100)

	fmt.Println("\n🔍 Field Analysis:")
	for fieldName, analysis := range result.Analysis.FieldAnalyses {
		fmt.Printf("  • %s: type=%s, repeat_rate=%.1f%%",
			fieldName, analysis.Type, analysis.RepeatRate*100)
		if analysis.IsNumeric {
			fmt.Printf(", volatility=%.3f", analysis.Volatility)
		}
		fmt.Println()
	}

	fmt.Printf("\n💾 Compression Results:\n")
	fmt.Printf("  • Original: %d bytes → TSLN: %d bytes\n",
		result.Stats.OriginalSize, result.Stats.TSLNSize)
	fmt.Printf("  • Space saved: %d bytes (%.1f%%)\n",
		result.Stats.OriginalSize-result.Stats.TSLNSize,
		result.Stats.CompressionRatio*100)
}

func formatComparisonExample() {
	fmt.Println("⚖️ Example 4: Format Comparison")
	fmt.Println("================================")

	baseTime := time.Now().Truncate(time.Second)

	// Create sample data
	data := make([]tsln.BufferedDataPoint, 50)
	for i := 0; i < 50; i++ {
		data[i] = tsln.BufferedDataPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Data: map[string]interface{}{
				"symbol": "BTC",
				"price":  50000.0 + float64(i)*10,
				"volume": 1234567.0,
			},
		}
	}

	comparison, err := tsln.CompareFormats(data)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n📊 Format Comparison:")
	fmt.Printf("\n  JSON:\n")
	fmt.Printf("    Size:   %6d bytes\n", comparison.JSON.Size)
	fmt.Printf("    Tokens: %6d\n", comparison.JSON.Tokens)

	fmt.Printf("\n  CSV:\n")
	fmt.Printf("    Size:   %6d bytes\n", comparison.CSV.Size)
	fmt.Printf("    Tokens: %6d\n", comparison.CSV.Tokens)

	fmt.Printf("\n  TOON:\n")
	fmt.Printf("    Size:   %6d bytes\n", comparison.TOON.Size)
	fmt.Printf("    Tokens: %6d\n", comparison.TOON.Tokens)

	fmt.Printf("\n  TSLN:\n")
	fmt.Printf("    Size:   %6d bytes\n", comparison.TSLN.Size)
	fmt.Printf("    Tokens: %6d\n", comparison.TSLN.Tokens)

	fmt.Printf("\n🏆 Best Format: %s\n", comparison.BestFormat)
	fmt.Printf("💰 Savings: %d%% compared to worst format\n", comparison.Savings)

	// Show token savings
	jsonVsTsln := comparison.JSON.Tokens - comparison.TSLN.Tokens
	savingsPercent := float64(jsonVsTsln) / float64(comparison.JSON.Tokens) * 100
	fmt.Printf("\n💡 TSLN vs JSON:\n")
	fmt.Printf("   Token savings: %d tokens (%.1f%%)\n", jsonVsTsln, savingsPercent)
	fmt.Printf("   Cost impact: ~%.1fx reduction in API costs\n", 1.0/(1.0-savingsPercent/100))
}

// Helper function to split string into lines
func splitLines(s string, maxLines int) []string {
	lines := []string{}
	current := ""
	count := 0

	for _, ch := range s {
		current += string(ch)
		if ch == '\n' {
			lines = append(lines, current[:len(current)-1])
			current = ""
			count++
			if count >= maxLines {
				break
			}
		}
	}

	if current != "" && count < maxLines {
		lines = append(lines, current)
	}

	return lines
}
