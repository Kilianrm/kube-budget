package kubernetes

import (
	"context"
	"testing"

	clustermode "kube-budget/internal/application/cluster"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCollectBuildsLiveInventory(t *testing.T) {
	replicas := int32(3)
	objects := []runtime.Object{
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{UID: types.UID("node-uid"), Name: "worker-1", Labels: map[string]string{
				corev1.LabelTopologyZone: "eu-west-1a", corev1.LabelTopologyRegion: "eu-west-1", corev1.LabelInstanceTypeStable: "m6i.large",
			}},
			Status: corev1.NodeStatus{
				Conditions:  []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
				Allocatable: resources("4", "8Gi"), Capacity: resources("4", "8Gi"),
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "api-abc", Namespace: "production"},
			Spec: corev1.PodSpec{NodeName: "worker-1", Containers: []corev1.Container{{
				Name: "api", Resources: corev1.ResourceRequirements{Requests: resources("500m", "512Mi")},
			}}},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{UID: types.UID("deployment-uid"), Name: "api", Namespace: "production"},
			Spec: appsv1.DeploymentSpec{Replicas: &replicas, Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name: "api", Resources: corev1.ResourceRequirements{Requests: resources("500m", "512Mi")},
			}}}}},
			Status: appsv1.DeploymentStatus{ReadyReplicas: 2},
		},
	}

	collector := NewCollector(fake.NewSimpleClientset(objects...), clustermode.ClusterInfo{Context: "kind-budget", Version: "v1.31.0"}, "")
	data, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if len(data.Warnings) != 0 {
		t.Fatalf("Collect() warnings = %#v, want none", data.Warnings)
	}
	if len(data.Nodes) != 1 || !data.Nodes[0].Ready || !data.Nodes[0].Schedulable || data.Nodes[0].Requests.CPUMilli != 500 {
		t.Errorf("Collect() nodes = %#v", data.Nodes)
	}
	if data.NamespacePodCounts["production"] != 1 {
		t.Errorf("Collect() pod count = %#v", data.NamespacePodCounts)
	}
	if len(data.Workloads) != 1 {
		t.Fatalf("Collect() workloads = %#v", data.Workloads)
	}
	workload := data.Workloads[0]
	if workload.Kind != "Deployment" || workload.DesiredReplicas != 3 || workload.ReadyReplicas != 2 {
		t.Errorf("Collect() workload status = %#v", workload)
	}
	if workload.Requests.CPUMilli != 1500 || workload.Requests.MemoryBytes != 3*(512<<20) || workload.MissingRequests {
		t.Errorf("Collect() workload requests = %#v", workload)
	}
}

func TestCollectMarksCordonedNodeUnschedulable(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-1"},
		Spec:       corev1.NodeSpec{Unschedulable: true},
		Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{
			Type: corev1.NodeReady, Status: corev1.ConditionTrue,
		}}},
	}

	data, err := NewCollector(fake.NewSimpleClientset(node), clustermode.ClusterInfo{}, "").Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if len(data.Nodes) != 1 || !data.Nodes[0].Ready || data.Nodes[0].Schedulable {
		t.Errorf("Collect() cordoned node = %#v, want Ready and not Schedulable", data.Nodes)
	}
}

func TestCollectBuildsSupportingResourceInventory(t *testing.T) {
	suspended := false
	storageClass := "standard"
	objects := []runtime.Object{
		&batchv1.Job{ObjectMeta: metav1.ObjectMeta{UID: "job-uid", Name: "demo-job", Namespace: "monitoring-demo"}, Status: batchv1.JobStatus{Succeeded: 1, CompletionTime: &metav1.Time{}}},
		&batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{UID: "cron-uid", Name: "demo-cronjob", Namespace: "monitoring-demo"}, Spec: batchv1.CronJobSpec{Schedule: "*/5 * * * *", Suspend: &suspended}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{UID: "config-uid", Name: "demo-config", Namespace: "monitoring-demo"}, Data: map[string]string{"ENV": "local"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{UID: "secret-uid", Name: "demo-secret", Namespace: "monitoring-demo"}, Type: corev1.SecretTypeOpaque, Data: map[string][]byte{"password": []byte("not-returned")}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{UID: "service-uid", Name: "demo-service", Namespace: "monitoring-demo"}, Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, ClusterIP: "10.96.0.10", Ports: []corev1.ServicePort{{Port: 80}}}},
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{UID: "pvc-uid", Name: "demo-storage", Namespace: "monitoring-demo"}, Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: &storageClass, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}}}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}},
		&corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{UID: "quota-uid", Name: "demo-quota", Namespace: "monitoring-demo"}, Spec: corev1.ResourceQuotaSpec{Hard: corev1.ResourceList{corev1.ResourcePods: resource.MustParse("20")}}},
		&corev1.LimitRange{ObjectMeta: metav1.ObjectMeta{UID: "limit-uid", Name: "demo-limits", Namespace: "monitoring-demo"}, Spec: corev1.LimitRangeSpec{Limits: []corev1.LimitRangeItem{{Type: corev1.LimitTypeContainer}}}},
	}

	data, err := NewCollector(fake.NewSimpleClientset(objects...), clustermode.ClusterInfo{}, "monitoring-demo").Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if len(data.Resources) != 8 {
		t.Fatalf("Collect() resources = %#v, want 8 kinds", data.Resources)
	}
	kinds := make(map[string]clustermode.Resource, len(data.Resources))
	for _, item := range data.Resources {
		kinds[item.Kind] = item
	}
	for _, kind := range []string{"Job", "CronJob", "ConfigMap", "Secret", "Service", "PersistentVolumeClaim", "ResourceQuota", "LimitRange"} {
		if _, ok := kinds[kind]; !ok {
			t.Errorf("Collect() resources missing %s", kind)
		}
	}
	if kinds["Job"].Status != "Completed" || kinds["CronJob"].Attributes["Schedule"] != "*/5 * * * *" {
		t.Errorf("Collect() batch resources = job %#v, cronjob %#v", kinds["Job"], kinds["CronJob"])
	}
	secret := kinds["Secret"]
	if secret.Attributes["Keys"] != "1" || secret.Attributes["Type"] != string(corev1.SecretTypeOpaque) || len(secret.Attributes) != 2 {
		t.Errorf("Collect() secret metadata = %#v, want only key count and type", secret.Attributes)
	}
	if kinds["PersistentVolumeClaim"].Requests.StorageBytes != 1<<30 {
		t.Errorf("Collect() PVC requests = %#v, want 1 GiB", kinds["PersistentVolumeClaim"].Requests)
	}
}

