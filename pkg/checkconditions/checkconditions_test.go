package checkconditions

import (
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
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

func TestHandleConditionSkipsHealthyForeignClusterConditions(t *testing.T) {
	gvr := schema.GroupVersionResource{Resource: "foreignclusters"}
	counter := &handleResourceTypeOutput{}

	tests := []map[string]interface{}{
		{
			"type":    "APIServerStatus",
			"status":  "Established",
			"reason":  "APIServerReady",
			"message": "The foreign cluster API Server is ready",
		},
	}

	var rows []conditionRow
	for _, condition := range tests {
		rows = handleCondition(condition, counter, gvr, rows)
	}

	if len(rows) != 0 {
		t.Fatalf("expected healthy foreign cluster conditions to be suppressed, got %d rows", len(rows))
	}
	if counter.checkedConditions != int32(len(tests)) {
		t.Fatalf("expected %d checked conditions, got %d", len(tests), counter.checkedConditions)
	}
}

func TestHandleConditionSkipsHealthyTigeraConditions(t *testing.T) {
	tests := []struct {
		name      string
		resource  string
		condition map[string]interface{}
		wantRows  int
	}{
		{
			name:     "tigerastatuses Degraded=False is healthy",
			resource: "tigerastatuses",
			condition: map[string]interface{}{
				"type":    "Degraded",
				"status":  "False",
				"reason":  "AllObjectsAvailable",
				"message": "All Objects Available",
			},
			wantRows: 0,
		},
		{
			name:     "tigerastatuses Progressing=False is healthy",
			resource: "tigerastatuses",
			condition: map[string]interface{}{
				"type":    "Progressing",
				"status":  "False",
				"reason":  "AllObjectsAvailable",
				"message": "All Objects Available",
			},
			wantRows: 0,
		},
		{
			name:     "installations Degraded=False is healthy",
			resource: "installations",
			condition: map[string]interface{}{
				"type":    "Degraded",
				"status":  "False",
				"reason":  "AllObjectsAvailable",
				"message": "All Objects Available",
			},
			wantRows: 0,
		},
		{
			name:     "installations Progressing=False is healthy",
			resource: "installations",
			condition: map[string]interface{}{
				"type":    "Progressing",
				"status":  "False",
				"reason":  "AllObjectsAvailable",
				"message": "All Objects Available",
			},
			wantRows: 0,
		},
		{
			name:     "generic Degraded=False is healthy",
			resource: "somethingelse",
			condition: map[string]interface{}{
				"type":    "Degraded",
				"status":  "False",
				"reason":  "AsExpected",
				"message": "",
			},
			wantRows: 0,
		},
		{
			name:     "deployments Progressing=False ProgressDeadlineExceeded is still reported",
			resource: "deployments",
			condition: map[string]interface{}{
				"type":    "Progressing",
				"status":  "False",
				"reason":  "ProgressDeadlineExceeded",
				"message": `ReplicaSet "istio-ingressgateway-76dc58b7f" has timed out progressing.`,
			},
			wantRows: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gvr := schema.GroupVersionResource{Resource: tc.resource}
			counter := &handleResourceTypeOutput{}
			var rows []conditionRow
			rows = handleCondition(tc.condition, counter, gvr, rows)
			if len(rows) != tc.wantRows {
				t.Fatalf("expected %d rows, got %d: %v", tc.wantRows, len(rows), rows)
			}
			if counter.checkedConditions != 1 {
				t.Fatalf("expected 1 checked condition, got %d", counter.checkedConditions)
			}
		})
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

func newDeploymentWithMinimumReplicasUnavailable(desired, available *int64) unstructured.Unstructured {
	obj := unstructured.Unstructured{}
	obj.SetName("topolvm-controller")
	obj.SetNamespace("topolvm-system")
	if desired != nil {
		_ = unstructured.SetNestedField(obj.Object, *desired, "spec", "replicas")
	}
	if available != nil {
		_ = unstructured.SetNestedField(obj.Object, *available, "status", "availableReplicas")
	}
	return obj
}

func minimumReplicasUnavailableConditions() []interface{} {
	return []interface{}{
		map[string]interface{}{
			"type":    "Available",
			"status":  "False",
			"reason":  "MinimumReplicasUnavailable",
			"message": "Deployment does not have minimum availability.",
		},
	}
}

func TestPrintConditionsAppendsDeploymentReplicaDetail(t *testing.T) {
	gvr := schema.GroupVersionResource{Resource: "deployments"}
	args := &Arguments{}
	desired, available := int64(3), int64(1)
	obj := newDeploymentWithMinimumReplicasUnavailable(&desired, &available)

	counter := &handleResourceTypeOutput{}
	lines, _ := printConditions(args, minimumReplicasUnavailableConditions(), counter, gvr, obj)

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "available 1/3") {
		t.Errorf("expected replica detail in output, got: %s", lines[0])
	}
}

func TestPrintConditionsReplicaDetailMissingAvailableReplicas(t *testing.T) {
	gvr := schema.GroupVersionResource{Resource: "deployments"}
	args := &Arguments{}
	desired := int64(3)
	obj := newDeploymentWithMinimumReplicasUnavailable(&desired, nil)

	counter := &handleResourceTypeOutput{}
	lines, _ := printConditions(args, minimumReplicasUnavailableConditions(), counter, gvr, obj)

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "available 0/3") {
		t.Errorf("expected available 0/3 when availableReplicas absent, got: %s", lines[0])
	}
}

