package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	qsgo "github.com/agentic-build-lab/qs-go"
)

const (
	sampleCount         = 40
	iterationsPerSample = 500
	warmupIterations    = 2000
)

var benchmarkSink int

type workloadResult struct {
	Name       string  `json:"name"`
	MedianNS   float64 `json:"median_ns_per_op"`
	P95NS      float64 `json:"p95_ns_per_op"`
	P99NS      float64 `json:"p99_ns_per_op"`
	MinimumNS  float64 `json:"minimum_ns_per_op"`
	MaximumNS  float64 `json:"maximum_ns_per_op"`
	Operations int     `json:"operations"`
	Checksum   int     `json:"checksum"`
}

type memoryResult struct {
	HeapAllocBefore uint64 `json:"heap_alloc_before_bytes"`
	HeapAllocAfter  uint64 `json:"heap_alloc_after_bytes"`
	TotalAllocDelta uint64 `json:"total_alloc_delta_bytes"`
}

type report struct {
	Schema              string           `json:"schema"`
	Runtime             string           `json:"runtime"`
	OS                  string           `json:"os"`
	Architecture        string           `json:"architecture"`
	LogicalCPUs         int              `json:"logical_cpus"`
	Samples             int              `json:"samples"`
	IterationsPerSample int              `json:"iterations_per_sample"`
	Memory              memoryResult     `json:"memory"`
	Workloads           []workloadResult `json:"workloads"`
}

func main() {
	flatQuery, flatValue := flatWorkload(100)
	nestedQuery, nestedValue := nestedWorkload(20)

	var before runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	workloads := []workloadResult{
		measure("parse_flat_100", func() int {
			value, err := qsgo.Parse(flatQuery, nil)
			if err != nil {
				panic(err)
			}
			members, _ := value.Members()
			return len(members)
		}),
		measure("parse_nested_20", func() int {
			value, err := qsgo.Parse(nestedQuery, nil)
			if err != nil {
				panic(err)
			}
			members, _ := value.Members()
			return len(members)
		}),
		measure("stringify_flat_100", func() int {
			value, err := qsgo.Stringify(flatValue, nil)
			if err != nil {
				panic(err)
			}
			return len(value)
		}),
		measure("stringify_nested_20", func() int {
			value, err := qsgo.Stringify(nestedValue, nil)
			if err != nil {
				panic(err)
			}
			return len(value)
		}),
	}

	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	result := report{
		Schema:              "qs_go_benchmark/v1",
		Runtime:             runtime.Version(),
		OS:                  runtime.GOOS,
		Architecture:        runtime.GOARCH,
		LogicalCPUs:         runtime.NumCPU(),
		Samples:             sampleCount,
		IterationsPerSample: iterationsPerSample,
		Memory: memoryResult{
			HeapAllocBefore: before.HeapAlloc,
			HeapAllocAfter:  after.HeapAlloc,
			TotalAllocDelta: after.TotalAlloc - before.TotalAlloc,
		},
		Workloads: workloads,
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(encoded))
	if benchmarkSink == 0 {
		os.Exit(2)
	}
}

func measure(name string, operation func() int) workloadResult {
	for index := 0; index < warmupIterations; index++ {
		benchmarkSink += operation()
	}
	samples := make([]float64, sampleCount)
	checksum := 0
	for sample := range samples {
		started := time.Now()
		for iteration := 0; iteration < iterationsPerSample; iteration++ {
			checksum += operation()
		}
		elapsed := time.Since(started)
		samples[sample] = float64(elapsed.Nanoseconds()) / float64(iterationsPerSample)
	}
	benchmarkSink += checksum
	sorted := append([]float64(nil), samples...)
	sort.Float64s(sorted)
	return workloadResult{
		Name:       name,
		MedianNS:   percentile(sorted, 0.50),
		P95NS:      percentile(sorted, 0.95),
		P99NS:      percentile(sorted, 0.99),
		MinimumNS:  sorted[0],
		MaximumNS:  sorted[len(sorted)-1],
		Operations: sampleCount * iterationsPerSample,
		Checksum:   checksum,
	}
}

func percentile(sorted []float64, quantile float64) float64 {
	index := int(float64(len(sorted)-1)*quantile + 0.5)
	return sorted[index]
}

func flatWorkload(size int) (string, qsgo.Value) {
	parts := make([]string, size)
	members := make([]qsgo.Member, size)
	for index := 0; index < size; index++ {
		key := "k" + strconv.Itoa(index)
		value := "value" + strconv.Itoa(index)
		parts[index] = key + "=" + value
		members[index] = qsgo.Member{Key: key, Value: qsgo.NewString(value)}
	}
	return strings.Join(parts, "&"), qsgo.NewObject(members...)
}

func nestedWorkload(size int) (string, qsgo.Value) {
	parts := make([]string, size)
	nestedMembers := make([]qsgo.Member, size)
	for index := 0; index < size; index++ {
		key := "k" + strconv.Itoa(index)
		value := "value" + strconv.Itoa(index)
		parts[index] = "root[group][" + key + "]=" + value
		nestedMembers[index] = qsgo.Member{Key: key, Value: qsgo.NewString(value)}
	}
	value := qsgo.NewObject(qsgo.Member{Key: "root", Value: qsgo.NewObject(
		qsgo.Member{Key: "group", Value: qsgo.NewObject(nestedMembers...)},
	)})
	return strings.Join(parts, "&"), value
}
