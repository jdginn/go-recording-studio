package main

import (
	"bufio"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
)

// Sample represents a processed impulse response sample
type Sample struct {
	TimeMs float64
	Linear float64
	Db     float64
}

// LinearToDb converts a linear amplitude value to decibels
func LinearToDb(volume float64, minDb float64) float64 {
	if volume == 0.0 {
		return minDb
	}
	return 20.0 * math.Log10(math.Abs(volume))
}

// ParseImpulseResponse reads the file and returns samples (time_ms, linear value)
func ParseImpulseResponse(filename string) (peakIndex int, sampleInterval float64, samples []float64, err error) {
	file, err := os.Open(filename)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	peakIndex = -1
	sampleInterval = 0.0
	var rawSamples []float64
	foundDataStart := false

	// First pass: parse metadata
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, "// Peak index") {
			parts := strings.Fields(line)
			peakIndex, _ = strconv.Atoi(parts[0])
		} else if strings.Contains(line, "// Sample interval (seconds)") {
			parts := strings.Fields(line)
			sampleInterval, _ = strconv.ParseFloat(parts[0], 64)
		} else if line == "* Data start" {
			foundDataStart = true
			break
		}
	}
	if !foundDataStart {
		err = fmt.Errorf("* Data start not found")
		return
	}

	// Second pass: read samples
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		val, parseErr := strconv.ParseFloat(line, 64)
		if parseErr != nil {
			continue
		}
		rawSamples = append(rawSamples, val)
	}

	if peakIndex < 0 || sampleInterval == 0.0 {
		err = fmt.Errorf("metadata missing or malformed")
		return
	}

	samples = rawSamples
	return
}

// MakeProcessedSamples returns a list of Sample starting at peakIndex
func MakeProcessedSamples(peakIndex int, sampleInterval float64, samples []float64) []Sample {
	var result []Sample
	for i, v := range samples[peakIndex:] {
		t := float64(i) * sampleInterval * 1000.0
		result = append(result, Sample{
			TimeMs: t,
			Linear: v,
			Db:     LinearToDb(v, -120.0),
		})
	}
	return result
}

// FindLocalMaxima finds local maxima as defined in the prompt
func FindLocalMaxima(samples []Sample, thresholdDb float64, baseline float64) []Sample {
	var maxima []Sample
	inSpan := false
	var currMax Sample

	for _, s := range samples {
		above := s.Db >= baseline+thresholdDb
		if above {
			if !inSpan {
				// Starting a new span
				inSpan = true
				currMax = s
			} else {
				if s.Db > currMax.Db {
					currMax = s
				}
			}
		} else {
			if inSpan {
				maxima = append(maxima, currMax)
				inSpan = false
			}
		}
	}
	if inSpan {
		maxima = append(maxima, currMax)
	}
	return maxima
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run ir_parser.go <impulse_response_file> <output_file>")
		os.Exit(1)
	}
	inputPath := os.Args[1]
	outputPath := os.Args[2]

	peakIndex, sampleInterval, samples, err := ParseImpulseResponse(inputPath)
	if err != nil {
		log.Fatalf("Parse error: %v\n", err)
	}

	processed := MakeProcessedSamples(peakIndex, sampleInterval, samples)

	// You can change the baseline value if needed
	baseline := -30.0
	maxima := FindLocalMaxima(processed, 6.0, baseline)

	out, err := os.Create(outputPath)
	if err != nil {
		log.Fatalf("Could not create output file: %v\n", err)
	}
	defer out.Close()
	w := bufio.NewWriter(out)
	fmt.Fprintf(w, "Impulse Response Peaks\n")
	for _, s := range maxima {
		fmt.Fprintf(w, "%.6fms, %.2fdB\n", s.TimeMs, s.Db)
	}
	w.Flush()
	fmt.Printf("Wrote %d local maxima to %s\n", len(maxima), outputPath)
}
