//go:build windows

package reader

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

type memoryStatusEx struct {
	cbSize                  uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var globalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")

type DefaultMemoryReader struct{}

func NewMemoryReader() *DefaultMemoryReader {
	return &DefaultMemoryReader{}
}

func (r *DefaultMemoryReader) GetStats() (MemoryStats, error) {
	var memStat memoryStatusEx
	memStat.cbSize = uint32(unsafe.Sizeof(memStat))

	ret, _, _ := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&memStat)))
	if ret == 0 {
		return MemoryStats{}, fmt.Errorf("failed to get memory status")
	}

	total := memStat.ullTotalPhys
	available := memStat.ullAvailPhys
	used := total - available

	const bytesToGB = 1024 * 1024 * 1024

	return MemoryStats{
		TotalGB:     float64(total) / float64(bytesToGB),
		UsedGB:      float64(used) / float64(bytesToGB),
		UsedPercent: (float64(used) / float64(total)) * 100,
		Timestamp:   time.Now(),
	}, nil
}
