package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	clustermode "kube-budget/internal/application/cluster"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

type Collector struct {
	client    kubernetes.Interface
	cluster   clustermode.ClusterInfo
	namespace string
}

func NewCollector(client kubernetes.Interface, cluster clustermode.ClusterInfo, namespace string) *Collector {
	return &Collector{client: client, cluster: cluster, namespace: strings.TrimSpace(namespace)}
}

// listed is one API call's answer.
type listed[T any] struct {
	value T
	err   error
}

// clusterLists holds every list the collector needs. They are independent, so
// they are requested at once: a snapshot waits for the slowest call instead of
// the sum of them all, which matters on remote API servers like EKS.
type clusterLists struct {
	pods         listed[*corev1.PodList]
	nodes        listed[*corev1.NodeList]
	deployments  listed[*appsv1.DeploymentList]
	statefulSets listed[*appsv1.StatefulSetList]
	daemonSets   listed[*appsv1.DaemonSetList]
	jobs         listed[*batchv1.JobList]
	cronJobs     listed[*batchv1.CronJobList]
	configMaps   listed[*corev1.ConfigMapList]
	secrets      listed[*corev1.SecretList]
	services     listed[*corev1.ServiceList]
	claims       listed[*corev1.PersistentVolumeClaimList]
	quotas       listed[*corev1.ResourceQuotaList]
	limitRanges  listed[*corev1.LimitRangeList]
	podMetrics   listed[[]byte]
}

func fetch[T any](group *sync.WaitGroup, result *listed[T], call func() (T, error)) {
	group.Add(1)
	go func() {
		defer group.Done()
		result.value, result.err = call()
	}()
}

func (collector *Collector) list(ctx context.Context) *clusterLists {
	lists := &clusterLists{}
	var group sync.WaitGroup
	fetch(&group, &lists.pods, func() (*corev1.PodList, error) {
		return collector.client.CoreV1().Pods(collector.namespace).List(ctx, metav1.ListOptions{})
	})
	fetch(&group, &lists.nodes, func() (*corev1.NodeList, error) {
		return collector.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	})
	fetch(&group, &lists.deployments, func() (*appsv1.DeploymentList, error) {
		return collector.client.AppsV1().Deployments(collector.namespace).List(ctx, metav1.ListOptions{})
	})
	fetch(&group, &lists.statefulSets, func() (*appsv1.StatefulSetList, error) {
		return collector.client.AppsV1().StatefulSets(collector.namespace).List(ctx, metav1.ListOptions{})
	})
	fetch(&group, &lists.daemonSets, func() (*appsv1.DaemonSetList, error) {
		return collector.client.AppsV1().DaemonSets(collector.namespace).List(ctx, metav1.ListOptions{})
	})
	fetch(&group, &lists.jobs, func() (*batchv1.JobList, error) {
		return collector.client.BatchV1().Jobs(collector.namespace).List(ctx, metav1.ListOptions{})
	})
	fetch(&group, &lists.cronJobs, func() (*batchv1.CronJobList, error) {
		return collector.client.BatchV1().CronJobs(collector.namespace).List(ctx, metav1.ListOptions{})
	})
	fetch(&group, &lists.configMaps, func() (*corev1.ConfigMapList, error) {
		return collector.client.CoreV1().ConfigMaps(collector.namespace).List(ctx, metav1.ListOptions{})
	})
	fetch(&group, &lists.secrets, func() (*corev1.SecretList, error) {
		return collector.client.CoreV1().Secrets(collector.namespace).List(ctx, metav1.ListOptions{})
	})
	fetch(&group, &lists.services, func() (*corev1.ServiceList, error) {
		return collector.client.CoreV1().Services(collector.namespace).List(ctx, metav1.ListOptions{})
	})
	fetch(&group, &lists.claims, func() (*corev1.PersistentVolumeClaimList, error) {
		return collector.client.CoreV1().PersistentVolumeClaims(collector.namespace).List(ctx, metav1.ListOptions{})
	})
	fetch(&group, &lists.quotas, func() (*corev1.ResourceQuotaList, error) {
		return collector.client.CoreV1().ResourceQuotas(collector.namespace).List(ctx, metav1.ListOptions{})
	})
	fetch(&group, &lists.limitRanges, func() (*corev1.LimitRangeList, error) {
		return collector.client.CoreV1().LimitRanges(collector.namespace).List(ctx, metav1.ListOptions{})
	})
	fetch(&group, &lists.podMetrics, func() ([]byte, error) {
		return collector.podMetrics(ctx)
	})
	group.Wait()
	return lists
}

