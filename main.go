package main

import (
	"context"
	"fmt"

	"golang.org/x/exp/slices"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var resourcesToSkip = []string{
	"bindings",
	"tokenreviews",
	"selfsubjectaccessreviews",
	"selfsubjectrulesreviews",
	"localsubjectaccessreviews",
	"subjectaccessreviews",
	"componentstatuses",
}

func main() {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeconfig.ClientConfig()
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	discoveryClient := clientset.Discovery()

	// Get the list of all API resources available
	serverResources, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		panic(err.Error())
	}

	for _, group := range serverResources {
		for _, resource := range group.APIResources {
			// Skip subresources like pod/logs, pod/status
			if containsSlash(resource.Name) {
				continue
			}
			if slices.Contains(resourcesToSkip, resource.Name) {
				continue
			}
			gvr := schema.GroupVersionResource{
				Group:    group.GroupVersion,
				Version:  resource.Version,
				Resource: resource.Name,
			}
			if gvr.Group == "v1" {
				gvr.Version = gvr.Group
				gvr.Group = ""
			}
			// if resource.Name != "machines" {
			// 	continue
			// }
			var list *unstructured.UnstructuredList
			if resource.Namespaced {
				list, err = dynClient.Resource(gvr).List(context.TODO(), metav1.ListOptions{})

				if err != nil {
					fmt.Printf("..Error listing %s: %v. group %q version %q resource %q\n", resource.Name, err,
						gvr.Group, gvr.Version, gvr.Resource)
					continue
				}
				printResources(list, resource.Name, gvr)

			} else {
				list, err = dynClient.Resource(gvr).List(context.TODO(), metav1.ListOptions{})
				if err != nil {
					fmt.Printf("..Error listing %s: %v\n", resource.Name, err)
					continue
				}
				printResources(list, resource.Name, gvr)
			}
		}
	}
}

func containsSlash(s string) bool {
	return len(s) > 0 && s[0] == '/'
}

func printResources(list *unstructured.UnstructuredList, resourceName string, gvr schema.GroupVersionResource) {
	fmt.Printf("Found %d resources of type %s. group %q version %q resource %q\n", len(list.Items), resourceName, gvr.Group, gvr.Version, gvr.Resource)
}
