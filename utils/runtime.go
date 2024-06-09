package utils

import (
	"runtime/metrics"
)

// Basic runtime info retrieved from the go runtime. These could
// be retrieved more directly, but go suggests using the metrics
// package, so... probably better I guess?
type RuntimeInfo struct {
	GoMemLimit      uint64
	TotalAllocBytes uint64
	LiveHeapBytes   uint64
	HeapGoal        uint64
	GoroutineCount  uint64
}

// Pull a uint64 from metrics OR default to 0 (no error)
func Uint64SafeMetric(sample *metrics.Sample) uint64 {
	if sample.Value.Kind() == metrics.KindBad {
		return 0
	}
	return sample.Value.Uint64()
}

// Retrieve runtime information. I don't know how performant this is
func GetRuntimeInfo() RuntimeInfo {
	// The list of these keys is at https://pkg.go.dev/runtime/metrics
	keys := []string{
		"/gc/gomemlimit:bytes",
		"/gc/heap/allocs:bytes",
		"/gc/heap/live:bytes",
		"/sched/goroutines:goroutines",
		"/gc/heap/goal:bytes",
	}
	sample := make([]metrics.Sample, len(keys))
	for i := range keys {
		sample[i].Name = keys[i]
	}
	metrics.Read(sample)

	return RuntimeInfo{
		GoMemLimit:      Uint64SafeMetric(&sample[0]),
		TotalAllocBytes: Uint64SafeMetric(&sample[1]),
		LiveHeapBytes:   Uint64SafeMetric(&sample[2]),
		GoroutineCount:  Uint64SafeMetric(&sample[3]),
		HeapGoal:        Uint64SafeMetric(&sample[4]),
	}
}
