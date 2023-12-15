package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

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
	"selfsubjectreviews",
	"selfsubjectaccessreviews",
	"selfsubjectrulesreviews",
	"localsubjectaccessreviews",
	"subjectaccessreviews",
	"componentstatuses",
}

type Counter struct {
	checkedResources     int32
	checkedConditions    int32
	checkedResourceTypes int32
	startTime            time.Time
}

func (c *Counter) add(o handleResourceTypeOutput) {
	c.checkedResources += o.checkedResources
	c.checkedConditions += o.checkedConditions
	c.checkedResourceTypes += o.checkedResourceTypes
}

func runAll(args Arguments) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeconfig.ClientConfig()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	// 80 concurrent requests were served in roughly 200ms
	// This means 400 requests in one second (to local kind cluster)
	// But why reduce this? I don't want people with better hardware
	// to wait for getting results from an api-server running at localhost
	config.QPS = 1000
	config.Burst = 1000

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
		if discovery.IsGroupDiscoveryFailedError(err) {
			fmt.Printf("WARNING: The Kubernetes server has an orphaned API service. Server reports: %s\n", err.Error())
			fmt.Printf("WARNING: To fix this, kubectl delete apiservice <service-name>\n")
		} else {
			panic(err)
		}
	}

	jobs := make(chan handleResourceTypeInput)
	results := make(chan handleResourceTypeOutput)
	var wg sync.WaitGroup

	// Concurrency needed?
	// Without: 320ms
	// With 10 or more workers: 190ms

	createWorkers(&wg, jobs, results)

	counter := Counter{startTime: time.Now()}

	go func() {
		for result := range results {
			counter.add(result)
		}
	}()

	createJobs(serverResources, jobs, args, dynClient)

	close(jobs)
	wg.Wait()
	close(results)
	fmt.Printf("Checked %d conditions of %d resources of %d types. Duration: %s\n",
		counter.checkedConditions, counter.checkedResources, counter.checkedResourceTypes, time.Since(counter.startTime).Round(time.Millisecond))
}

func createJobs(serverResources []*metav1.APIResourceList, jobs chan handleResourceTypeInput, args Arguments, dynClient *dynamic.DynamicClient) {
	for _, resourceList := range serverResources {
		groupVersion, err := schema.ParseGroupVersion(resourceList.GroupVersion)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse group version: %v\n", err)
			continue
		}
		for i := range resourceList.APIResources {
			jobs <- handleResourceTypeInput{
				args:      &args,
				dynClient: dynClient,
				gvr: schema.GroupVersionResource{
					Group:    groupVersion.Group,
					Version:  groupVersion.Version,
					Resource: resourceList.APIResources[i].Name,
				},
			}
		}
	}
}

func createWorkers(wg *sync.WaitGroup, jobs chan handleResourceTypeInput, results chan handleResourceTypeOutput) {
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(workerID int32) {
			defer wg.Done()
			for input := range jobs {
				input.workerID = workerID
				results <- handleResourceType(input)
			}
		}(int32(i))
	}
}

func containsSlash(s string) bool {
	return len(s) > 0 && s[0] == '/'
}

func printResources(args *Arguments, list *unstructured.UnstructuredList, gvr schema.GroupVersionResource,
	counter *handleResourceTypeOutput, workerID int32,
) {
	for _, obj := range list.Items {
		counter.checkedResources++
		var conditions []interface{}
		var err error
		if gvr.Resource == "hetznerbaremetalhosts" {
			// For some reasons this resource stores the conditions differnetly
			conditions, _, err = unstructured.NestedSlice(obj.Object, "spec", "status", "conditions")
		} else {
			conditions, _, err = unstructured.NestedSlice(obj.Object, "status", "conditions")
		}
		if err != nil {
			panic(err)
		}
		// Convert each condition to a map[string]interface{}
		// Access the desired fields within the condition map
		// For example, to access the "type" and "status" fields:
		printConditions(conditions, counter, gvr, obj)
	}
	if args.verbose {
		fmt.Printf("    checked %s %s %s workerID=%d\n", gvr.Resource, gvr.Group, gvr.Version, workerID)
	}
}