func resources(cpu, memory string) corev1.ResourceList {
	return corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse(cpu),
		corev1.ResourceMemory: resource.MustParse(memory),
	}
}

func TestPodOwnerResolvesDeploymentsThroughReplicaSets(t *testing.T) {
	controller := true
	owned := func(kind, name string, labels map[string]string) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name:            "pod-1",
			Labels:          labels,
			OwnerReferences: []metav1.OwnerReference{{Kind: kind, Name: name, Controller: &controller}},
		}}
	}

	cases := []struct {
		name      string
		pod       *corev1.Pod
		wantKind  string
		wantOwner string
	}{
		{"deployment", owned("ReplicaSet", "api-7d9f8b", map[string]string{appsv1.DefaultDeploymentUniqueLabelKey: "7d9f8b"}), "Deployment", "api"},
		{"bare replicaset", owned("ReplicaSet", "legacy", nil), "ReplicaSet", "legacy"},
		{"job", owned("Job", "backup-2890", nil), "Job", "backup-2890"},
		{"bare pod", &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "debug"}}, "Pod", "debug"},
	}
	for _, tc := range cases {
		kind, owner := podOwner(tc.pod)
		if kind != tc.wantKind || owner != tc.wantOwner {
			t.Errorf("%s: podOwner() = %s/%s, want %s/%s", tc.name, kind, owner, tc.wantKind, tc.wantOwner)
		}
	}
}

func TestCollectPodsSkipsFinishedPodsAndKeepsPlacement(t *testing.T) {
	data := clustermode.CollectedData{NamespacePodCounts: map[string]int{}}
	collectPods([]corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: "prod"}, Spec: corev1.PodSpec{NodeName: "worker-1", Containers: []corev1.Container{{
			Name: "api", Resources: corev1.ResourceRequirements{Requests: resources("250m", "256Mi")},
		}}}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
		{ObjectMeta: metav1.ObjectMeta{Name: "done", Namespace: "prod"}, Status: corev1.PodStatus{Phase: corev1.PodSucceeded}},
	}, &data)

	if len(data.Pods) != 1 {
		t.Fatalf("Pods = %#v, want only the running pod", data.Pods)
	}
	pod := data.Pods[0]
	if pod.NodeName != "worker-1" || pod.Requests.CPUMilli != 250 || pod.OwnerKind != "Pod" || pod.MissingRequests {
		t.Errorf("Pod = %#v", pod)
	}
}

func TestApplyPodMetricsSumsContainerUsage(t *testing.T) {
	pods := []clustermode.Pod{{Name: "api-1", Namespace: "prod"}, {Name: "fresh", Namespace: "prod"}}
	raw := []byte(`{"kind":"PodMetricsList","items":[
		{"metadata":{"name":"api-1","namespace":"prod"},"containers":[
			{"name":"api","usage":{"cpu":"12500000n","memory":"100Mi"}},
			{"name":"proxy","usage":{"cpu":"3m","memory":"28Mi"}}]},
		{"metadata":{"name":"gone","namespace":"prod"},"containers":[{"name":"x","usage":{"cpu":"1","memory":"1Gi"}}]}]}`)

	if err := applyPodMetrics(raw, pods); err != nil {
		t.Fatalf("applyPodMetrics() error = %v", err)
	}
	if pods[0].Usage == nil || pods[0].Usage.CPUMilli != 16 || pods[0].Usage.MemoryBytes != 128*1024*1024 {
		t.Errorf("api-1 usage = %+v, want 16m and 128Mi", pods[0].Usage)
	}
	if pods[1].Usage != nil {
		t.Errorf("fresh usage = %+v, want nil when the pod was not sampled", pods[1].Usage)
	}
}

func TestPodClaimsListsMountedClaimsOnly(t *testing.T) {
	spec := &corev1.PodSpec{Volumes: []corev1.Volume{
		{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "postgres-data"}}},
		{Name: "cache", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
	}}

	if claims := podClaims(spec); len(claims) != 1 || claims[0] != "postgres-data" {
		t.Errorf("podClaims() = %v, want [postgres-data]", claims)
	}
}

func TestContainerRequestsListsUnrequestedResources(t *testing.T) {
	spec := &corev1.PodSpec{Containers: []corev1.Container{
		{Name: "aws-node", Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("25m")}}},
		{Name: "agent", Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("25m")}}},
	}}

	_, total, missing := containerRequests(spec)

	if total.CPUMilli != 50 || len(missing) != 1 || missing[0] != "memory" {
		t.Errorf("containerRequests() = %d mCPU, missing %v, want 50 and [memory]", total.CPUMilli, missing)
	}
}
