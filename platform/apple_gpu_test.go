package platform

import "testing"

func TestParseDarwinGPUReportSingle(t *testing.T) {
	out := []byte(`Graphics/Displays:

    Apple M1:

      Chipset Model: Apple M1
      Type: GPU
      Bus: Built-In
      Vendor: Apple (0x106b)
`)
	gpus := parseDarwinGPUReport(out)
	if len(gpus) != 1 {
		t.Fatalf("got %d GPUs, want 1", len(gpus))
	}
	g := gpus[0]
	if g.Name != "Apple M1" || g.Vendor != "apple" || !g.Integrated {
		t.Fatalf("unexpected GPU: %+v", g)
	}
	if g.ID != "name:apple m1" {
		t.Fatalf("ID = %q, want name:apple m1", g.ID)
	}
}

func TestParseDarwinGPUReportDuplicateModelsGetDistinctIDs(t *testing.T) {
	out := []byte(`Graphics/Displays:

    Radeon Pro W5700X:

      Chipset Model: Radeon Pro W5700X
      Type: GPU
      Bus: PCIe
      Vendor: AMD (0x1002)

    Radeon Pro W5700X:

      Chipset Model: Radeon Pro W5700X
      Type: GPU
      Bus: PCIe
      Vendor: AMD (0x1002)
`)
	gpus := parseDarwinGPUReport(out)
	if len(gpus) != 2 {
		t.Fatalf("got %d GPUs, want 2", len(gpus))
	}
	if gpus[0].ID == gpus[1].ID {
		t.Fatalf("duplicate IDs %q; mergeGPULists would drop the second GPU", gpus[0].ID)
	}
	for _, g := range gpus {
		if g.Vendor != "amd" || g.Integrated {
			t.Fatalf("unexpected GPU: %+v", g)
		}
	}
	merged := mergeGPULists(nil, gpus)
	if len(merged) != 2 {
		t.Fatalf("mergeGPULists kept %d GPUs, want 2", len(merged))
	}
}

func TestParseDarwinGPUReportEmptyFallsBackToSynthetic(t *testing.T) {
	for _, out := range [][]byte{nil, []byte("not a report")} {
		gpus := parseDarwinGPUReport(out)
		if len(gpus) != 1 {
			t.Fatalf("got %d GPUs, want 1 synthetic fallback", len(gpus))
		}
		if gpus[0].Vendor != "apple" || gpus[0].Name != "Apple GPU" || !gpus[0].Integrated {
			t.Fatalf("unexpected fallback GPU: %+v", gpus[0])
		}
	}
}
