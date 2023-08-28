package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/exp/slices"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/spf13/cobra"
)

// allCmd represents the all command
var allCmd = &cobra.Command{
	Use:   "all",
	Short: "Check all conditions of all api-resources",
	Long:  `...`,
	Run: func(cmd *cobra.Command, args []string) {
		runAll(arguments)
	},
}

func init() {
	rootCmd.AddCommand(allCmd)
}

var resourcesToSkip = []string{
	"bindings",
	"tokenreviews",
	"selfsubjectaccessreviews",
	"selfsubjectrulesreviews",
	"localsubjectaccessreviews",
	"subjectaccessreviews",
	"componentstatuses",
}

type Counter struct {
	checkedCrds       int
	checkedConditions int
	startTime         time.Time
}

func runAll(args Arguments) {
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

	counter := Counter{startTime: time.Now()}
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

			// core resource types (pods, deployments, ...) need this.
			// I found his by trial-and-error. Is this correct?
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
				printResources(args, list, resource.Name, gvr, &counter)

			} else {
				list, err = dynClient.Resource(gvr).List(context.TODO(), metav1.ListOptions{})
				if err != nil {
					fmt.Printf("..Error listing %s: %v\n", resource.Name, err)
					continue
				}
				printResources(args, list, resource.Name, gvr, &counter)
			}
		}
	}
	fmt.Printf("Checked %d conditions of %d CRDs. Duration: %s\n",
		counter.checkedConditions, counter.checkedCrds, time.Since(counter.startTime))
}

func containsSlash(s string) bool {
	return len(s) > 0 && s[0] == '/'
}

func printResources(args Arguments, list *unstructured.UnstructuredList, resourceName string, gvr schema.GroupVersionResource,
	counter *Counter) {
	//fmt.Printf("Found %d resources of type %s. group %q version %q resource %q\n", len(list.Items), resourceName, gvr.Group, gvr.Version, gvr.Resource)
	for _, obj := range list.Items {
		counter.checkedCrds++
		conditions, _, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
		if err != nil {
			panic(err)
		}
		// Iterate over the conditions slice
		for _, condition := range conditions {
			// Convert each condition to a map[string]interface{}
			conditionMap, ok := condition.(map[string]interface{})
			if !ok {
				fmt.Println("Invalid condition format")
				continue
			}
			counter.checkedConditions++
			// Access the desired fields within the condition map
			// For example, to access the "type" and "status" fields:
			conditionType, _ := conditionMap["type"].(string)
			conditionStatus, _ := conditionMap["status"].(string)
			if conditionToSkip(conditionType) {
				continue
			}
			if conditionTypeHasPositiveMeaning(conditionType) && conditionStatus == "True" {
				continue
			}
			if conditionTypeHasNegativeMeaning(conditionType) && conditionStatus == "False" {
				continue
			}
			fmt.Printf("  %s %s %s Condition %s=%s\n", obj.GetNamespace(), gvr.Resource, obj.GetName(), conditionType, conditionStatus)
		}
	}
	if args.verbose {
		fmt.Printf("  checked %s %s %s\n", gvr.Resource, gvr.Group, gvr.Version)
	}
}

func conditionToSkip(ct string) bool {
	// Skip conditions which can be True or False, and both values are fine.
	var toSkip = []string{
		"DisruptionAllowed",
		"LoadBalancerAttachedToNetwork",
		"NetworkAttached",
	}
	return slices.Contains(toSkip, ct)
}
func conditionTypeHasPositiveMeaning(ct string) bool {
	for _, suffix := range []string{
		"Ready", "Succeeded", "Healthy", "Available", "Approved",
		"Initialized", "PodScheduled", "Complete", "Established",
		"NamesAccepted", "Synced", "Created", "Resized",
		"Progressing", "RemediationAllowed",
		"LoadBalancerAttached",
	} {
		if strings.HasSuffix(ct, suffix) {
			return true
		}
	}
	return false
}

func conditionTypeHasNegativeMeaning(ct string) bool {
	for _, suffix := range []string{
		"Unavailable", "Pressure", "Dangling",
	} {
		if strings.HasSuffix(ct, suffix) {
			return true
		}
	}
	return false
}
