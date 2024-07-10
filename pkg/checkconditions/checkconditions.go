package checkconditions

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/exp/slices"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Arguments struct {
	Verbose      bool
	WhileRegex   *regexp.Regexp
	WhileForever bool
	StartTime    time.Time
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
	checkAgain           bool
}

func (c *Counter) add(o handleResourceTypeOutput) {
	c.checkedResources += o.checkedResources
	c.checkedConditions += o.checkedConditions
	c.checkedResourceTypes += o.checkedResourceTypes
	if o.checkAgain {
		c.checkAgain = true
	}
}

func RunAll(args Arguments) {
	args.StartTime = time.Now()
	for {
		if RunAllOnce(args) {
			continue
		}
		break
	}
}

// RunAllOnce returns true if command should run again.
func RunAllOnce(args Arguments) bool {
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
	checkAgain, err := RunCheckAllConditions(config, args)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	durationInt := int(time.Since(args.StartTime).Seconds())
	// durationStr as string, without subseconds
	durationStr := time.Duration(durationInt * int(time.Second)).String()
	if !(args.WhileForever || checkAgain) {
		if args.WhileRegex != nil {
			fmt.Printf("Regex %q did not match. Stopping\n", args.WhileRegex.String())
		}

		if durationInt > 5 { //nolint:gomnd
			fmt.Printf("Stopping after %s\n", durationStr)
		}
		return false
	}
	sleepSeconds := 15
	pre := "Running forever. "
	if args.WhileRegex != nil {
		pre = fmt.Sprintf("Regex %q did match. ", args.WhileRegex.String())
	}
	fmt.Printf("%sWaiting %d seconds, then checking again. %s (%s).\n\n",
		pre,
		sleepSeconds,
		time.Now().Format("2006-01-02 15:04:05 -0700 MST"),
		durationStr)
	time.Sleep(time.Duration(sleepSeconds * int(time.Second)))
	return true
}

