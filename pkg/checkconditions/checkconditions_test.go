package checkconditions

import (
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestHandleConditionSkipsHealthyMachineConditions(t *testing.T) {
	gvr := schema.GroupVersionResource{Resource: "machines"}
	counter := &handleResourceTypeOutput{}

	tests := []map[string]interface{}{
		{
			"type":    "NodeKubeadmLabelsAndTaintsSet",
			"status":  "True",
			"reason":  "Set",
			"message": "",
		},
		{
			"type":    "Updating",
			"status":  "False",
			"reason":  "NotUpdating",
			"message": "",
		},
	}

	var rows []conditionRow
	for _, condition := range tests {
		rows = handleCondition(condition, counter, gvr, rows)
	}

	if len(rows) != 0 {
		t.Fatalf("expected healthy machine conditions to be suppressed, got %d rows", len(rows))
	}
	if counter.checkedConditions != int32(len(tests)) {
		t.Fatalf("expected %d checked conditions, got %d", len(tests), counter.checkedConditions)
	}
}

func TestPrintConditionsMergesDuplicateReasonMessage(t *testing.T) {
	gvr := schema.GroupVersionResource{Resource: "jobs"}
	args := &Arguments{}
	obj := unstructured.Unstructured{}
	obj.SetName("my-job")
	obj.SetNamespace("agentloop")

	conditions := []interface{}{
		map[string]interface{}{
			"type":    "Failed",
			"status":  "True",
			"reason":  "BackoffLimitExceeded",
			"message": "Job has reached the specified backoff limit",
		},
		map[string]interface{}{
			"type":    "FailureTarget",
			"status":  "True",
			"reason":  "BackoffLimitExceeded",
			"message": "Job has reached the specified backoff limit",
		},
	}
	counter := &handleResourceTypeOutput{}
	lines, _ := printConditions(args, conditions, counter, gvr, obj)

	if len(lines) != 1 {
		t.Fatalf("expected 1 merged line, got %d: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "Failed/FailureTarget=True") {
		t.Errorf("expected merged condition types in output, got: %s", lines[0])
	}
}

func TestPrintResourcesWarnsDeletionTimestamp(t *testing.T) {
	gvr := schema.GroupVersionResource{Resource: "pods"}
	args := &Arguments{
		WarnDeletionTimestampOlderThan: 10 * time.Minute,
	}

	oldTime := metav1.NewTime(time.Now().Add(-15 * time.Minute))
	obj := unstructured.Unstructured{}
	obj.SetName("stuck-pod")
	obj.SetNamespace("default")
	obj.SetDeletionTimestamp(&oldTime)

	list := &unstructured.UnstructuredList{Items: []unstructured.Unstructured{obj}}
	counter := &handleResourceTypeOutput{}
	lines, _ := printResources(args, list, gvr, counter, 0)

	if len(lines) == 0 {
		t.Fatal("expected a warning line for old deletionTimestamp, got none")
	}
	if !strings.Contains(lines[0], "DeletionTimestamp") {
		t.Errorf("expected line to mention DeletionTimestamp, got: %s", lines[0])
	}
}

func TestPrintResourcesNoWarnRecentDeletionTimestamp(t *testing.T) {
	gvr := schema.GroupVersionResource{Resource: "pods"}
	args := &Arguments{
		WarnDeletionTimestampOlderThan: 10 * time.Minute,
	}

	recentTime := metav1.NewTime(time.Now().Add(-1 * time.Minute))
	obj := unstructured.Unstructured{}
	obj.SetName("deleting-pod")
	obj.SetNamespace("default")
	obj.SetDeletionTimestamp(&recentTime)

	list := &unstructured.UnstructuredList{Items: []unstructured.Unstructured{obj}}
	counter := &handleResourceTypeOutput{}
	lines, _ := printResources(args, list, gvr, counter, 0)

	for _, l := range lines {
		if strings.Contains(l, "DeletionTimestamp") {
			t.Errorf("expected no warning for recent deletionTimestamp, got: %s", l)
		}
	}
}

func TestPrintResourcesDisabledDeletionTimestampCheck(t *testing.T) {
	gvr := schema.GroupVersionResource{Resource: "pods"}
	args := &Arguments{
		WarnDeletionTimestampOlderThan: 0, // disabled
	}

	oldTime := metav1.NewTime(time.Now().Add(-1 * time.Hour))
	obj := unstructured.Unstructured{}
	obj.SetName("old-pod")
	obj.SetNamespace("default")
	obj.SetDeletionTimestamp(&oldTime)

	list := &unstructured.UnstructuredList{Items: []unstructured.Unstructured{obj}}
	counter := &handleResourceTypeOutput{}
	lines, _ := printResources(args, list, gvr, counter, 0)

	for _, l := range lines {
		if strings.Contains(l, "DeletionTimestamp") {
			t.Errorf("expected no warning when check is disabled, got: %s", l)
		}
	}
}