func (collector *Collector) Collect(ctx context.Context) (clustermode.CollectedData, error) {
	data := clustermode.CollectedData{
		Cluster:            collector.cluster,
		Nodes:              make([]clustermode.Node, 0),
		Workloads:          make([]clustermode.Workload, 0),
		Pods:               make([]clustermode.Pod, 0),
		Resources:          make([]clustermode.Resource, 0),
		NamespacePodCounts: make(map[string]int),
		Warnings:           make([]clustermode.Warning, 0),
	}
	lists := collector.list(ctx)

	pods, err := lists.pods.value, lists.pods.err
	podItems := make([]corev1.Pod, 0)
	if err != nil {
		data.Warnings = append(data.Warnings, collectionWarning("pods", err))
	} else {
		podItems = pods.Items
		collectPods(podItems, &data)
	}

	nodes, err := lists.nodes.value, lists.nodes.err
	if err != nil {
		data.Warnings = append(data.Warnings, collectionWarning("nodes", err))
	} else {
		data.Nodes = collectNodes(nodes.Items, podItems)
	}

	deployments, err := lists.deployments.value, lists.deployments.err
	if err != nil {
		data.Warnings = append(data.Warnings, collectionWarning("deployments", err))
	} else {
		for index := range deployments.Items {
			data.Workloads = append(data.Workloads, deploymentWorkload(&deployments.Items[index]))
		}
	}

	statefulSets, err := lists.statefulSets.value, lists.statefulSets.err
	if err != nil {
		data.Warnings = append(data.Warnings, collectionWarning("statefulsets", err))
	} else {
		for index := range statefulSets.Items {
			data.Workloads = append(data.Workloads, statefulSetWorkload(&statefulSets.Items[index]))
		}
	}

	daemonSets, err := lists.daemonSets.value, lists.daemonSets.err
	if err != nil {
		data.Warnings = append(data.Warnings, collectionWarning("daemonsets", err))
	} else {
		for index := range daemonSets.Items {
			data.Workloads = append(data.Workloads, daemonSetWorkload(&daemonSets.Items[index]))
		}
	}

	collectPodUsage(lists.podMetrics, &data)
	collectBatchResources(lists, &data)
	collectCoreResources(lists, &data)

	sort.Slice(data.Workloads, func(left, right int) bool {
		if data.Workloads[left].Namespace == data.Workloads[right].Namespace {
			return data.Workloads[left].Name < data.Workloads[right].Name
		}
		return data.Workloads[left].Namespace < data.Workloads[right].Namespace
	})
	sort.Slice(data.Resources, func(left, right int) bool {
		if data.Resources[left].Category != data.Resources[right].Category {
			return data.Resources[left].Category < data.Resources[right].Category
		}
		if data.Resources[left].Namespace != data.Resources[right].Namespace {
			return data.Resources[left].Namespace < data.Resources[right].Namespace
		}
		if data.Resources[left].Kind != data.Resources[right].Kind {
			return data.Resources[left].Kind < data.Resources[right].Kind
		}
		return data.Resources[left].Name < data.Resources[right].Name
	})
	return data, nil
}

