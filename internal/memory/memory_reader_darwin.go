//go:build darwin

package reader

// #include <unistd.h>
// #include <sys/types.h>
// #include <sys/sysctl.h>
// #include <mach/mach.h>
import "C"
import (
	"fmt"
	"time"
	"unsafe"
)

type DefaultMemoryReader struct{}

func NewMemoryReader() *DefaultMemoryReader {
	return &DefaultMemoryReader{}
}

func (r *DefaultMemoryReader) GetStats() (MemoryStats, error) {
	host := C.mach_host_self()
	var stats C.vm_statistics64_data_t
	var count C.mach_msg_type_number_t = C.HOST_VM_INFO64_COUNT

	ret := C.host_statistics64(
		C.host_t(host),
		C.HOST_VM_INFO64,
		C.host_info_t(unsafe.Pointer(&stats)),
		&count,
	)

	if ret != C.KERN_SUCCESS {
		return MemoryStats{}, fmt.Errorf("failed to get VM stats")
	}

	// Total physical memory
	var totalMem C.uint64_t
	var size C.size_t = C.size_t(unsafe.Sizeof(totalMem))
	name := [2]C.int{C.CTL_HW, C.HW_MEMSIZE}
	_, err := C.sysctl(&name[0], 2, unsafe.Pointer(&totalMem), &size, nil, 0)
	if err != nil {
		return MemoryStats{}, fmt.Errorf("failed to get total memory: %v", err)
	}

	pageSize := uint64(C.sysconf(C._SC_PAGESIZE))
	appMemory := uint64(stats.active_count) * pageSize
	wiredMemory := uint64(stats.wire_count) * pageSize
	compressed := uint64(stats.compressor_page_count) * pageSize
	cached := uint64(stats.external_page_count) * pageSize
	used := appMemory + wiredMemory + compressed
	total := uint64(totalMem)

	const bytesToGB = 1024 * 1024 * 1024

	return MemoryStats{
		TotalGB:       float64(total) / float64(bytesToGB),
		UsedGB:        float64(used) / float64(bytesToGB),
		AppMemoryGB:   float64(appMemory) / float64(bytesToGB),
		WiredGB:       float64(wiredMemory) / float64(bytesToGB),
		CompressedGB:  float64(compressed) / float64(bytesToGB),
		CachedFilesGB: float64(cached) / float64(bytesToGB),
		UsedPercent:   (float64(used) / float64(total)) * 100,
		Timestamp:     time.Now(),
	}, nil
}
