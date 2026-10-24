package capabilities

import (
	"context"
	"testing"
)

func TestFunctionalHWProbeTimeoutClassification(t *testing.T) {
	code, _ := ClassifyHWProbeFailure("probe timed out after 10s", context.DeadlineExceeded, "vaapi")
	if code != HWProbeTimeout {
		t.Fatalf("code=%s", code)
	}
}
