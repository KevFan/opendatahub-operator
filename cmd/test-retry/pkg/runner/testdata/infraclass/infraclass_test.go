package infraclass_test

import (
	"fmt"
	"testing"
)

// TestInfraFail fails and emits an infrastructure classification, mimicking
// the e2e framework's failure diagnostics output.
func TestInfraFail(t *testing.T) {
	fmt.Println(`FAILURE_CLASSIFICATION: {"category":"infrastructure","subcategory":"pod-startup","error_code":1002,"evidence":["pod stuck pending"],"confidence":"high"}`)
	t.Fatal("simulated infra failure")
}
