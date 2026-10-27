package infraclass_partial_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// markerPath makes the suite behave differently per attempt: the first run
// completes cleanly; on the retry, TestCHang trips the go test -timeout before
// TestARealFail gets to run, so the retry recovers only partial results.
var markerPath = filepath.Join(os.TempDir(), "test-retry-infraclass-partial-attempt2")

// TestBInfra fails fast with an infrastructure classification.
func TestBInfra(t *testing.T) {
	fmt.Println(`FAILURE_CLASSIFICATION: {"category":"infrastructure","subcategory":"pod-startup","error_code":1002,"evidence":["pod stuck pending"],"confidence":"high"}`)
	t.Fatal("simulated infra failure")
}

// TestCHang fails fast on the first attempt and hangs on the retry.
func TestCHang(t *testing.T) {
	if _, err := os.Stat(markerPath); err == nil {
		fmt.Println(`FAILURE_CLASSIFICATION: {"category":"infrastructure","subcategory":"cluster-distress","error_code":1010,"evidence":["API endpoint unreachable"],"confidence":"high"}`)
		time.Sleep(10 * time.Second) // trips a short go test -timeout on the retry
		return
	}
	_ = os.WriteFile(markerPath, []byte("1"), 0o600)
	t.Fatal("fails fast on first attempt")
}

// TestARealFail is a genuine test failure. It runs after TestCHang, so the
// timed-out retry never reaches it.
func TestARealFail(t *testing.T) {
	t.Fatal("real test bug")
}