func printConditions(conditions []interface{}, counter *handleResourceTypeOutput, gvr schema.GroupVersionResource, obj unstructured.Unstructured) {
	type row struct {
		conditionType               string
		conditionStatus             string
		conditionReason             string
		conditionMessage            string
		conditionLastTransitionTime time.Time
	}
	var rows []row
	for _, condition := range conditions {
		conditionMap, ok := condition.(map[string]interface{})
		if !ok {
			fmt.Println("Invalid condition format")
			continue
		}
		counter.checkedConditions++

		conditionType, _ := conditionMap["type"].(string)
		conditionStatus, _ := conditionMap["status"].(string)
		if conditionToSkip(conditionType) {
			continue
		}
		if conditionTypeHasPositiveMeaning(gvr.Resource, conditionType) && conditionStatus == "True" {
			continue
		}
		if conditionTypeHasNegativeMeaning(gvr.Resource, conditionType) && conditionStatus == "False" {
			continue
		}
		conditionReason, _ := conditionMap["reason"].(string)
		if conditionDone(gvr.Resource, conditionType, conditionStatus, conditionReason) {
			continue
		}
		s, _ := conditionMap["lastTransitionTime"].(string)
		conditionLastTransitionTime := time.Time{}
		if s != "" {
			conditionLastTransitionTime, _ = time.Parse(time.RFC3339, s)
		}
		conditionMessage, _ := conditionMap["message"].(string)
		rows = append(rows, row{
			conditionType, conditionStatus,
			conditionReason, conditionMessage, conditionLastTransitionTime,
		})
	}
	// remove general ready condition, if it is already contained in a particular condition
	// https://pkg.go.dev/sigs.k8s.io/cluster-api/util/conditions#SetSummary
	var ready *row
	for i := range rows {
		if rows[i].conditionType == "Ready" {
			ready = &rows[i]
			break
		}
	}
	skipReadyCondition := false
	if ready != nil {
		for _, r := range rows {
			if r.conditionType == "Ready" {
				continue
			}
			if r.conditionMessage == ready.conditionMessage &&
				r.conditionReason == ready.conditionReason &&
				r.conditionStatus == ready.conditionStatus {
				skipReadyCondition = true
				break
			}
		}
	}
	for _, r := range rows {
		if skipReadyCondition && r.conditionType == "Ready" {
			continue
		}
		duration := ""
		if !r.conditionLastTransitionTime.IsZero() {
			d := time.Since(r.conditionLastTransitionTime)
			duration = fmt.Sprint(d.Round(time.Second))
		}
		fmt.Printf("  %s %s %s Condition %s=%s %s %q (%s)\n", obj.GetNamespace(), gvr.Resource, obj.GetName(), r.conditionType, r.conditionStatus,
			r.conditionReason, r.conditionMessage, duration)
	}
}

func conditionToSkip(ct string) bool {
	// Skip conditions which can be True or False, and both values are fine.
	toSkip := []string{
		"DisruptionAllowed",
		"LoadBalancerAttachedToNetwork",
		"NetworkAttached",
	}
	return slices.Contains(toSkip, ct)
}

var conditionTypesOfResourceWithPositiveMeaning = map[string][]string{
	"hetznerbaremetalmachines": {
		"AssociateBMHCondition",
	},
	"horizontalpodautoscalers": {
		"AbleToScale",
		"ScalingActive",
	},
}

var conditionTypesOfResourceWithNegativeMeaning = map[string][]string{
	"nodes": {
		"KernelDeadlock",
		"ReadonlyFilesystem",
		"FrequentUnregisterNetDevice",
		"NTPProblem",
	},
	"horizontalpodautoscalers": {
		"ScalingLimited",
	},
}

func conditionTypeHasPositiveMeaning(resource string, ct string) bool {
	types := conditionTypesOfResourceWithPositiveMeaning[resource]
	if slices.Contains(types, ct) {
		return true
	}

	for _, suffix := range []string{
		"Applied",
		"Approved",
		"Available",
		"Built",
		"Complete",
		"Created",
		"Downloaded",
		"Established",
		"Healthy",
		"Initialized",
		"Installed",
		"LoadBalancerAttached",
		"NamesAccepted",
		"Passed",
		"PodScheduled",
		"Progressing",
		"Provisioned",
		"Reachable",
		"Ready",
		"Reconciled",
		"RemediationAllowed",
		"Resized",
		"Succeeded",
		"Synced",
		"UpToDate",
	} {
		if strings.HasSuffix(ct, suffix) {
			return true
		}
	}
	for _, prefix := range []string{
		"Created",
	} {
		if strings.HasPrefix(ct, prefix) {
			return true
		}
	}
	return false
}

func conditionDone(resource string, conditionType string, conditionStatus string, conditionReason string) bool {
	if slices.Contains([]string{"Ready", "ContainersReady", "InfrastructureReady"}, conditionType) &&
		slices.Contains([]string{"PodCompleted", "InstanceTerminated"}, conditionReason) &&
		conditionStatus == "False" {
		return true
	}
	return false
}

func conditionTypeHasNegativeMeaning(resource string, ct string) bool {
	types := conditionTypesOfResourceWithNegativeMeaning[resource]
	if slices.Contains(types, ct) {
		return true
	}

	for _, suffix := range []string{
		"Unavailable", "Pressure", "Dangling", "Unhealthy",
	} {
		if strings.HasSuffix(ct, suffix) {
			return true
		}
	}
	if strings.HasPrefix(ct, "Frequent") && strings.HasSuffix(ct, "Restart") {
		return true
	}

	return false
}

type handleResourceTypeInput struct {
	args      *Arguments
	dynClient *dynamic.DynamicClient
	gvr       schema.GroupVersionResource
	workerID  int32
}

type handleResourceTypeOutput struct {
	checkedResourceTypes int32
	checkedResources     int32
	checkedConditions    int32
}

func handleResourceType(input handleResourceTypeInput) handleResourceTypeOutput {
	var output handleResourceTypeOutput

	args := input.args
	name := input.gvr.Resource
	dynClient := input.dynClient
	gvr := input.gvr
	// Skip subresources like pod/logs, pod/status
	if containsSlash(name) {
		return output
	}
	if slices.Contains(resourcesToSkip, name) {
		return output
	}

	output.checkedResourceTypes++

	list, err := dynClient.Resource(gvr).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("..Error listing %s: %v. group %q version %q resource %q\n", name, err,
			gvr.Group, gvr.Version, gvr.Resource)
		return output
	}

	printResources(args, list, gvr, &output, input.workerID)

	return output
}
