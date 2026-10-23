package platform

import "testing"

func TestParseWindowsAdapterList(t *testing.T) {
	out := "PCI\\VEN_10DE&DEV_25A2&SUBSYS_15B41043\\4&1f4d0f4f&0&0009|NVIDIA GeForce RTX 3050 Laptop GPU\r\n" +
		"PCI\\VEN_1002&DEV_164E&SUBSYS_12FD1043\\4&2a3b0c0d&0&0041|AMD Radeon(TM) Graphics\r\n"
	gpus := parseWindowsAdapterList(out)
	if len(gpus) != 2 {
		t.Fatalf("gpus=%d want 2", len(gpus))
	}
	if gpus[0].Vendor != "nvidia" || gpus[1].Vendor != "amd" {
		t.Fatalf("vendors=%q,%q", gpus[0].Vendor, gpus[1].Vendor)
	}
	if gpus[0].Integrated {
		t.Fatalf("RTX 3050 must classify as discrete")
	}
	if !gpus[1].Integrated {
		t.Fatalf("AMD Radeon Graphics must classify as integrated")
	}
	// WSL adapters carry no Windows backend pinning — that is applied by
	// queryWindowsGPUs only.
	if gpus[0].Pinning.D3D11 != "" {
		t.Fatalf("parser must not set Windows pinning")
	}
}

func TestParseWindowsAdapterListSkipsBlank(t *testing.T) {
	gpus := parseWindowsAdapterList("\r\n  \nno-separator\n| \n")
	if len(gpus) != 0 {
		t.Fatalf("gpus=%v want empty", gpus)
	}
}
