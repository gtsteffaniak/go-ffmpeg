//go:build darwin

package main

import (
	"os/exec"
	"regexp"
	"strconv"
)

var appleDeviceUtilRE = regexp.MustCompile(`"Device Utilization %"=(\d+)`)

func darwinGPUBackend() gpuBackend {
	return gpuBackend{name: "ioreg", read: readIORegGPUUtil}
}

// readIORegGPUUtil reads Apple GPU "Device Utilization %" from IOKit (no root required).
// VideoToolbox encode often uses the dedicated media engine; this reflects AGX GPU activity
// and may under-report pure media-engine workloads.
func readIORegGPUUtil() *float64 {
	for _, class := range []string{"AGXAccelerator", "IOAccelerator"} {
		out, err := exec.Command("ioreg", "-l", "-w", "0", "-r", "-c", class).Output()
		if err != nil {
			continue
		}
		if v := maxAppleDeviceUtilPercent(out); v != nil {
			return v
		}
	}
	return nil
}

func maxAppleDeviceUtilPercent(data []byte) *float64 {
	matches := appleDeviceUtilRE.FindAllSubmatch(data, -1)
	if len(matches) == 0 {
		return nil
	}
	var max float64
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		v, err := strconv.ParseFloat(string(m[1]), 64)
		if err != nil || v < 0 {
			continue
		}
		if v > max {
			max = v
		}
	}
	return &max
}
