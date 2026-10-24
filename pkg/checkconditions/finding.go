package checkconditions

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// FindingKind tells which check produced a Finding.
type FindingKind string

const (
	// FindingCondition is one or more unhealthy entries of status.conditions
	// which share the same status, reason and message.
	FindingCondition FindingKind = "Condition"
	// FindingDeletionTimestamp is a resource whose deletionTimestamp is older
	// than Arguments.WarnDeletionTimestampOlderThan.
	FindingDeletionTimestamp FindingKind = "DeletionTimestamp"
	// FindingContainerRestarts is a Pod container which restarted at least
	// Arguments.PodRestartWarnCount times and is still unhealthy.
	FindingContainerRestarts FindingKind = "ContainerRestarts"
	// FindingError is an object whose conditions could not be read.
	FindingError FindingKind = "Error"
)

// Finding is one problem of one object. Use Line to format it like the
// check-conditions CLI, and Key to identify it across checks.
type Finding struct {
	Kind      FindingKind
	GVR       schema.GroupVersionResource
	Namespace string
	Name      string
	UID       types.UID

	// ConditionTypes holds the (sorted) condition types of a FindingCondition.
	// Several types are merged into one finding when they carry the same
	// status, reason and message.
	ConditionTypes []string
	Status         string
	Reason         string
	// Message is the condition message, or the error text of a FindingError.
	Message string
	// LastTransitionTime is the condition's lastTransitionTime, or the
	// deletionTimestamp of a FindingDeletionTimestamp. Zero if unknown.
	LastTransitionTime time.Time
	// Detail is optional extra context, e.g. "available 1/3" for Deployments.
	Detail string

	// Container, RestartCount and State are set for FindingContainerRestarts.
	Container    string
	RestartCount int64
	State        string
}

func newFinding(obj *unstructured.Unstructured, gvr schema.GroupVersionResource, kind FindingKind) Finding {
	return Finding{
		Kind:      kind,
		GVR:       gvr,
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
		UID:       obj.GetUID(),
	}
}

// Key identifies a finding across checks: it does not contain values which
// change while the problem persists (age, status, reason, message, restart
// count). Merged condition types are part of the key, so a finding whose
// merged types change gets a new key.
func (f *Finding) Key() string {
	sub := ""
	switch f.Kind {
	case FindingCondition:
		sub = strings.Join(f.ConditionTypes, "/")
	case FindingContainerRestarts:
		sub = f.Container
	case FindingDeletionTimestamp, FindingError:
	}
	return strings.Join([]string{f.GVR.Group, f.GVR.Resource, f.Namespace, f.Name, string(f.Kind), sub}, "|")
}

// Line formats the finding like the output of the check-conditions CLI.
// Durations are relative to now.
func (f *Finding) Line(now time.Time) string {
	switch f.Kind {
	case FindingCondition:
		return f.conditionLine(strings.Join(f.ConditionTypes, "/"), now)
	case FindingDeletionTimestamp:
		return fmt.Sprintf("  %s %s %s DeletionTimestamp set for %s",
			f.Namespace, f.GVR.Resource, f.Name, now.Sub(f.LastTransitionTime).Round(time.Second))
	case FindingContainerRestarts:
		return fmt.Sprintf("  %s %s %s container %s restarted %d times, currently %s",
			f.Namespace, f.GVR.Resource, f.Name, f.Container, f.RestartCount, f.State)
	case FindingError:
		return f.Message
	}
	return fmt.Sprintf("  %s %s %s %s", f.Namespace, f.GVR.Resource, f.Name, f.Kind)
}

func (f *Finding) conditionLine(conditionType string, now time.Time) string {
	duration := ""
	if !f.LastTransitionTime.IsZero() {
		duration = fmt.Sprint(now.Sub(f.LastTransitionTime).Round(time.Second))
	}
	detail := ""
	if f.Detail != "" {
		detail = " " + f.Detail
	}
	return fmt.Sprintf("  %s %s %s Condition %s=%s %s %q (%s)%s", f.Namespace, f.GVR.Resource, f.Name,
		conditionType, f.Status, f.Reason, f.Message, duration, detail)
}

// whileCandidateLines returns the lines the --while regex is matched against.
// For merged conditions each single type is tried first, for backward
// compatibility with regexes that match one condition type (e.g.
// "Failed=True"), then the merged line.
func (f *Finding) whileCandidateLines(now time.Time) []string {
	if f.Kind != FindingCondition || len(f.ConditionTypes) < 2 {
		return []string{f.Line(now)}
	}
	lines := make([]string, 0, len(f.ConditionTypes)+1)
	for _, t := range f.ConditionTypes {
		lines = append(lines, f.conditionLine(t, now))
	}
	return append(lines, f.Line(now))
}

// Check returns the findings of one object, using the same rules as the
// check-conditions CLI. Namespace filters (NamespacePatterns,
// ExcludeNamespacePatterns) are not applied: that is the caller's job. Ages are
// computed relative to now.
func (a *Arguments) Check(obj *unstructured.Unstructured, gvr schema.GroupVersionResource, now time.Time) []Finding {
	var counter handleResourceTypeOutput
	return a.objectFindings(obj, gvr, &counter, now)
}

// SkipResource reports whether resources of this type are never checked:
// subresources and types without status conditions (e.g. secrets, events).
func SkipResource(gr schema.GroupResource) bool {
	// Skip subresources like pod/logs, pod/status
	if containsSlash(gr.Resource) {
		return true
	}
	return slices.Contains(resourcesToSkip, gr)
}