func collectBatchResources(lists *clusterLists, data *clustermode.CollectedData) {
	jobs, err := lists.jobs.value, lists.jobs.err
	if err != nil {
		data.Warnings = append(data.Warnings, collectionWarning("jobs", err))
	} else {
		for index := range jobs.Items {
			data.Resources = append(data.Resources, jobResource(&jobs.Items[index]))
		}
	}

	cronJobs, err := lists.cronJobs.value, lists.cronJobs.err
	if err != nil {
		data.Warnings = append(data.Warnings, collectionWarning("cronjobs", err))
	} else {
		for index := range cronJobs.Items {
			data.Resources = append(data.Resources, cronJobResource(&cronJobs.Items[index]))
		}
	}
}

func collectCoreResources(lists *clusterLists, data *clustermode.CollectedData) {
	configMaps, err := lists.configMaps.value, lists.configMaps.err
	if err != nil {
		data.Warnings = append(data.Warnings, collectionWarning("configmaps", err))
	} else {
		for index := range configMaps.Items {
			configMap := &configMaps.Items[index]
			data.Resources = append(data.Resources, inventoryResource(configMap.UID, "ConfigMap", configMap.Name, configMap.Namespace, "Configuration", "Available", map[string]string{
				"Keys": strconv.Itoa(len(configMap.Data) + len(configMap.BinaryData)),
			}))
		}
	}

	secrets, err := lists.secrets.value, lists.secrets.err
	if err != nil {
		data.Warnings = append(data.Warnings, collectionWarning("secrets", err))
	} else {
		for index := range secrets.Items {
			secret := &secrets.Items[index]
			data.Resources = append(data.Resources, inventoryResource(secret.UID, "Secret", secret.Name, secret.Namespace, "Configuration", "Available", map[string]string{
				"Keys": strconv.Itoa(len(secret.Data)), "Type": string(secret.Type),
			}))
		}
	}

	services, err := lists.services.value, lists.services.err
	if err != nil {
		data.Warnings = append(data.Warnings, collectionWarning("services", err))
	} else {
		for index := range services.Items {
			service := &services.Items[index]
			data.Resources = append(data.Resources, inventoryResource(service.UID, "Service", service.Name, service.Namespace, "Networking", "Active", map[string]string{
				"Type": string(service.Spec.Type), "Cluster IP": service.Spec.ClusterIP, "Ports": strconv.Itoa(len(service.Spec.Ports)),
			}))
		}
	}

	claims, err := lists.claims.value, lists.claims.err
	if err != nil {
		data.Warnings = append(data.Warnings, collectionWarning("persistentvolumeclaims", err))
	} else {
		for index := range claims.Items {
			claim := &claims.Items[index]
			storage := resourceListValues(claim.Spec.Resources.Requests)
			item := inventoryResource(claim.UID, "PersistentVolumeClaim", claim.Name, claim.Namespace, "Storage", string(claim.Status.Phase), map[string]string{
				"Requested": formatStorage(storage.StorageBytes), "Volume": claim.Spec.VolumeName, "Storage class": valueOrDefault(claim.Spec.StorageClassName, "default"),
			})
			item.Requests = storage
			data.Resources = append(data.Resources, item)
		}
	}

	quotas, err := lists.quotas.value, lists.quotas.err
	if err != nil {
		data.Warnings = append(data.Warnings, collectionWarning("resourcequotas", err))
	} else {
		for index := range quotas.Items {
			quota := &quotas.Items[index]
			attributes := make(map[string]string, len(quota.Status.Hard))
			for name, hard := range quota.Status.Hard {
				used := quota.Status.Used[name]
				attributes[string(name)] = used.String() + " / " + hard.String()
			}
			if len(attributes) == 0 {
				for name, hard := range quota.Spec.Hard {
					attributes[string(name)] = "0 / " + hard.String()
				}
			}
			data.Resources = append(data.Resources, inventoryResource(quota.UID, "ResourceQuota", quota.Name, quota.Namespace, "Policy", "Active", attributes))
		}
	}

	limitRanges, err := lists.limitRanges.value, lists.limitRanges.err
	if err != nil {
		data.Warnings = append(data.Warnings, collectionWarning("limitranges", err))
	} else {
		for index := range limitRanges.Items {
			limitRange := &limitRanges.Items[index]
			data.Resources = append(data.Resources, inventoryResource(limitRange.UID, "LimitRange", limitRange.Name, limitRange.Namespace, "Policy", "Active", map[string]string{
				"Rules": strconv.Itoa(len(limitRange.Spec.Limits)),
			}))
		}
	}
}

