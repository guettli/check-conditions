package checkconditions

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestCheckReturnsStructuredFindings(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	gvr := schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{
					"type":               "Failed",
					"status":             "True",
					"reason":             "BackoffLimitExceeded",
					"message":            "Job has reached the specified backoff limit",
					"lastTransitionTime": now.Add(-90 * time.Second).Format(time.RFC3339),
				},
				map[string]interface{}{
					"type":    "FailureTarget",
					"status":  "True",
					"reason":  "BackoffLimitExceeded",
					"message": "Job has reached the specified backoff limit",
				},
				map[string]interface{}{
					"type":   "Complete",
					"status": "True",
				},
			},
		},
	}}
	obj.SetNamespace("ns")
	obj.SetName("my-job")
	obj.SetUID("uid-1")

	findings := (&Arguments{}).Check(obj, gvr, now)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.Kind != FindingCondition || f.UID != "uid-1" || f.Reason != "BackoffLimitExceeded" {
		t.Errorf("unexpected finding: %+v", f)
	}
	if got, want := f.Key(), "batch|jobs|ns|my-job|Condition|Failed/FailureTarget"; got != want {
		t.Errorf("Key() = %q, want %q", got, want)
	}
	want := `  ns jobs my-job Condition Failed/FailureTarget=True BackoffLimitExceeded "Job has reached the specified backoff limit" (1m30s)`
	if got := f.Line(now); got != want {
		t.Errorf("Line() =\n%q, want\n%q", got, want)
	}
}

func TestCheckDeletionTimestampUsesNow(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	obj := &unstructured.Unstructured{Object: map[string]interface{}{}}
	obj.SetNamespace("ns")
	obj.SetName("stuck")
	dt := metav1.NewTime(now.Add(-15 * time.Minute))
	obj.SetDeletionTimestamp(&dt)
	args := &Arguments{WarnDeletionTimestampOlderThan: 10 * time.Minute}

	if got := args.Check(obj, gvr, now.Add(-10*time.Minute)); len(got) != 0 {
		t.Errorf("expected no finding 5m after deletion, got %+v", got)
	}
	findings := args.Check(obj, gvr, now)
	if len(findings) != 1 || findings[0].Kind != FindingDeletionTimestamp {
		t.Fatalf("expected one DeletionTimestamp finding, got %+v", findings)
	}
	if got, want := findings[0].Line(now), "  ns pods stuck DeletionTimestamp set for 15m0s"; got != want {
		t.Errorf("Line() = %q, want %q", got, want)
	}
}

func TestSkipResource(t *testing.T) {
	if !SkipResource(schema.GroupResource{Resource: "secrets"}) {
		t.Error("secrets should be skipped")
	}
	if SkipResource(schema.GroupResource{Group: "apps", Resource: "deployments"}) {
		t.Error("deployments should not be skipped")
	}
}
