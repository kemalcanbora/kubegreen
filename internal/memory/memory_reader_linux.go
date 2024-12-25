//go:build linux

package reader

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"
)

type DefaultMemoryReader struct{}

func NewMemoryReader() *DefaultMemoryReader {
	return &DefaultMemoryReader{}
}

func (r *DefaultMemoryReader) GetStats() (MemoryStats, error) {
	data, err := ioutil.ReadFile("/proc/meminfo")
	if err != nil {
		return MemoryStats{}, fmt.Errorf("failed to read /proc/meminfo: %v", err)
	}

	memInfo := make(map[string]uint64)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		value, _ := strconv.ParseUint(fields[1], 10, 64)
		memInfo[key] = value * 1024 // Convert to bytes
	}

	total := memInfo["MemTotal"]
	available := memInfo["MemAvailable"]
	used := total - available

	const bytesToGB = 1024 * 1024 * 1024

	return MemoryStats{
		TotalGB:       float64(total) / float64(bytesToGB),
		UsedGB:        float64(used) / float64(bytesToGB),
		CachedFilesGB: float64(memInfo["Cached"]) / float64(bytesToGB),
		UsedPercent:   (float64(used) / float64(total)) * 100,
		Timestamp:     time.Now(),
	}, nil
}
