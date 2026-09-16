package kubernetes

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	clustermode "kube-budget/internal/application/cluster"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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

func (collector *Collector) Collect(ctx context.Context) (clustermode.CollectedData, error) {
	data := clustermode.CollectedData{
		Cluster:            collector.cluster,
		Nodes:              make([]clustermode.Node, 0),
		Workloads:          make([]clustermode.Workload, 0),
		Resources:          make([]clustermode.Resource, 0),
		NamespacePodCounts: make(map[string]int),
		Warnings:           make([]clustermode.Warning, 0),
	}

	pods, err := collector.client.CoreV1().Pods(collector.namespace).List(ctx, metav1.ListOptions{})
	podItems := make([]corev1.Pod, 0)
	if err != nil {
		data.Warnings = append(data.Warnings, collectionWarning("pods", err))
	} else {
		podItems = pods.Items
		collectPods(podItems, &data)
	}

	nodes, err := collector.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		data.Warnings = append(data.Warnings, collectionWarning("nodes", err))
	} else {
		data.Nodes = collectNodes(nodes.Items, podItems)
	}

	deployments, err := collector.client.AppsV1().Deployments(collector.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		data.Warnings = append(data.Warnings, collectionWarning("deployments", err))
	} else {
		for index := range deployments.Items {
			data.Workloads = append(data.Workloads, deploymentWorkload(&deployments.Items[index]))
		}
	}

	statefulSets, err := collector.client.AppsV1().StatefulSets(collector.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		data.Warnings = append(data.Warnings, collectionWarning("statefulsets", err))
	} else {
		for index := range statefulSets.Items {
			data.Workloads = append(data.Workloads, statefulSetWorkload(&statefulSets.Items[index]))
		}
	}

	daemonSets, err := collector.client.AppsV1().DaemonSets(collector.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		data.Warnings = append(data.Warnings, collectionWarning("daemonsets", err))
	} else {
		for index := range daemonSets.Items {
			data.Workloads = append(data.Workloads, daemonSetWorkload(&daemonSets.Items[index]))
		}
	}

	collector.collectBatchResources(ctx, &data)
	collector.collectCoreResources(ctx, &data)

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

func (collector *Collector) collectBatchResources(ctx context.Context, data *clustermode.CollectedData) {
	jobs, err := collector.client.BatchV1().Jobs(collector.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		data.Warnings = append(data.Warnings, collectionWarning("jobs", err))
	} else {
		for index := range jobs.Items {
			data.Resources = append(data.Resources, jobResource(&jobs.Items[index]))
		}
	}

	cronJobs, err := collector.client.BatchV1().CronJobs(collector.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		data.Warnings = append(data.Warnings, collectionWarning("cronjobs", err))
	} else {
		for index := range cronJobs.Items {
			data.Resources = append(data.Resources, cronJobResource(&cronJobs.Items[index]))
		}
	}
}

func (collector *Collector) collectCoreResources(ctx context.Context, data *clustermode.CollectedData) {
	configMaps, err := collector.client.CoreV1().ConfigMaps(collector.namespace).List(ctx, metav1.ListOptions{})
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

	secrets, err := collector.client.CoreV1().Secrets(collector.namespace).List(ctx, metav1.ListOptions{})
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

	services, err := collector.client.CoreV1().Services(collector.namespace).List(ctx, metav1.ListOptions{})
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

	claims, err := collector.client.CoreV1().PersistentVolumeClaims(collector.namespace).List(ctx, metav1.ListOptions{})
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

	quotas, err := collector.client.CoreV1().ResourceQuotas(collector.namespace).List(ctx, metav1.ListOptions{})
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

	limitRanges, err := collector.client.CoreV1().LimitRanges(collector.namespace).List(ctx, metav1.ListOptions{})
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
	}
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
		UID:             string(uid),
		Kind:            kind,
		Name:            name,
		Namespace:       namespace,
		DesiredReplicas: desired,
		ReadyReplicas:   ready,
		Containers:      containers,
		Requests:        multiplyResources(perReplica, int64(desired)),
		MissingRequests: missing,
	}
}

func containerRequests(spec *corev1.PodSpec) ([]clustermode.Container, clustermode.ResourceValues, bool) {
	containers := make([]clustermode.Container, 0, len(spec.Containers)+len(spec.InitContainers))
	total := clustermode.ResourceValues{}
	missing := false
	appendContainer := func(container corev1.Container) {
		requests := resourceListValues(container.Resources.Requests)
		containers = append(containers, clustermode.Container{Name: container.Name, Requests: requests})
		addResources(&total, requests)
		if requests.CPUMilli == 0 || requests.MemoryBytes == 0 {
			missing = true
		}
	}
	for _, container := range spec.Containers {
		appendContainer(container)
	}
	for _, container := range spec.InitContainers {
		if container.RestartPolicy != nil && *container.RestartPolicy == corev1.ContainerRestartPolicyAlways {
			appendContainer(container)
		}
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