func jobResource(job *batchv1.Job) clustermode.Resource {
	status := "Pending"
	if job.Status.Failed > 0 {
		status = "Failed"
	} else if job.Status.Succeeded > 0 && job.Status.CompletionTime != nil {
		status = "Completed"
	} else if job.Status.Active > 0 {
		status = "Running"
	}
	_, requests, _ := containerRequests(&job.Spec.Template.Spec)
	item := inventoryResource(job.UID, "Job", job.Name, job.Namespace, "Batch", status, map[string]string{
		"Active": strconv.Itoa(int(job.Status.Active)), "Succeeded": strconv.Itoa(int(job.Status.Succeeded)), "Failed": strconv.Itoa(int(job.Status.Failed)),
	})
	item.Requests = requests
	return item
}

func cronJobResource(cronJob *batchv1.CronJob) clustermode.Resource {
	status := "Scheduled"
	if cronJob.Spec.Suspend != nil && *cronJob.Spec.Suspend {
		status = "Suspended"
	}
	lastSchedule := "Never"
	if cronJob.Status.LastScheduleTime != nil {
		lastSchedule = cronJob.Status.LastScheduleTime.Time.Format("2006-01-02 15:04:05")
	}
	_, requests, _ := containerRequests(&cronJob.Spec.JobTemplate.Spec.Template.Spec)
	item := inventoryResource(cronJob.UID, "CronJob", cronJob.Name, cronJob.Namespace, "Batch", status, map[string]string{
		"Schedule": cronJob.Spec.Schedule, "Active jobs": strconv.Itoa(len(cronJob.Status.Active)), "Last schedule": lastSchedule,
	})
	item.Requests = requests
	return item
}

func inventoryResource(uid types.UID, kind, name, namespace, category, status string, attributes map[string]string) clustermode.Resource {
	return clustermode.Resource{UID: string(uid), Kind: kind, Name: name, Namespace: namespace, Category: category, Status: status, Attributes: attributes}
}

func valueOrDefault(value *string, fallback string) string {
	if value == nil || *value == "" {
		return fallback
	}
	return *value
}

func formatStorage(bytes int64) string {
	const gibibyte = int64(1024 * 1024 * 1024)
	if bytes >= gibibyte {
		return fmt.Sprintf("%.1f GiB", float64(bytes)/float64(gibibyte))
	}
	return fmt.Sprintf("%d bytes", bytes)
}

func collectPods(pods []corev1.Pod, data *clustermode.CollectedData) {
	for index := range pods {
		pod := &pods[index]
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		data.NamespacePodCounts[pod.Namespace]++

		_, _, missing := containerRequests(&pod.Spec)
		ownerKind, ownerName := podOwner(pod)
		data.Pods = append(data.Pods, clustermode.Pod{
			Name:             pod.Name,
			Namespace:        pod.Namespace,
			NodeName:         pod.Spec.NodeName,
			Phase:            string(pod.Status.Phase),
			OwnerKind:        ownerKind,
			OwnerName:        ownerName,
			Requests:         podRequests(&pod.Spec),
			MissingRequests:  len(missing) > 0,
			MissingResources: missing,
			Claims:           podClaims(&pod.Spec),
		})
	}
}

