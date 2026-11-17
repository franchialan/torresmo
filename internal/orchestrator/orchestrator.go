package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sync/atomic"
	"time"

	v1 "github.com/torresmo-dev/torresmo/api/v1"
	"github.com/torresmo-dev/torresmo/internal/runtime"
	"github.com/torresmo-dev/torresmo/internal/storage"

	"gopkg.in/yaml.v3"
)

var nameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

var serviceIPCounter atomic.Int64

func init() {
	serviceIPCounter.Store(10)
}

// Orchestrator manages pods, deployments, services and namespaces within a cluster.
type Orchestrator struct {
	runtime   runtime.ContainerRuntime
	store     storage.Store
	scheduler *Scheduler
}

func New(rt runtime.ContainerRuntime, store storage.Store) *Orchestrator {
	return &Orchestrator{
		runtime:   rt,
		store:     store,
		scheduler: NewScheduler(store),
	}
}

// --- Namespace methods ---

func (o *Orchestrator) CreateNamespace(ctx context.Context, clusterName, name string) error {
	if !nameRe.MatchString(name) {
		return fmt.Errorf("invalid namespace name %q: must match [a-z0-9][a-z0-9-]{0,61}[a-z0-9]", name)
	}

	if _, err := o.store.GetNamespace(ctx, clusterName, name); err == nil {
		return fmt.Errorf("namespace %q already exists in cluster %q", name, clusterName)
	}

	now := time.Now().Unix()
	ns := &storage.NamespaceRecord{
		Name:        name,
		ClusterName: clusterName,
		Phase:       "Active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := o.store.SaveNamespace(ctx, ns); err != nil {
		return fmt.Errorf("saving namespace: %w", err)
	}

	o.emitEvent(ctx, clusterName, name, "Namespace", name, "Normal", "Created", fmt.Sprintf("Namespace %q created", name))
	return nil
}

func (o *Orchestrator) GetNamespace(ctx context.Context, clusterName, name string) (*storage.NamespaceRecord, error) {
	return o.store.GetNamespace(ctx, clusterName, name)
}

func (o *Orchestrator) ListNamespaces(ctx context.Context, clusterName string) ([]storage.NamespaceRecord, error) {
	return o.store.ListNamespaces(ctx, clusterName)
}

func (o *Orchestrator) DeleteNamespace(ctx context.Context, clusterName, name string) error {
	pods, err := o.store.ListPods(ctx, clusterName, name)
	if err != nil {
		return fmt.Errorf("checking pods in namespace: %w", err)
	}
	if len(pods) > 0 {
		return fmt.Errorf("cannot delete namespace %q: %d pods still exist", name, len(pods))
	}

	if err := o.store.DeleteNamespace(ctx, clusterName, name); err != nil {
		return fmt.Errorf("deleting namespace: %w", err)
	}

	o.emitEvent(ctx, clusterName, name, "Namespace", name, "Normal", "Deleted", fmt.Sprintf("Namespace %q deleted", name))
	return nil
}

// ensureNamespace creates the given namespace if it does not exist.
func (o *Orchestrator) ensureNamespace(ctx context.Context, clusterName, namespace string) error {
	if _, err := o.store.GetNamespace(ctx, clusterName, namespace); err == nil {
		return nil
	}
	return o.CreateNamespace(ctx, clusterName, namespace)
}

// --- Pod methods ---

