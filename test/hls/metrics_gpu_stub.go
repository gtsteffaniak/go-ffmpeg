//go:build !darwin

package main

func darwinGPUBackend() gpuBackend {
	return gpuBackend{name: "none", read: nil}
}
