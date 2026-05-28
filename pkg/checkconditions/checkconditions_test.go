package checkconditions

import (
	"testing"

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