func TestPrintConditionsNoReplicaDetailForOtherReason(t *testing.T) {
	gvr := schema.GroupVersionResource{Resource: "deployments"}
	args := &Arguments{}
	desired, available := int64(3), int64(1)
	obj := newDeploymentWithMinimumReplicasUnavailable(&desired, &available)

	conditions := []interface{}{
		map[string]interface{}{
			"type":    "Progressing",
			"status":  "False",
			"reason":  "ProgressDeadlineExceeded",
			"message": "ReplicaSet has timed out progressing.",
		},
	}
	counter := &handleResourceTypeOutput{}
	lines, _ := printConditions(args, conditions, counter, gvr, obj)

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %v", len(lines), lines)
	}
	if strings.Contains(lines[0], "available") {
		t.Errorf("did not expect replica detail for non-MinimumReplicasUnavailable reason, got: %s", lines[0])
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

func newStartingPod(restartCount int64) unstructured.Unstructured {
	obj := unstructured.Unstructured{}
	obj.SetName("agentloop-sharedinbox-issue-300-run")
	obj.SetNamespace("agentloop")
	if restartCount > 0 {
		_ = unstructured.SetNestedSlice(obj.Object, []interface{}{
			map[string]interface{}{
				"name":         "worker",
				"restartCount": restartCount,
			},
		}, "status", "containerStatuses")
	}
	return obj
}

func startingPodConditions(transition time.Time) []interface{} {
	ts := transition.Format(time.RFC3339)
	return []interface{}{
		map[string]interface{}{
			"type":               "ContainersReady",
			"status":             "False",
			"reason":             "ContainersNotReady",
			"message":            "containers with unready status: [worker]",
			"lastTransitionTime": ts,
		},
		map[string]interface{}{
			"type":               "Initialized",
			"status":             "False",
			"reason":             "ContainersNotInitialized",
			"message":            "containers with incomplete status: [inject-agentloop]",
			"lastTransitionTime": ts,
		},
	}
}

func TestPrintConditionsSuppressesStartingPod(t *testing.T) {
	gvr := schema.GroupVersionResource{Resource: "pods"}
	args := &Arguments{PodStartGracePeriod: 30 * time.Second}
	obj := newStartingPod(0)

	counter := &handleResourceTypeOutput{}
	lines, _ := printConditions(args, startingPodConditions(time.Now().Add(-3*time.Second)), counter, gvr, obj)

	if len(lines) != 0 {
		t.Fatalf("expected starting pod conditions to be suppressed, got %d lines: %v", len(lines), lines)
	}
}

func TestPrintConditionsReportsStalledPod(t *testing.T) {
	gvr := schema.GroupVersionResource{Resource: "pods"}
	args := &Arguments{PodStartGracePeriod: 30 * time.Second}
	obj := newStartingPod(0)

	counter := &handleResourceTypeOutput{}
	lines, _ := printConditions(args, startingPodConditions(time.Now().Add(-5*time.Minute)), counter, gvr, obj)

	if len(lines) == 0 {
		t.Fatal("expected a warning line for a pod stuck past the grace period, got none")
	}
}

func TestPrintConditionsReportsRestartedPod(t *testing.T) {
	gvr := schema.GroupVersionResource{Resource: "pods"}
	args := &Arguments{PodStartGracePeriod: 30 * time.Second}
	obj := newStartingPod(1)

	counter := &handleResourceTypeOutput{}
	lines, _ := printConditions(args, startingPodConditions(time.Now().Add(-3*time.Second)), counter, gvr, obj)

	if len(lines) == 0 {
		t.Fatal("expected a warning line for a restarted pod even within the grace period, got none")
	}
}

func TestPrintConditionsGraceDisabled(t *testing.T) {
	gvr := schema.GroupVersionResource{Resource: "pods"}
	args := &Arguments{PodStartGracePeriod: 0}
	obj := newStartingPod(0)

	counter := &handleResourceTypeOutput{}
	lines, _ := printConditions(args, startingPodConditions(time.Now().Add(-3*time.Second)), counter, gvr, obj)

	if len(lines) == 0 {
		t.Fatal("expected a warning line when the grace period is disabled, got none")
	}
}

func TestKubeconfigSource(t *testing.T) {
	t.Run("explicit path wins", func(t *testing.T) {
		rules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: "/tmp/explicit"}
		if got := kubeconfigSource(rules); got != "/tmp/explicit" {
			t.Errorf("expected explicit path, got %q", got)
		}
	})

	t.Run("precedence list without KUBECONFIG env", func(t *testing.T) {
		t.Setenv(clientcmd.RecommendedConfigPathEnvVar, "")
		rules := &clientcmd.ClientConfigLoadingRules{Precedence: []string{"/home/user/.kube/config"}}
		if got := kubeconfigSource(rules); got != "/home/user/.kube/config" {
			t.Errorf("unexpected source %q", got)
		}
	})

	t.Run("notes KUBECONFIG env when set", func(t *testing.T) {
		t.Setenv(clientcmd.RecommendedConfigPathEnvVar, "/env/config")
		rules := &clientcmd.ClientConfigLoadingRules{Precedence: []string{"/env/config"}}
		got := kubeconfigSource(rules)
		if !strings.Contains(got, "/env/config") || !strings.Contains(got, "$"+clientcmd.RecommendedConfigPathEnvVar) {
			t.Errorf("expected source to note KUBECONFIG env, got %q", got)
		}
	})
}
