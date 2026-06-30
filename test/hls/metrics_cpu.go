package main

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// readProcessTreeCPUPercent returns the summed OS-reported CPU utilization for root
// and all descendant processes. Values may exceed 100% when multiple cores are busy
// (e.g. 300% ≈ three cores at full utilization).
func readProcessTreeCPUPercent(root int) float64 {
	if root <= 0 {
		root = os.Getpid()
	}
	if runtime.GOOS == "windows" {
		return readProcessTreeCPUPercentWindows(root)
	}
	return readProcessTreeCPUPercentUnix(root)
}

func processTreePIDs(root int) []int {
	if root <= 0 {
		return nil
	}
	seen := map[int]bool{root: true}
	queue := []int{root}
	out := []int{root}
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		for _, child := range childPIDs(pid) {
			if child <= 0 || seen[child] {
				continue
			}
			seen[child] = true
			out = append(out, child)
			queue = append(queue, child)
		}
	}
	return out
}

func childPIDs(ppid int) []int {
	if runtime.GOOS == "windows" {
		return childPIDsWindows(ppid)
	}
	return childPIDsUnix(ppid)
}

func childPIDsUnix(ppid int) []int {
	out, err := exec.Command("pgrep", "-P", strconv.Itoa(ppid)).Output()
	if err != nil {
		return nil
	}
	return parsePIDLines(out)
}

func childPIDsWindows(ppid int) []int {
	script := `Get-CimInstance Win32_Process -Filter "ParentProcessId=` + strconv.Itoa(ppid) + `" | Select-Object -ExpandProperty ProcessId`
	out, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
	if err != nil {
		return nil
	}
	return parsePIDLines(out)
}

func parsePIDLines(out []byte) []int {
	var pids []int
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err != nil || pid <= 0 {
			continue
		}
		pids = append(pids, pid)
	}
	return pids
}

func readProcessTreeCPUPercentUnix(root int) float64 {
	return sumPIDsCPUPercentUnix(processTreePIDs(root))
}

func sumPIDsCPUPercentUnix(pids []int) float64 {
	if len(pids) == 0 {
		return 0
	}
	args := []string{"-o", "pcpu="}
	args = append(args, "-p")
	for _, pid := range pids {
		args = append(args, strconv.Itoa(pid))
	}
	out, err := exec.Command("ps", args...).Output()
	if err != nil {
		return 0
	}
	var sum float64
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		v, err := strconv.ParseFloat(line, 64)
		if err != nil || v < 0 {
			continue
		}
		sum += v
	}
	return sum
}

func readProcessTreeCPUPercentWindows(root int) float64 {
	pids := processTreePIDs(root)
	if len(pids) == 0 {
		return 0
	}
	var idParts []string
	for _, pid := range pids {
		idParts = append(idParts, strconv.Itoa(pid))
	}
	script := `$ids = @(` + strings.Join(idParts, ",") + `); $sum = 0; Get-CimInstance Win32_PerfFormattedData_PerfProc_Process | Where-Object { $ids -contains $_.IDProcess } | ForEach-Object { $sum += $_.PercentProcessorTime }; Write-Output $sum`
	out, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
	if err != nil {
		return 0
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || v < 0 {
		return 0
	}
	return v
}

func integrateCPUTimeSec(samples []resourceSample, sampleIntervalSec float64) float64 {
	if len(samples) == 0 || sampleIntervalSec <= 0 {
		return 0
	}
	var total float64
	for _, s := range samples {
		total += (s.CPUPercent / 100.0) * sampleIntervalSec
	}
	return total
}

const resourceSampleIntervalSec = 0.2
