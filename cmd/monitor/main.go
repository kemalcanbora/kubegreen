package main

import (
	"fmt"
	reader "kubegreen/internal/memory"
	"log"
	"time"
)

func main() {
	memReader := reader.NewMemoryReader()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	fmt.Println("Starting memory monitoring...")
	fmt.Println("Press Ctrl+C to stop")

	// Print header
	fmt.Printf("\n%-20s %-10s %-10s %-10s %-10s %-10s %-10s %s\n",
		"Timestamp",
		"Total",
		"Used",
		"App",
		"Wired",
		"Compressed",
		"Cached",
		"Used%")

	for range ticker.C {
		stats, err := memReader.GetStats()
		if err != nil {
			log.Printf("Error reading memory stats: %v", err)
			continue
		}

		appMemory := "-"
		wiredMemory := "-"
		compressedMemory := "-"

		if stats.AppMemoryGB > 0 {
			appMemory = fmt.Sprintf("%.2f", stats.AppMemoryGB)
		}
		if stats.WiredGB > 0 {
			wiredMemory = fmt.Sprintf("%.2f", stats.WiredGB)
		}
		if stats.CompressedGB > 0 {
			compressedMemory = fmt.Sprintf("%.2f", stats.CompressedGB)
		}

		fmt.Printf("%-20s %-10.2f %-10.2f %-10s %-10s %-10s %-10.2f %.1f%%\n",
			stats.Timestamp.Format("15:04:05"),
			stats.TotalGB,
			stats.UsedGB,
			appMemory,
			wiredMemory,
			compressedMemory,
			stats.CachedFilesGB,
			stats.UsedPercent)
	}
}