func (o *Orchestrator) CreatePod(ctx context.Context, clusterName string, pod *v1.Pod) error {
	namespace := pod.Namespace
	if namespace == "" {
		namespace = "default"
	}

	if err := o.ensureNamespace(ctx, clusterName, namespace); err != nil {
		return fmt.Errorf("ensuring namespace: %w", err)
	}

	node, err := o.scheduler.PickNode(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("scheduling pod: %w", err)
	}

	if len(pod.Spec.Containers) == 0 {
		return fmt.Errorf("pod %q must have at least one container", pod.Name)
	}
	container := pod.Spec.Containers[0]

	containerName := fmt.Sprintf("torresmo-%s-%s-%s", clusterName, namespace, pod.Name)

	var envVars []string
	for _, e := range container.Env {
		envVars = append(envVars, e.Name+"="+e.Value)
	}

	var portMappings []runtime.PortMapping
	for _, p := range container.Ports {
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		portMappings = append(portMappings, runtime.PortMapping{
			ContainerPort: p.ContainerPort,
			Protocol:      proto,
		})
	}

	labels := map[string]string{
		"torresmo.dev/managed":   "true",
		"torresmo.dev/cluster":   clusterName,
		"torresmo.dev/namespace": namespace,
		"torresmo.dev/pod":       pod.Name,
	}
	for k, v := range pod.Labels {
		labels[k] = v
	}

	networkName := "torresmo-" + clusterName

	// Ensure the container image is available locally.
	if exists, _ := o.runtime.ImageExists(ctx, container.Image); !exists {
		if err := o.runtime.PullImage(ctx, container.Image); err != nil {
			return fmt.Errorf("pulling image %q for pod %q: %w", container.Image, pod.Name, err)
		}
	}

	cID, err := o.runtime.CreateContainer(ctx, &runtime.ContainerConfig{
		Name:     containerName,
		Image:    container.Image,
		Command:  container.Command,
		Env:      envVars,
		Labels:   labels,
		Ports:    portMappings,
		Networks: []string{networkName},
	})
	if err != nil {
		return fmt.Errorf("creating container for pod %q: %w", pod.Name, err)
	}

	if err := o.runtime.StartContainer(ctx, cID); err != nil {
		return fmt.Errorf("starting container for pod %q: %w", pod.Name, err)
	}

	var podIP string
	detail, err := o.runtime.InspectContainer(ctx, cID)
	if err == nil {
		podIP = detail.NetworkSettings.IPAddress
		if podIP == "" {
			if ep, ok := detail.NetworkSettings.Networks[networkName]; ok {
				podIP = ep.IPAddress
			}
		}
	}

	cmdJSON, _ := json.Marshal(container.Command)
	labelsJSON, _ := json.Marshal(pod.Labels)
	envJSON, _ := json.Marshal(container.Env)
	portsJSON, _ := json.Marshal(container.Ports)

	now := time.Now().Unix()
	podRec := &storage.PodRecord{
		Name:        pod.Name,
		ClusterName: clusterName,
		Namespace:   namespace,
		ContainerID: cID,
		Image:       container.Image,
		Command:     string(cmdJSON),
		Phase:       "Running",
		Node:        node.Name,
		PodIP:       podIP,
		Restarts:    0,
		Labels:      string(labelsJSON),
		Env:         string(envJSON),
		Ports:       string(portsJSON),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := o.store.SavePod(ctx, podRec); err != nil {
		return fmt.Errorf("saving pod record: %w", err)
	}

	o.emitEvent(ctx, clusterName, namespace, "Pod", pod.Name, "Normal", "Scheduled", fmt.Sprintf("Pod scheduled on node %s", node.Name))
	o.emitEvent(ctx, clusterName, namespace, "Pod", pod.Name, "Normal", "Started", "Pod started")
	return nil
}

func (o *Orchestrator) GetPod(ctx context.Context, clusterName, namespace, name string) (*storage.PodRecord, error) {
	return o.store.GetPod(ctx, clusterName, namespace, name)
}

func (o *Orchestrator) ListPods(ctx context.Context, clusterName, namespace string) ([]storage.PodRecord, error) {
	if namespace == "" {
		return o.store.ListAllPods(ctx, clusterName)
	}
	return o.store.ListPods(ctx, clusterName, namespace)
}

func (o *Orchestrator) DeletePod(ctx context.Context, clusterName, namespace, name string) error {
	pod, err := o.store.GetPod(ctx, clusterName, namespace, name)
	if err != nil {
		return fmt.Errorf("pod %q not found: %w", name, err)
	}

	if pod.ContainerID != "" {
		_ = o.runtime.StopContainer(ctx, pod.ContainerID, 10)
		_ = o.runtime.RemoveContainer(ctx, pod.ContainerID, true)
	}

	if err := o.store.DeletePod(ctx, clusterName, namespace, name); err != nil {
		return fmt.Errorf("deleting pod record: %w", err)
	}

	o.emitEvent(ctx, clusterName, namespace, "Pod", name, "Normal", "Deleted", fmt.Sprintf("Pod %q deleted", name))
	return nil
}

// --- Deployment methods ---

func (o *Orchestrator) ApplyDeployment(ctx context.Context, clusterName string, dep *v1.Deployment) error {
	namespace := dep.Namespace
	if namespace == "" {
		namespace = "default"
	}

	if err := o.ensureNamespace(ctx, clusterName, namespace); err != nil {
		return fmt.Errorf("ensuring namespace: %w", err)
	}

	templateJSON, _ := json.Marshal(dep.Spec.Template.Spec)
	labelsJSON, _ := json.Marshal(dep.Labels)

	selector := dep.Spec.Template.Labels
	if selector == nil {
		selector = map[string]string{}
	}
	selectorJSON, _ := json.Marshal(selector)

	replicas := dep.Spec.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	now := time.Now().Unix()
	existing, err := o.store.GetDeployment(ctx, clusterName, namespace, dep.Name)
	if err == nil {
		existing.Replicas = replicas
		existing.Template = string(templateJSON)
		existing.Labels = string(labelsJSON)
		existing.Selector = string(selectorJSON)
		existing.UpdatedAt = now

		if err := o.store.SaveDeployment(ctx, existing); err != nil {
			return fmt.Errorf("updating deployment: %w", err)
		}

		if err := o.reconcileDeploymentPods(ctx, clusterName, namespace, dep.Name, replicas, dep.Spec.Template.Spec, selector); err != nil {
			return fmt.Errorf("reconciling pods: %w", err)
		}

		o.emitEvent(ctx, clusterName, namespace, "Deployment", dep.Name, "Normal", "Updated", fmt.Sprintf("Deployment %q updated to %d replicas", dep.Name, replicas))
		return nil
	}

	depRec := &storage.DeploymentRecord{
		Name:          dep.Name,
		ClusterName:   clusterName,
		Namespace:     namespace,
		Replicas:      replicas,
		ReadyReplicas: 0,
		Template:      string(templateJSON),
		Labels:        string(labelsJSON),
		Selector:      string(selectorJSON),
		Phase:         "Progressing",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := o.store.SaveDeployment(ctx, depRec); err != nil {
		return fmt.Errorf("saving deployment: %w", err)
	}

	readyCount := 0
	for i := 0; i < replicas; i++ {
		podName := dep.Name + "-" + randomSuffix()
		pod := &v1.Pod{
			TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Pod"},
			ObjectMeta: v1.ObjectMeta{
				Name:      podName,
				Namespace: namespace,
				Labels:    mergeLabels(selector, map[string]string{"torresmo.dev/deployment": dep.Name}),
			},
			Spec: dep.Spec.Template.Spec,
		}
		if err := o.CreatePod(ctx, clusterName, pod); err != nil {
			o.emitEvent(ctx, clusterName, namespace, "Deployment", dep.Name, "Warning", "PodCreateFailed", fmt.Sprintf("Failed to create pod %q: %v", podName, err))
			continue
		}
		readyCount++
	}

	depRec.ReadyReplicas = readyCount
	if readyCount == replicas {
		depRec.Phase = "Available"
	}
	depRec.UpdatedAt = time.Now().Unix()
	if err := o.store.SaveDeployment(ctx, depRec); err != nil {
		return fmt.Errorf("updating deployment status: %w", err)
	}

	o.emitEvent(ctx, clusterName, namespace, "Deployment", dep.Name, "Normal", "Created", fmt.Sprintf("Deployment %q created with %d/%d replicas ready", dep.Name, readyCount, replicas))
	return nil
}

func (o *Orchestrator) reconcileDeploymentPods(ctx context.Context, clusterName, namespace, depName string, desiredReplicas int, spec v1.PodSpec, selector map[string]string) error {
	allPods, err := o.store.ListPods(ctx, clusterName, namespace)
	if err != nil {
		return err
	}

	var depPods []storage.PodRecord
	for _, p := range allPods {
		var podLabels map[string]string
		_ = json.Unmarshal([]byte(p.Labels), &podLabels)
		if podLabels["torresmo.dev/deployment"] == depName {
			depPods = append(depPods, p)
		}
	}

	current := len(depPods)

	if current < desiredReplicas {
		for i := 0; i < desiredReplicas-current; i++ {
			podName := depName + "-" + randomSuffix()
			pod := &v1.Pod{
				TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Pod"},
				ObjectMeta: v1.ObjectMeta{
					Name:      podName,
					Namespace: namespace,
					Labels:    mergeLabels(selector, map[string]string{"torresmo.dev/deployment": depName}),
				},
				Spec: spec,
			}
			_ = o.CreatePod(ctx, clusterName, pod)
		}
	} else if current > desiredReplicas {
		toRemove := current - desiredReplicas
		for i := 0; i < toRemove && i < len(depPods); i++ {
			_ = o.DeletePod(ctx, clusterName, namespace, depPods[i].Name)
		}
	}

	// Update ready count.
	updatedPods, _ := o.store.ListPods(ctx, clusterName, namespace)
	readyCount := 0
	for _, p := range updatedPods {
		var podLabels map[string]string
		_ = json.Unmarshal([]byte(p.Labels), &podLabels)
		if podLabels["torresmo.dev/deployment"] == depName && p.Phase == "Running" {
			readyCount++
		}
	}

	dep, err := o.store.GetDeployment(ctx, clusterName, namespace, depName)
	if err == nil {
		dep.ReadyReplicas = readyCount
		if readyCount >= desiredReplicas {
			dep.Phase = "Available"
		} else {
			dep.Phase = "Progressing"
		}
		dep.UpdatedAt = time.Now().Unix()
		_ = o.store.SaveDeployment(ctx, dep)
	}

	return nil
}

func (o *Orchestrator) ListDeployments(ctx context.Context, clusterName, namespace string) ([]storage.DeploymentRecord, error) {
	return o.store.ListDeployments(ctx, clusterName, namespace)
}

func (o *Orchestrator) DeleteDeployment(ctx context.Context, clusterName, namespace, name string) error {
	allPods, err := o.store.ListPods(ctx, clusterName, namespace)
	if err != nil {
		return fmt.Errorf("listing pods for deployment: %w", err)
	}

	for _, p := range allPods {
		var podLabels map[string]string
		_ = json.Unmarshal([]byte(p.Labels), &podLabels)
		if podLabels["torresmo.dev/deployment"] == name {
			_ = o.DeletePod(ctx, clusterName, namespace, p.Name)
		}
	}

	if err := o.store.DeleteDeployment(ctx, clusterName, namespace, name); err != nil {
		return fmt.Errorf("deleting deployment record: %w", err)
	}

	o.emitEvent(ctx, clusterName, namespace, "Deployment", name, "Normal", "Deleted", fmt.Sprintf("Deployment %q deleted", name))
	return nil
}

// --- Service methods ---

func (o *Orchestrator) ApplyService(ctx context.Context, clusterName string, svc *v1.Service) error {
	namespace := svc.Namespace
	if namespace == "" {
		namespace = "default"
	}

	if err := o.ensureNamespace(ctx, clusterName, namespace); err != nil {
		return fmt.Errorf("ensuring namespace: %w", err)
	}

	portsJSON, _ := json.Marshal(svc.Spec.Ports)
	selectorJSON, _ := json.Marshal(svc.Spec.Selector)

	svcType := svc.Spec.Type
	if svcType == "" {
		svcType = "ClusterIP"
	}

	clusterIP := fmt.Sprintf("10.244.0.%d", serviceIPCounter.Add(1))

	// If updating, preserve existing cluster IP.
	existing, err := o.store.GetService(ctx, clusterName, namespace, svc.Name)
	if err == nil && existing.ClusterIP != "" {
		clusterIP = existing.ClusterIP
	}

	now := time.Now().Unix()
	var createdAt int64
	if existing != nil {
		createdAt = existing.CreatedAt
	} else {
		createdAt = now
	}

	svcRec := &storage.ServiceRecord{
		Name:        svc.Name,
		ClusterName: clusterName,
		Namespace:   namespace,
		Type:        svcType,
		Ports:       string(portsJSON),
		Selector:    string(selectorJSON),
		ClusterIP:   clusterIP,
		CreatedAt:   createdAt,
		UpdatedAt:   now,
	}
	if err := o.store.SaveService(ctx, svcRec); err != nil {
		return fmt.Errorf("saving service: %w", err)
	}

	o.emitEvent(ctx, clusterName, namespace, "Service", svc.Name, "Normal", "Applied", fmt.Sprintf("Service %q applied with ClusterIP %s", svc.Name, clusterIP))
	return nil
}

func (o *Orchestrator) ListServices(ctx context.Context, clusterName, namespace string) ([]storage.ServiceRecord, error) {
	return o.store.ListServices(ctx, clusterName, namespace)
}

func (o *Orchestrator) DeleteService(ctx context.Context, clusterName, namespace, name string) error {
	if err := o.store.DeleteService(ctx, clusterName, namespace, name); err != nil {
		return fmt.Errorf("deleting service record: %w", err)
	}

	o.emitEvent(ctx, clusterName, namespace, "Service", name, "Normal", "Deleted", fmt.Sprintf("Service %q deleted", name))
	return nil
}

// --- Event methods ---

func (o *Orchestrator) ListEvents(ctx context.Context, clusterName, namespace string, limit int) ([]storage.EventRecord, error) {
	return o.store.ListEvents(ctx, clusterName, namespace, limit)
}

func (o *Orchestrator) ListEventsByResource(ctx context.Context, clusterName, kind, name string, limit int) ([]storage.EventRecord, error) {
	return o.store.ListEventsByResource(ctx, clusterName, kind, name, limit)
}

func (o *Orchestrator) emitEvent(ctx context.Context, clusterName, namespace, kind, name, eventType, reason, message string) {
	event := &storage.EventRecord{
		ClusterName: clusterName,
		Namespace:   namespace,
		Kind:        kind,
		Name:        name,
		Type:        eventType,
		Reason:      reason,
		Message:     message,
		CreatedAt:   time.Now().Unix(),
	}
	_ = o.store.SaveEvent(ctx, event)
}

// --- Manifest apply ---

func (o *Orchestrator) ApplyManifest(ctx context.Context, clusterName string, data []byte) error {
	var meta v1.TypeMeta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return fmt.Errorf("parsing manifest metadata: %w", err)
	}

	if meta.APIVersion != "torresmo.io/v1" {
		return fmt.Errorf("unsupported apiVersion %q, expected torresmo.io/v1", meta.APIVersion)
	}

	switch meta.Kind {
	case "Pod":
		var pod v1.Pod
		if err := yaml.Unmarshal(data, &pod); err != nil {
			return fmt.Errorf("parsing Pod manifest: %w", err)
		}
		return o.CreatePod(ctx, clusterName, &pod)

	case "Deployment":
		var dep v1.Deployment
		if err := yaml.Unmarshal(data, &dep); err != nil {
			return fmt.Errorf("parsing Deployment manifest: %w", err)
		}
		return o.ApplyDeployment(ctx, clusterName, &dep)

	case "Service":
		var svc v1.Service
		if err := yaml.Unmarshal(data, &svc); err != nil {
			return fmt.Errorf("parsing Service manifest: %w", err)
		}
		return o.ApplyService(ctx, clusterName, &svc)

	case "Namespace":
		var ns v1.Namespace
		if err := yaml.Unmarshal(data, &ns); err != nil {
			return fmt.Errorf("parsing Namespace manifest: %w", err)
		}
		return o.CreateNamespace(ctx, clusterName, ns.Name)

	default:
		return fmt.Errorf("unsupported kind %q", meta.Kind)
	}
}

func (o *Orchestrator) ApplyManifestFile(ctx context.Context, clusterName, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("reading path %q: %w", path, err)
	}

	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return fmt.Errorf("reading directory %q: %w", path, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := filepath.Ext(entry.Name())
			if ext != ".yaml" && ext != ".yml" {
				continue
			}
			if err := o.ApplyManifestFile(ctx, clusterName, filepath.Join(path, entry.Name())); err != nil {
				return err
			}
		}
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file %q: %w", path, err)
	}

	docs := splitYAMLDocuments(data)
	for _, doc := range docs {
		doc = bytes.TrimSpace(doc)
		if len(doc) == 0 {
			continue
		}
		if err := o.ApplyManifest(ctx, clusterName, doc); err != nil {
			return fmt.Errorf("applying document from %q: %w", path, err)
		}
	}
	return nil
}

func splitYAMLDocuments(data []byte) [][]byte {
	var docs [][]byte
	for _, part := range bytes.Split(data, []byte("\n---")) {
		part = bytes.TrimSpace(part)
		if len(part) > 0 {
			docs = append(docs, part)
		}
	}
	return docs
}

// --- Helpers ---

const suffixChars = "abcdefghijklmnopqrstuvwxyz0123456789"

func randomSuffix() string {
	b := make([]byte, 5)
	for i := range b {
		b[i] = suffixChars[rand.Intn(len(suffixChars))]
	}
	return string(b)
}

func mergeLabels(base, extra map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(extra))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range extra {
		merged[k] = v
	}
	return merged
}
