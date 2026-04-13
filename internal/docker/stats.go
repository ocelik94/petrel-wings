package docker

import (
	"context"
	"encoding/json"
	"fmt"
)

// ResourceUsage is a normalized resource usage snapshot for a container.
type ResourceUsage struct {
	CPUPercent  float64 `json:"cpu_percent"`
	MemoryBytes uint64  `json:"memory_bytes"`
	MemoryLimit uint64  `json:"memory_limit_bytes"`
	NetworkRx   uint64  `json:"network_rx_bytes"`
	NetworkTx   uint64  `json:"network_tx_bytes"`
	BlockRead   uint64  `json:"block_read_bytes"`
	BlockWrite  uint64  `json:"block_write_bytes"`
}

// GetStats polls Docker stats and maps them into ResourceUsage.
func (c *Client) GetStats(ctx context.Context, containerID string) (ResourceUsage, error) {
	resp, err := c.cli.ContainerStats(ctx, containerID, false)
	if err != nil {
		return ResourceUsage{}, fmt.Errorf("getting container stats: %w", err)
	}
	defer resp.Body.Close()

	var stats struct {
		CPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemUsage uint64 `json:"system_cpu_usage"`
			OnlineCPUs  uint64 `json:"online_cpus"`
		} `json:"cpu_stats"`
		PreCPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemUsage uint64 `json:"system_cpu_usage"`
		} `json:"precpu_stats"`
		MemoryStats struct {
			Usage uint64 `json:"usage"`
			Limit uint64 `json:"limit"`
		} `json:"memory_stats"`
		Networks map[string]struct {
			RxBytes uint64 `json:"rx_bytes"`
			TxBytes uint64 `json:"tx_bytes"`
		} `json:"networks"`
		BlkioStats struct {
			IoServiceBytesRecursive []struct {
				Op    string `json:"op"`
				Value uint64 `json:"value"`
			} `json:"io_service_bytes_recursive"`
		} `json:"blkio_stats"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return ResourceUsage{}, fmt.Errorf("decoding container stats: %w", err)
	}

	usage := ResourceUsage{
		MemoryBytes: stats.MemoryStats.Usage,
		MemoryLimit: stats.MemoryStats.Limit,
	}

	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	if sysDelta > 0 && cpuDelta > 0 {
		cpus := float64(stats.CPUStats.OnlineCPUs)
		if cpus == 0 {
			cpus = 1
		}
		usage.CPUPercent = (cpuDelta / sysDelta) * cpus * 100
	}

	for _, netStat := range stats.Networks {
		usage.NetworkRx += netStat.RxBytes
		usage.NetworkTx += netStat.TxBytes
	}
	for _, ioStat := range stats.BlkioStats.IoServiceBytesRecursive {
		switch ioStat.Op {
		case "Read":
			usage.BlockRead += ioStat.Value
		case "Write":
			usage.BlockWrite += ioStat.Value
		}
	}

	return usage, nil
}