func RunCheckAllConditions(config *restclient.Config, args Arguments) (bool, error) {
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
	return counter.checkAgain, nil
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

// printResources returns true if the conditions should get checked again N seconds later.
func printResources(args *Arguments, list *unstructured.UnstructuredList, gvr schema.GroupVersionResource,
	counter *handleResourceTypeOutput, workerID int32,
) bool {
	again := false
	for _, obj := range list.Items {
		counter.checkedResources++
		var conditions []interface{}
		var err error
		if gvr.Resource == "hetznerbaremetalhosts" {
			// For some reasons this resource stores the conditions differently
			conditions, _, err = unstructured.NestedSlice(obj.Object, "spec", "status", "conditions")
		} else {
			conditions, _, err = unstructured.NestedSlice(obj.Object, "status", "conditions")
		}
		if err != nil {
			panic(err)
		}
		if printConditions(args, conditions, counter, gvr, obj) {
			again = true
		}
	}
	if args.Verbose {
		fmt.Printf("    checked %s %s %s workerID=%d\n", gvr.Resource, gvr.Group, gvr.Version, workerID)
	}
	return again
}

type conditionRow struct {
	conditionType               string
	conditionStatus             string
	conditionReason             string
	conditionMessage            string
	conditionLastTransitionTime time.Time
}

var readyString = "Ready"

// printConditions returns true if the conditions should be checked again N seconds later.
func printConditions(args *Arguments, conditions []interface{}, counter *handleResourceTypeOutput,
	gvr schema.GroupVersionResource, obj unstructured.Unstructured,
) bool {
	var rows []conditionRow
	for _, condition := range conditions {
		rows = handleCondition(condition, counter, gvr, rows)
	}
	// remove general ready condition, if it is already contained in a particular condition
	// https://pkg.go.dev/sigs.k8s.io/cluster-api/util/conditions#SetSummary
	var ready *conditionRow
	for i := range rows {
		if rows[i].conditionType == readyString {
			ready = &rows[i]
			break
		}
	}
	skipReadyCondition := false
	if ready != nil {
		for _, r := range rows {
			if r.conditionType == readyString {
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
	again := false
	for _, r := range rows {
		if skipReadyCondition && r.conditionType == readyString {
			continue
		}
		duration := ""
		if !r.conditionLastTransitionTime.IsZero() {
			d := time.Since(r.conditionLastTransitionTime)
			duration = fmt.Sprint(d.Round(time.Second))
		}
		outLine := fmt.Sprintf("  %s %s %s Condition %s=%s %s %q (%s)", obj.GetNamespace(), gvr.Resource, obj.GetName(), r.conditionType, r.conditionStatus,
			r.conditionReason, r.conditionMessage, duration)
		fmt.Println(outLine)
		if args.WhileRegex != nil {
			if args.WhileRegex.MatchString(outLine) {
				again = true
			}
		}
	}
	return again
}

func handleCondition(condition interface{}, counter *handleResourceTypeOutput, gvr schema.GroupVersionResource, rows []conditionRow) []conditionRow {
	conditionMap, ok := condition.(map[string]interface{})
	if !ok {
		fmt.Println("Invalid condition format")
		return rows
	}
	counter.checkedConditions++

	conditionType, _ := conditionMap["type"].(string)
	conditionStatus, _ := conditionMap["status"].(string)
	if conditionToSkip(conditionType) {
		return rows
	}
	switch conditionStatus {
	case "True":
		if conditionTypeHasPositiveMeaning(gvr.Resource, conditionType) {
			return rows
		}
	case "False":
		if conditionTypeHasNegativeMeaning(gvr.Resource, conditionType) {
			return rows
		}
	}
	conditionReason, _ := conditionMap["reason"].(string)
	conditionMessage, _ := conditionMap["message"].(string)
	conditionLine := fmt.Sprintf("%s %s=%s %s %q", gvr.Resource, conditionType, conditionStatus, conditionReason, conditionMessage)
	for _, r := range conditionLinesToIgnoreRegexs {
		if r.MatchString(conditionLine) {
			return rows
		}
	}
	if conditionDone(conditionType, conditionStatus, conditionReason) {
		return rows
	}
	s, _ := conditionMap["lastTransitionTime"].(string)
	conditionLastTransitionTime := time.Time{}
	if s != "" {
		conditionLastTransitionTime, _ = time.Parse(time.RFC3339, s)
	}
	rows = append(rows, conditionRow{
		conditionType, conditionStatus,
		conditionReason, conditionMessage, conditionLastTransitionTime,
	})
	return rows
}

func conditionToSkip(ct string) bool {
	// Skip conditions which can be True or False, and both values are fine.
	toSkip := []string{
		"DisruptionAllowed",
		"LoadBalancerAttachedToNetwork",
		"NetworkAttached",
		"PodReadyToStartContainers", // completed pods have "False".
	}
	return slices.Contains(toSkip, ct)
}

var conditionTypesOfResourceWithPositiveMeaning = map[string][]string{
	"extensionconfigs": { // runtime.cluster.x-k8s.io
		"Discovered",
	},
	"hetznerclusters": {
		"ControlPlaneEndpointSet",
	},
	"hetznerbaremetalmachines": {
		"AssociateBMHCondition",
	},
	"horizontalpodautoscalers": {
		"AbleToScale",
		"ScalingActive",
	},
	"hetznerbaremetalhosts": {
		"RootDeviceHintsValidated",
	},
	"clusters": { // postgresql.cnpg.io/v1
		"ContinuousArchiving",
	},
	"clusteraddons": {
		"ClusterAddonConfigValidated",
		"ClusterAddonHelmChartUntarred",
	},
	"engineimages": { // Longhorn
		"ready",
	},
	"nodes": {
		"Schedulable", // Longhorn
		"MountPropagation",
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

// To create new IngoreRegex take the line you see and remove the namespace, the resource name and the time from that line.
// Example:
// from: longhorn-system backuptargets default Condition Unavailable=True Unavailable "backup target URL is empty" (5m21s)
// to: `backuptargets Unavailable=True Unavailable "backup target URL is empty"`
var conditionLinesToIgnoreRegexs = []*regexp.Regexp{
	regexp.MustCompile("machinesets MachinesReady=False Deleted @.*"),
	regexp.MustCompile("machinesets Ready=False Deleted @.*"),

	// Longhorn
	regexp.MustCompile(`backuptargets Unavailable=True Unavailable "backup target URL is empty"`),
	regexp.MustCompile(`engines InstanceCreation=True`),
	regexp.MustCompile(`engines FilesystemReadOnly=False`),
	regexp.MustCompile(`replicas InstanceCreation=True`),
	regexp.MustCompile(`replicas FilesystemReadOnly=False`),
	regexp.MustCompile(`replicas WaitForBackingImage=False`),
	regexp.MustCompile(`volumes WaitForBackingImage=False`),
	regexp.MustCompile(`volumes TooManySnapshots=False`),
	regexp.MustCompile(`volumes Scheduled=True`),
	regexp.MustCompile(`volumes Restore=False`),
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
		"ProviderUpgraded",
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

func conditionDone(conditionType string, conditionStatus string, conditionReason string) bool {
	// machinesets demo-1-md-0-q9qzp-6gsw9 Condition MachinesReady=False Deleted @ Machine/demo-1-md-0-q9qzp-6gsw9-vkxrp ""
	// The reason contains "@ ...". We need to split that
	if conditionType == "MachinesReady" {
		parts := strings.Split(conditionReason, "@")
		conditionType = strings.TrimSpace(parts[0])
	}

	if slices.Contains([]string{"Ready", "ContainersReady", "InfrastructureReady", "MachinesReady"}, conditionType) &&
		slices.Contains([]string{"PodCompleted", "InstanceTerminated", "Deleted"}, conditionReason) &&
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
	checkAgain           bool
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

	output.checkAgain = printResources(args, list, gvr, &output, input.workerID)
	return output
}
