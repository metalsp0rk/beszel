//go:build linux

// Package zfs provides functions to read ZFS statistics.
package zfs

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/henrygd/beszel/agent/utils"
	"github.com/henrygd/beszel/internal/entities/system"
)

func ARCSize() (uint64, error) {
	file, err := os.Open("/proc/spl/kstat/zfs/arcstats")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "size") {
			fields := strings.Fields(line)
			if len(fields) < 3 {
				return 0, fmt.Errorf("unexpected arcstats size format: %s", line)
			}
			return strconv.ParseUint(fields[2], 10, 64)
		}
	}

	return 0, fmt.Errorf("size field not found in arcstats")
}

func GetZFSStats() (*system.ZFSStats, error) {
	stats := &system.ZFSStats{
		Datasets: make(map[string]*system.ZFSDataset),
	}

	arcStats, err := getARCStats()
	if err != nil {
		slog.Debug("Error getting ARC stats", "err", err)
	} else {
		slog.Debug("Got ARC stats", "size", arcStats)
		stats.Arc = arcStats
	}
	datasets, err := getDatasets()
	if err != nil {
		slog.Debug("Error getting datasets", "err", err)
	} else {
		for _, ds := range datasets {
			stats.Datasets[ds.Name] = ds
		}
	}

	pools, err := ParseZPoolStatus()
	if err != nil {
		slog.Debug("Error getting pools", "err", err)
	} else {
		stats.Pools = pools
	}

	return stats, nil
}

// getARCStats reads ARC statistics from /proc/spl/kstat/zfs/arcstats
func getARCStats() (*system.ZFSArcStats, error) {
	file, err := os.Open("/proc/spl/kstat/zfs/arcstats")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stats := &system.ZFSArcStats{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		name := fields[0]
		value, err := strconv.ParseUint(fields[2], 10, 64)
		if err != nil {
			continue
		}
		switch name {
		case "size":
			stats.Size = value
		case "c_max":
			stats.Max = value
		case "c_min":
			stats.Min = value
		case "hits":
			stats.Hits = value
		case "misses":
			stats.Misses = value
		case "compressed_size":
			stats.Compressed = value
		case "uncompressed_size":
			stats.Uncompressed = value
		case "l2_size":
			stats.L2Size = value
		case "l2_asize":
			stats.L2ASize = value
		case "l2_hits":
			stats.L2Hits = value
		case "l2_misses":
			stats.L2Misses = value
		case "l2_read_bytes":
			stats.L2ReadBytes = value
		case "l2_write_bytes":
			stats.L2WriteBytes = value
		case "l2_hdr_size":
			stats.L2HdrSize = value
		}
	}
	return stats, nil
}

type ZPoolStatusOutput struct {
	OutputVersion struct {
		Command   string `json:"command"`
		VersMajor int    `json:"vers_major"`
		VersMinor int    `json:"vers_minor"`
	} `json:"output_version"`
	Pools map[string]ZPoolRaw `json:"pools"`
}

type ZPoolRaw struct {
	Name       string             `json:"name"`
	State      string             `json:"state"`
	PoolGUID   string             `json:"pool_guid"`
	Txg        string             `json:"txg"`
	Status     string             `json:"status"`
	Action     string             `json:"action"`
	MsgID      string             `json:"msgid"`
	ErrorCount string             `json:"error_count"`
	ScanStats  ScanStatsRaw       `json:"scan_stats"`
	VDevs      map[string]VDevRaw `json:"vdevs"`
}

type VDevRaw struct {
	AllocSpace     string             `json:"alloc_space"`
	TotalSpace     string             `json:"total_space"`
	DefSpace       string             `json:"def_space"` // sometimes used for "deflated" size
	ReadErrors     string             `json:"read_errors"`
	WriteErrors    string             `json:"write_errors"`
	ChecksumErrors string             `json:"checksum_errors"`
	VDevs          map[string]VDevRaw `json:"vdevs"`
}