// podMetricsList is the subset of the metrics.k8s.io PodMetricsList the
// collector reads. It is decoded locally to avoid a dependency on the metrics
// client for two fields.
type podMetricsList struct {
	Items []struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Containers []struct {
			Usage map[corev1.ResourceName]resource.Quantity `json:"usage"`
		} `json:"containers"`
	} `json:"items"`
}

// collectPodUsage reads current pod usage from metrics-server. A cluster
// without metrics-server still produces a full snapshot, only without usage.
func collectPodUsage(metrics listed[[]byte], data *clustermode.CollectedData) {
	if len(data.Pods) == 0 {
		return
	}
	if metrics.err != nil {
		data.Warnings = append(data.Warnings, clustermode.Warning{
			Resource: "pod metrics",
			Message:  fmt.Sprintf("metrics-server is not reachable, so usage and efficiency are unavailable: %v", metrics.err),
		})
		return
	}
	if metrics.value == nil {
		return
	}
	if err := applyPodMetrics(metrics.value, data.Pods); err != nil {
		data.Warnings = append(data.Warnings, collectionWarning("pod metrics", err))
	}
}

// podMetrics reads the raw metrics-server pod list; nil without a REST client.
func (collector *Collector) podMetrics(ctx context.Context) ([]byte, error) {
	client := collector.client.Discovery().RESTClient()
	if client == nil || reflect.ValueOf(client).IsNil() {
		return nil, nil
	}
	path := "/apis/metrics.k8s.io/v1beta1/pods"
	if collector.namespace != "" {
		path = "/apis/metrics.k8s.io/v1beta1/namespaces/" + collector.namespace + "/pods"
	}
	return client.Get().AbsPath(path).DoRaw(ctx)
}

// applyPodMetrics sums container usage into the matching collected pods.
func applyPodMetrics(raw []byte, pods []clustermode.Pod) error {
	var metrics podMetricsList
	if err := json.Unmarshal(raw, &metrics); err != nil {
		return fmt.Errorf("decode pod metrics: %w", err)
	}

	index := make(map[string]*clustermode.Pod, len(pods))
	for position := range pods {
		index[pods[position].Namespace+"/"+pods[position].Name] = &pods[position]
	}
	for _, item := range metrics.Items {
		pod := index[item.Metadata.Namespace+"/"+item.Metadata.Name]
		if pod == nil {
			continue
		}
		usage := clustermode.ResourceValues{}
		for _, container := range item.Containers {
			if quantity, ok := container.Usage[corev1.ResourceCPU]; ok {
				usage.CPUMilli += quantity.MilliValue()
			}
			if quantity, ok := container.Usage[corev1.ResourceMemory]; ok {
				usage.MemoryBytes += quantity.Value()
			}
		}
		pod.Usage = &usage
	}
	return nil
}

// schedulingTaints keeps the taints that stop a pod without a toleration from
// being scheduled on the node.
func schedulingTaints(taints []corev1.Taint) []string {
	kept := make([]string, 0)
	for _, taint := range taints {
		if taint.Effect != corev1.TaintEffectNoSchedule && taint.Effect != corev1.TaintEffectNoExecute {
			continue
		}
		kept = append(kept, fmt.Sprintf("%s=%s:%s", taint.Key, taint.Value, taint.Effect))
	}
	return kept
}

func podClaims(spec *corev1.PodSpec) []string {
	claims := make([]string, 0)
	for _, volume := range spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			claims = append(claims, volume.PersistentVolumeClaim.ClaimName)
		}
	}
	return claims
}

// podOwner resolves the controller a pod belongs to. A Deployment's pods are
// owned by a ReplicaSet named after the Deployment plus the pod template hash,
// so the hash is stripped to reach the Deployment without another API call.
func podOwner(pod *corev1.Pod) (string, string) {
	owner := metav1.GetControllerOf(pod)
	if owner == nil {
		return "Pod", pod.Name
	}
	if owner.Kind == "ReplicaSet" {
		hash := pod.Labels[appsv1.DefaultDeploymentUniqueLabelKey]
		if hash != "" && strings.HasSuffix(owner.Name, "-"+hash) {
			return "Deployment", strings.TrimSuffix(owner.Name, "-"+hash)
		}
	}
	return owner.Kind, owner.Name
}

