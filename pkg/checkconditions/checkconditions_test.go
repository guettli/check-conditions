package checkconditions

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	discoveryfake "k8s.io/client-go/discovery/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clientgotesting "k8s.io/client-go/testing"
)

func TestRunWhileRegex(t *testing.T) {
	ctx := t.Context()
	args := &Arguments{}

	discoveryClient := &discoveryfake.FakeDiscovery{
		Fake: &clientgotesting.Fake{
			Resources: []*metav1.APIResourceList{
				{
					GroupVersion: "apps/v1",
					APIResources: []metav1.APIResource{
						{
							Name:       "deployments",
							Namespaced: true,
							Kind:       "Deployment",
							Verbs:      []string{"get", "list", "create", "update", "patch", "delete"},
						},
					},
				},
			},
		},
	}

	// 2) Dynamic fake seeded with some objects
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)

	// Seed as unstructured (ensure apiVersion/kind are set)
	deploy := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":      "nginx",
			"namespace": "default",
		},
	}}

	args.dynClient = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "apps", Version: "v1", Resource: "deployments"}: "DeploymentList",
	}, deploy)

	args.dicoveryClient = discoveryClient
	RunCheckAllConditions(ctx, args)
}