type ScanStatsRaw struct {
	Function  string `json:"function"`
	State     string `json:"state"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Size      uint64 `json:"to_examine"`
	Done      uint64 `json:"examined"`
	Skipped   uint64 `json:"skipped"`
	Errors    string `json:"errors"`
}

// parseSize helper (handles "30.5T", "60.2G", etc.)
func parseSize(s string) uint64 {
	if s == "" {
		return 0
	}
	s = strings.ReplaceAll(s, ",", "")
	var multiplier uint64 = 1
	if idx := strings.LastIndexAny(s, "TGMK"); idx != -1 {
		unit := s[idx:]
		s = s[:idx]
		switch unit {
		case "T":
			multiplier = 1 << 40
		case "G":
			multiplier = 1 << 30
		case "M":
			multiplier = 1 << 20
		case "K":
			multiplier = 1 << 10
		}
	}
	val, _ := strconv.ParseFloat(s, 64)
	return uint64(val * float64(multiplier))
}

func ParseZPoolStatus() (map[string]*system.ZFSPool, error) {
	cmd := exec.Command("zpool", "status", "-j")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var out ZPoolStatusOutput
	if err := json.Unmarshal(output, &out); err != nil {
		return nil, err
	}

	result := make(map[string]*system.ZFSPool)
	for poolName, raw := range out.Pools {
		pool := &system.ZFSPool{
			Name:  poolName,
			State: raw.State,
		}

		if root, ok := raw.VDevs[poolName]; ok {
			pool.Size = parseSize(root.TotalSpace)
			pool.Used = parseSize(root.AllocSpace)
		}

		pool.CksumErrors, _ = strconv.ParseUint(raw.ErrorCount, 10, 64)

		pool.Scan.Type = raw.ScanStats.Function
		pool.Scan.State = raw.ScanStats.State
		pool.Scan.Start, _ = utils.ParseDateString(raw.ScanStats.StartTime)
		pool.Scan.End, _ = utils.ParseDateString(raw.ScanStats.EndTime)
		pool.Scan.Size = raw.ScanStats.Size
		pool.Scan.Completed = raw.ScanStats.Done
		pool.Scan.Skipped = raw.ScanStats.Skipped

		// IO bytes/ops are not in this JSON (use /proc/spl/kstat for those)
		// pool.ReadBytes, etc. remain 0

		path := fmt.Sprintf("/proc/spl/kstat/zfs/%s/iostats", poolName)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		m := make(map[string]uint64)
		for line := range strings.SplitSeq(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if val, err := strconv.ParseUint(fields[len(fields)-1], 10, 64); err == nil {
					m[fields[0]] = val
				}
			}
		}
		pool.ReadBytes = m["arc_read_bytes"]
		pool.WriteBytes = m["arc_write_bytes"]
		pool.ReadOps = m["arc_read_count"]
		pool.WriteOps = m["arc_write_count"]

		result[poolName] = pool

	}

	return result, nil
}

// getDatasets returns list of ZFS datasets
func getDatasets() ([]*system.ZFSDataset, error) {
	cmd := exec.Command("zfs", "list", "-H", "-p", "-o", "name,used,available,referenced,type,compressratio")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var datasets []*system.ZFSDataset
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	first := true
	for scanner.Scan() {
		if first {
			first = false
			continue
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}

		ds := &system.ZFSDataset{
			Pool: strings.Split(string(fields[0]), "/")[0],
			Name: fields[0],
		}
		if len(fields) > 1 {
			ds.Used, _ = strconv.ParseUint(fields[1], 10, 64)
		}
		if len(fields) > 2 {
			ds.Available, _ = strconv.ParseUint(fields[2], 10, 64)
		}
		if len(fields) > 3 {
			ds.Refer, _ = strconv.ParseUint(fields[3], 10, 64)
		}
		if len(fields) > 4 {
			ds.Type = fields[4]
		}
		if len(fields) > 5 {
			ds.CompressRatio, _ = strconv.ParseFloat(fields[5], 64)
		}
		datasets = append(datasets, ds)
	}
	// slog.Debug(datasets)
	// for _, ds := range datasets {
	// 	slog.Debug("Datasets: ", "msg", ds.String())
	// }
	return datasets, nil
}