func collectNodes(nodes []corev1.Node, pods []corev1.Pod) []clustermode.Node {
	requestsByNode := make(map[string]clustermode.ResourceValues)
	for index := range pods {
		pod := &pods[index]
		if pod.Spec.NodeName == "" || pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		requests := podRequests(&pod.Spec)
		total := requestsByNode[pod.Spec.NodeName]
		addResources(&total, requests)
		requestsByNode[pod.Spec.NodeName] = total
	}

	result := make([]clustermode.Node, 0, len(nodes))
	for index := range nodes {
		node := &nodes[index]
		result = append(result, clustermode.Node{
			UID:          string(node.UID),
			Name:         node.Name,
			Ready:        nodeReady(node),
			Schedulable:  !node.Spec.Unschedulable,
			Role:         nodeRole(node.Labels),
			Zone:         node.Labels[corev1.LabelTopologyZone],
			Region:       node.Labels[corev1.LabelTopologyRegion],
			InstanceType: node.Labels[corev1.LabelInstanceTypeStable],
			ProviderID:   node.Spec.ProviderID,
			Capacity:     resourceListValues(node.Status.Capacity),
			Allocatable:  resourceListValues(node.Status.Allocatable),
			Requests:     requestsByNode[node.Name],
			Taints:       schedulingTaints(node.Spec.Taints),
		})
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Name < result[right].Name })
	return result
}

func deploymentWorkload(deployment *appsv1.Deployment) clustermode.Workload {
	replicas := int32(1)
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}
	return workloadFromTemplate(deployment.UID, "Deployment", deployment.Name, deployment.Namespace, replicas, deployment.Status.ReadyReplicas, &deployment.Spec.Template.Spec)
}

func statefulSetWorkload(statefulSet *appsv1.StatefulSet) clustermode.Workload {
	replicas := int32(1)
	if statefulSet.Spec.Replicas != nil {
		replicas = *statefulSet.Spec.Replicas
	}
	return workloadFromTemplate(statefulSet.UID, "StatefulSet", statefulSet.Name, statefulSet.Namespace, replicas, statefulSet.Status.ReadyReplicas, &statefulSet.Spec.Template.Spec)
}

func daemonSetWorkload(daemonSet *appsv1.DaemonSet) clustermode.Workload {
	return workloadFromTemplate(daemonSet.UID, "DaemonSet", daemonSet.Name, daemonSet.Namespace, daemonSet.Status.DesiredNumberScheduled, daemonSet.Status.NumberReady, &daemonSet.Spec.Template.Spec)
}

func workloadFromTemplate(uid types.UID, kind, name, namespace string, desired, ready int32, spec *corev1.PodSpec) clustermode.Workload {
	containers, perReplica, missing := containerRequests(spec)
	return clustermode.Workload{
		UID:              string(uid),
		Kind:             kind,
		Name:             name,
		Namespace:        namespace,
		DesiredReplicas:  desired,
		ReadyReplicas:    ready,
		Containers:       containers,
		Requests:         multiplyResources(perReplica, int64(desired)),
		MissingRequests:  len(missing) > 0,
		MissingResources: missing,
	}
}

