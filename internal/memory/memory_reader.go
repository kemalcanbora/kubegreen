package reader

import (
	"time"
)

// MemoryStats represents memory usage statistics
type MemoryStats struct {
	TotalGB       float64
	UsedGB        float64
	AppMemoryGB   float64
	WiredGB       float64
	CompressedGB  float64
	CachedFilesGB float64
	UsedPercent   float64
	Timestamp     time.Time
}

// MemoryReader provides an interface for reading memory statistics
type MemoryReader interface {
	GetStats() (MemoryStats, error)
}