// containerRequests sums the requests of the long-running containers and
// lists the resources ("cpu", "memory") that at least one of them leaves
// unrequested.
func containerRequests(spec *corev1.PodSpec) ([]clustermode.Container, clustermode.ResourceValues, []string) {
	containers := make([]clustermode.Container, 0, len(spec.Containers)+len(spec.InitContainers))
	total := clustermode.ResourceValues{}
	missingCPU, missingMemory := false, false
	appendContainer := func(container corev1.Container) {
		requests := resourceListValues(container.Resources.Requests)
		containers = append(containers, clustermode.Container{Name: container.Name, Requests: requests})
		addResources(&total, requests)
		missingCPU = missingCPU || requests.CPUMilli == 0
		missingMemory = missingMemory || requests.MemoryBytes == 0
	}
	for _, container := range spec.Containers {
		appendContainer(container)
	}
	for _, container := range spec.InitContainers {
		if container.RestartPolicy != nil && *container.RestartPolicy == corev1.ContainerRestartPolicyAlways {
			appendContainer(container)
		}
	}

	missing := make([]string, 0, 2)
	if missingCPU {
		missing = append(missing, "cpu")
	}
	if missingMemory {
		missing = append(missing, "memory")
	}
	return containers, total, missing
}

func podRequests(spec *corev1.PodSpec) clustermode.ResourceValues {
	_, total, _ := containerRequests(spec)
	for _, container := range spec.InitContainers {
		if container.RestartPolicy != nil && *container.RestartPolicy == corev1.ContainerRestartPolicyAlways {
			continue
		}
		requests := resourceListValues(container.Resources.Requests)
		total = maxResources(total, requests)
	}
	addResources(&total, resourceListValues(spec.Overhead))
	return total
}

func resourceListValues(resources corev1.ResourceList) clustermode.ResourceValues {
	values := clustermode.ResourceValues{}
	if quantity, ok := resources[corev1.ResourceCPU]; ok {
		values.CPUMilli = quantity.MilliValue()
	}
	if quantity, ok := resources[corev1.ResourceMemory]; ok {
		values.MemoryBytes = quantity.Value()
	}
	if quantity, ok := resources[corev1.ResourceEphemeralStorage]; ok {
		values.StorageBytes = quantity.Value()
	}
	if quantity, ok := resources[corev1.ResourceStorage]; ok {
		values.StorageBytes = quantity.Value()
	}
	if quantity, ok := resources[corev1.ResourceName("nvidia.com/gpu")]; ok {
		values.GPUUnits = quantity.Value()
	}
	return values
}

func nodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func nodeRole(labels map[string]string) string {
	const prefix = "node-role.kubernetes.io/"
	roles := make([]string, 0)
	for label := range labels {
		if strings.HasPrefix(label, prefix) {
			roles = append(roles, strings.TrimPrefix(label, prefix))
		}
	}
	if len(roles) == 0 {
		return "worker"
	}
	sort.Strings(roles)
	return strings.Join(roles, ", ")
}

func collectionWarning(resource string, err error) clustermode.Warning {
	return clustermode.Warning{Resource: resource, Message: fmt.Sprintf("could not list %s: %v", resource, err)}
}

func addResources(total *clustermode.ResourceValues, value clustermode.ResourceValues) {
	total.CPUMilli += value.CPUMilli
	total.MemoryBytes += value.MemoryBytes
	total.StorageBytes += value.StorageBytes
	total.GPUUnits += value.GPUUnits
}

func multiplyResources(value clustermode.ResourceValues, multiplier int64) clustermode.ResourceValues {
	return clustermode.ResourceValues{
		CPUMilli:     value.CPUMilli * multiplier,
		MemoryBytes:  value.MemoryBytes * multiplier,
		StorageBytes: value.StorageBytes * multiplier,
		GPUUnits:     value.GPUUnits * multiplier,
	}
}

func maxResources(left, right clustermode.ResourceValues) clustermode.ResourceValues {
	if right.CPUMilli > left.CPUMilli {
		left.CPUMilli = right.CPUMilli
	}
	if right.MemoryBytes > left.MemoryBytes {
		left.MemoryBytes = right.MemoryBytes
	}
	if right.StorageBytes > left.StorageBytes {
		left.StorageBytes = right.StorageBytes
	}
	if right.GPUUnits > left.GPUUnits {
		left.GPUUnits = right.GPUUnits
	}
	return left
}
