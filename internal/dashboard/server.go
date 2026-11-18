package dashboard

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/torresmo-dev/torresmo/internal/cluster"
	"github.com/torresmo-dev/torresmo/internal/orchestrator"
	"github.com/torresmo-dev/torresmo/internal/runtime"
	"github.com/torresmo-dev/torresmo/internal/storage"
)

//go:embed static/*
var staticFiles embed.FS

// Server serves the web dashboard.
type Server struct {
	clusterMgr  *cluster.Manager
	orch        *orchestrator.Orchestrator
	store       storage.Store
	runtime     runtime.ContainerRuntime
	cluster     string
	addr        string
	liveUpdates chan []byte
	liveClients []chan []byte
	liveMu      sync.Mutex
}

func NewServer(mgr *cluster.Manager, orch *orchestrator.Orchestrator, store storage.Store, rt runtime.ContainerRuntime, clusterName, addr string) *Server {
	return &Server{
		clusterMgr: mgr,
		orch:       orch,
		store:      store,
		runtime:    rt,
		cluster:    clusterName,
		addr:       addr,
	}
}

// SetLiveChannel sets the channel for receiving live pipeline updates.
// Each message should be a JSON-encoded byte slice. The server fans out
// every received message to all connected SSE clients.
func (s *Server) SetLiveChannel(ch chan []byte) {
	s.liveUpdates = ch
	go s.fanoutUpdates()
}

func (s *Server) fanoutUpdates() {
	for msg := range s.liveUpdates {
		s.liveMu.Lock()
		for _, client := range s.liveClients {
			select {
			case client <- msg:
			default:
			}
		}
		s.liveMu.Unlock()
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/cluster", s.handleCluster)
	mux.HandleFunc("/api/nodes", s.handleNodes)
	mux.HandleFunc("/api/pods", s.handlePods)
	mux.HandleFunc("/api/pods/", s.handlePodRoute)
	mux.HandleFunc("/api/services", s.handleServices)
	mux.HandleFunc("/api/deployments", s.handleDeployments)
	mux.HandleFunc("/api/namespaces", s.handleNamespaces)
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/api/addons", s.handleAddons)
	mux.HandleFunc("/api/pipelines/live", s.handlePipelineLive)
	mux.HandleFunc("/api/pipelines", s.handlePipelines)
	mux.HandleFunc("/api/pipelines/", s.handlePipelineTargets)
	mux.HandleFunc("/api/workflows", s.handleWorkflows)
	mux.HandleFunc("/api/workflows/", s.handleWorkflowLogs)
	mux.HandleFunc("/api/overview", s.handleOverview)

	// Static files (embedded)
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("loading static files: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	server := &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	return server.ListenAndServe()
}

// --- JSON helpers ---

func (s *Server) setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
}

func (s *Server) writeJSON(w http.ResponseWriter, data interface{}) {
	s.setCORS(w)
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) writeError(w http.ResponseWriter, status int, msg string) {
	s.setCORS(w)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// --- Response types ---

type OverviewResponse struct {
	Cluster    *ClusterInfo      `json:"cluster"`
	Nodes      []NodeInfo        `json:"nodes"`
	Pods       []PodInfo         `json:"pods"`
	Services   []ServiceInfo     `json:"services"`
	Namespaces []NamespaceInfo   `json:"namespaces"`
	Events     []EventInfo       `json:"events"`
	Addons     []AddonInfo       `json:"addons"`
	Pipelines  []PipelineRunInfo `json:"pipelines"`
	Workflows  []WorkflowRunInfo `json:"workflows"`
	Timestamp  string            `json:"timestamp"`
}

type ClusterInfo struct {
	Name       string `json:"name"`
	Runtime    string `json:"runtime"`
	Profile    string `json:"profile"`
	Phase      string `json:"phase"`
	Nodes      int    `json:"nodes"`
	ReadyNodes int    `json:"readyNodes"`
	PodCIDR    string `json:"podCIDR"`
}

type NodeInfo struct {
	Name    string `json:"name"`
	Role    string `json:"role"`
	Phase   string `json:"phase"`
	Address string `json:"address"`
}

type PodInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Image     string `json:"image"`
	Phase     string `json:"phase"`
	Node      string `json:"node"`
	Restarts  int    `json:"restarts"`
	PodIP     string `json:"podIP"`
}

type ServiceInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
	ClusterIP string `json:"clusterIP"`
	Ports     string `json:"ports"`
}

type NamespaceInfo struct {
	Name  string `json:"name"`
	Phase string `json:"phase"`
}

type EventInfo struct {
	Type    string `json:"type"`
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
	Time    string `json:"time"`
}

type AddonInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Version   string `json:"version"`
	Phase     string `json:"phase"`
}

type PipelineRunInfo struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	StartedAt     string `json:"startedAt"`
	FinishedAt    string `json:"finishedAt,omitempty"`
	Duration      string `json:"duration"`
	TotalTargets  int    `json:"totalTargets"`
	PassedTargets int    `json:"passedTargets"`
	FailedTargets int    `json:"failedTargets"`
	File          string `json:"file"`
	TargetFilter  string `json:"targetFilter,omitempty"`
}

type WorkflowRunInfo struct {
	ID           string `json:"id"`
	WorkflowFile string `json:"workflowFile"`
	Job          string `json:"job"`
	Event        string `json:"event"`
	Status       string `json:"status"`
	StartedAt    string `json:"startedAt"`
	Duration     string `json:"duration"`
	Error        string `json:"error,omitempty"`
}

// --- Conversion helpers ---

func clusterInfoFromMgr(ci *cluster.ClusterInfo) *ClusterInfo {
	return &ClusterInfo{
		Name:       ci.Name,
		Runtime:    ci.Runtime,
		Profile:    ci.Profile,
		Phase:      ci.Phase,
		Nodes:      ci.Nodes,
		ReadyNodes: ci.ReadyNodes,
		PodCIDR:    ci.PodCIDR,
	}
}

func nodeInfoFromMgr(ni cluster.NodeInfo) NodeInfo {
	return NodeInfo{
		Name:    ni.Name,
		Role:    ni.Role,
		Phase:   ni.Phase,
		Address: ni.Address,
	}
}

func podInfoFromRecord(p storage.PodRecord) PodInfo {
	return PodInfo{
		Name:      p.Name,
		Namespace: p.Namespace,
		Image:     p.Image,
		Phase:     p.Phase,
		Node:      p.Node,
		Restarts:  p.Restarts,
		PodIP:     p.PodIP,
	}
}

func serviceInfoFromRecord(s storage.ServiceRecord) ServiceInfo {
	return ServiceInfo{
		Name:      s.Name,
		Namespace: s.Namespace,
		Type:      s.Type,
		ClusterIP: s.ClusterIP,
		Ports:     formatServicePorts(s.Ports),
	}
}

func namespaceInfoFromRecord(ns storage.NamespaceRecord) NamespaceInfo {
	return NamespaceInfo{
		Name:  ns.Name,
		Phase: ns.Phase,
	}
}

func eventInfoFromRecord(e storage.EventRecord) EventInfo {
	return EventInfo{
		Type:    e.Type,
		Kind:    e.Kind,
		Name:    e.Name,
		Reason:  e.Reason,
		Message: e.Message,
		Time:    time.Unix(e.CreatedAt, 0).Format("15:04:05"),
	}
}

func addonInfoFromRecord(a storage.AddonRecord) AddonInfo {
	return AddonInfo{
		Name:      a.Name,
		Namespace: a.Namespace,
		Version:   a.Version,
		Phase:     a.Phase,
	}
}

func pipelineRunInfoFromRecord(run storage.PipelineRunRecord) PipelineRunInfo {
	started := time.Unix(run.StartedAt, 0)
	info := PipelineRunInfo{
		ID:            run.ID,
		Status:        run.Status,
		StartedAt:     started.Format("15:04:05"),
		TotalTargets:  run.TotalTargets,
		PassedTargets: run.PassedTargets,
		FailedTargets: run.FailedTargets,
		File:          run.File,
		TargetFilter:  run.TargetFilter,
	}
	if run.FinishedAt > 0 {
		finished := time.Unix(run.FinishedAt, 0)
		info.FinishedAt = finished.Format("15:04:05")
		info.Duration = finished.Sub(started).Round(time.Millisecond).String()
	} else {
		info.Duration = time.Since(started).Round(time.Second).String()
	}
	return info
}

// formatServicePorts parses the JSON-encoded []ServicePort and formats as "80:8080/TCP" style.
func formatServicePorts(portsJSON string) string {
	type servicePort struct {
		Name       string `json:"name"`
		Port       int    `json:"port"`
		TargetPort int    `json:"targetPort"`
		Protocol   string `json:"protocol"`
	}

	var ports []servicePort
	if err := json.Unmarshal([]byte(portsJSON), &ports); err != nil {
		return portsJSON
	}

	var parts []string
	for _, p := range ports {
		proto := p.Protocol
		if proto == "" {
			proto = "TCP"
		}
		if p.TargetPort != 0 && p.TargetPort != p.Port {
			parts = append(parts, fmt.Sprintf("%d:%d/%s", p.Port, p.TargetPort, proto))
		} else {
			parts = append(parts, fmt.Sprintf("%d/%s", p.Port, proto))
		}
	}
	return strings.Join(parts, ", ")
}

// --- Handlers ---

func (s *Server) handleCluster(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ci, err := s.clusterMgr.GetCluster(ctx, s.cluster)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, clusterInfoFromMgr(ci))
}

func (s *Server) handleNodes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nodes, err := s.clusterMgr.GetNodes(ctx, s.cluster)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	infos := make([]NodeInfo, len(nodes))
	for i, n := range nodes {
		infos[i] = nodeInfoFromMgr(n)
	}
	s.writeJSON(w, infos)
}

func (s *Server) handlePods(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := r.URL.Query().Get("namespace")
	pods, err := s.orch.ListPods(ctx, s.cluster, namespace)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	infos := make([]PodInfo, len(pods))
	for i, p := range pods {
		infos[i] = podInfoFromRecord(p)
	}
	s.writeJSON(w, infos)
}

func (s *Server) handleServices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := r.URL.Query().Get("namespace")
	svcs, err := s.orch.ListServices(ctx, s.cluster, namespace)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	infos := make([]ServiceInfo, len(svcs))
	for i, svc := range svcs {
		infos[i] = serviceInfoFromRecord(svc)
	}
	s.writeJSON(w, infos)
}

func (s *Server) handleDeployments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := r.URL.Query().Get("namespace")
	deps, err := s.orch.ListDeployments(ctx, s.cluster, namespace)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type DeploymentInfo struct {
		Name          string `json:"name"`
		Namespace     string `json:"namespace"`
		Replicas      int    `json:"replicas"`
		ReadyReplicas int    `json:"readyReplicas"`
		Phase         string `json:"phase"`
	}

	infos := make([]DeploymentInfo, len(deps))
	for i, d := range deps {
		infos[i] = DeploymentInfo{
			Name:          d.Name,
			Namespace:     d.Namespace,
			Replicas:      d.Replicas,
			ReadyReplicas: d.ReadyReplicas,
			Phase:         d.Phase,
		}
	}
	s.writeJSON(w, infos)
}

func (s *Server) handleNamespaces(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespaces, err := s.orch.ListNamespaces(ctx, s.cluster)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	infos := make([]NamespaceInfo, len(namespaces))
	for i, ns := range namespaces {
		infos[i] = namespaceInfoFromRecord(ns)
	}
	s.writeJSON(w, infos)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := r.URL.Query().Get("namespace")
	events, err := s.orch.ListEvents(ctx, s.cluster, namespace, 20)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	infos := make([]EventInfo, len(events))
	for i, e := range events {
		infos[i] = eventInfoFromRecord(e)
	}
	s.writeJSON(w, infos)
}

func (s *Server) handleAddons(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	addons, err := s.store.ListAddons(ctx, s.cluster)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	infos := make([]AddonInfo, len(addons))
	for i, a := range addons {
		infos[i] = addonInfoFromRecord(a)
	}
	s.writeJSON(w, infos)
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	resp := OverviewResponse{
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// Cluster info
	ci, err := s.clusterMgr.GetCluster(ctx, s.cluster)
	if err == nil {
		resp.Cluster = clusterInfoFromMgr(ci)
	}

	// Nodes
	nodes, err := s.clusterMgr.GetNodes(ctx, s.cluster)
	if err == nil {
		resp.Nodes = make([]NodeInfo, len(nodes))
		for i, n := range nodes {
			resp.Nodes[i] = nodeInfoFromMgr(n)
		}
	}

	// Pods (all namespaces)
	pods, err := s.orch.ListPods(ctx, s.cluster, "")
	if err == nil {
		resp.Pods = make([]PodInfo, len(pods))
		for i, p := range pods {
			resp.Pods[i] = podInfoFromRecord(p)
		}
	}

	// Services (all namespaces)
	svcs, err := s.orch.ListServices(ctx, s.cluster, "")
	if err == nil {
		resp.Services = make([]ServiceInfo, len(svcs))
		for i, svc := range svcs {
			resp.Services[i] = serviceInfoFromRecord(svc)
		}
	}

	// Namespaces
	namespaces, err := s.orch.ListNamespaces(ctx, s.cluster)
	if err == nil {
		resp.Namespaces = make([]NamespaceInfo, len(namespaces))
		for i, ns := range namespaces {
			resp.Namespaces[i] = namespaceInfoFromRecord(ns)
		}
	}

	// Events (last 20)
	events, err := s.orch.ListEvents(ctx, s.cluster, "", 20)
	if err == nil {
		resp.Events = make([]EventInfo, len(events))
		for i, e := range events {
			resp.Events[i] = eventInfoFromRecord(e)
		}
	}

	// Addons
	addons, err := s.store.ListAddons(ctx, s.cluster)
	if err == nil {
		resp.Addons = make([]AddonInfo, len(addons))
		for i, a := range addons {
			resp.Addons[i] = addonInfoFromRecord(a)
		}
	}

	// Recent pipeline runs
	pipelineRuns, err := s.store.ListPipelineRuns(ctx, 5)
	if err == nil {
		resp.Pipelines = make([]PipelineRunInfo, len(pipelineRuns))
		for i, run := range pipelineRuns {
			resp.Pipelines[i] = pipelineRunInfoFromRecord(run)
		}
	}

	// Recent workflow runs
	workflowRuns, err := s.store.ListWorkflowRuns(ctx, 5)
	if err == nil {
		resp.Workflows = make([]WorkflowRunInfo, len(workflowRuns))
		for i, run := range workflowRuns {
			resp.Workflows[i] = workflowRunInfoFromRecord(run)
		}
	}

	s.writeJSON(w, resp)
}

func (s *Server) handlePipelines(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	runs, err := s.store.ListPipelineRuns(ctx, 20)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]PipelineRunInfo, len(runs))
	for i, run := range runs {
		result[i] = pipelineRunInfoFromRecord(run)
	}

	s.writeJSON(w, result)
}

func workflowRunInfoFromRecord(run storage.WorkflowRunRecord) WorkflowRunInfo {
	started := time.Unix(run.StartedAt, 0)
	dur := fmt.Sprintf("%dms", run.DurationMs)
	if run.DurationMs >= 1000 {
		dur = fmt.Sprintf("%.1fs", float64(run.DurationMs)/1000)
	}
	return WorkflowRunInfo{
		ID:           run.ID,
		WorkflowFile: run.WorkflowFile,
		Job:          run.Job,
		Event:        run.Event,
		Status:       run.Status,
		StartedAt:    started.Format("15:04:05"),
		Duration:     dur,
		Error:        run.Error,
	}
}

func (s *Server) handleWorkflows(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	runs, err := s.store.ListWorkflowRuns(ctx, 20)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]WorkflowRunInfo, len(runs))
	for i, run := range runs {
		result[i] = workflowRunInfoFromRecord(run)
	}

	s.writeJSON(w, result)
}

func (s *Server) handleWorkflowLogs(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	// /api/workflows/{id}/logs → ["", "api", "workflows", "{id}", "logs"]
	if len(parts) < 5 || parts[4] != "logs" {
		s.writeError(w, http.StatusBadRequest, "expected /api/workflows/{id}/logs")
		return
	}
	runID := parts[3]

	ctx := context.Background()
	run, err := s.store.GetWorkflowRun(ctx, runID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "workflow run not found")
		return
	}

	s.writeJSON(w, map[string]string{
		"id":     run.ID,
		"output": run.Output,
	})
}

func (s *Server) handlePipelineTargets(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	// /api/pipelines/<id>/targets → ["", "api", "pipelines", "<id>", "targets"]
	if len(parts) < 5 {
		s.writeError(w, http.StatusBadRequest, "missing run ID")
		return
	}
	runID := parts[3]

	ctx := context.Background()
	targets, err := s.store.ListPipelineTargets(ctx, runID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type TargetInfo struct {
		Name     string `json:"name"`
		Status   string `json:"status"`
		Duration string `json:"duration"`
		Error    string `json:"error,omitempty"`
		Output   string `json:"output,omitempty"`
	}

	result := make([]TargetInfo, len(targets))
	for i, t := range targets {
		result[i] = TargetInfo{
			Name:     t.Name,
			Status:   t.Status,
			Duration: fmt.Sprintf("%dms", t.Duration),
			Error:    t.Error,
			Output:   t.Output,
		}
	}

	s.writeJSON(w, result)
}

func (s *Server) handlePipelineLive(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	clientCh := make(chan []byte, 100)
	s.liveMu.Lock()
	s.liveClients = append(s.liveClients, clientCh)
	s.liveMu.Unlock()

	defer func() {
		s.liveMu.Lock()
		for i, ch := range s.liveClients {
			if ch == clientCh {
				s.liveClients = append(s.liveClients[:i], s.liveClients[i+1:]...)
				break
			}
		}
		s.liveMu.Unlock()
	}()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-clientCh:
			_, _ = fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

// --- Pod detail, logs, and exec ---

// PodDetailResponse contains detailed info about a specific pod.
type PodDetailResponse struct {
	Name        string                   `json:"name"`
	Namespace   string                   `json:"namespace"`
	Image       string                   `json:"image"`
	Phase       string                   `json:"phase"`
	Node        string                   `json:"node"`
	PodIP       string                   `json:"podIP"`
	Restarts    int                      `json:"restarts"`
	ContainerID string                   `json:"containerID"`
	Labels      map[string]string        `json:"labels,omitempty"`
	Ports       []map[string]interface{} `json:"ports,omitempty"`
	Env         []map[string]string      `json:"env,omitempty"`
}

// handlePodRoute dispatches /api/pods/{namespace}/{name}[/logs|/exec] routes.
func (s *Server) handlePodRoute(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/pods/"), "/")

	switch {
	case len(parts) == 2:
		s.handlePodDetail(w, r)
	case len(parts) == 3 && parts[2] == "logs":
		s.handlePodLogs(w, r)
	case len(parts) == 3 && parts[2] == "exec":
		s.handlePodExec(w, r)
	default:
		s.handlePods(w, r)
	}
}

func (s *Server) handlePodDetail(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	// /api/pods/{namespace}/{name} → ["", "api", "pods", "{ns}", "{name}"]
	if len(parts) < 5 {
		s.writeError(w, http.StatusBadRequest, "invalid pod path")
		return
	}
	namespace := parts[3]
	podName := parts[4]

	ctx := r.Context()
	pod, err := s.store.GetPod(ctx, s.cluster, namespace, podName)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "pod not found")
		return
	}

	resp := PodDetailResponse{
		Name:        pod.Name,
		Namespace:   pod.Namespace,
		Image:       pod.Image,
		Phase:       pod.Phase,
		Node:        pod.Node,
		PodIP:       pod.PodIP,
		Restarts:    pod.Restarts,
		ContainerID: pod.ContainerID,
	}

	if pod.Labels != "" {
		_ = json.Unmarshal([]byte(pod.Labels), &resp.Labels)
	}
	if pod.Ports != "" {
		_ = json.Unmarshal([]byte(pod.Ports), &resp.Ports)
	}
	if pod.Env != "" {
		_ = json.Unmarshal([]byte(pod.Env), &resp.Env)
	}

	s.writeJSON(w, resp)
}

func (s *Server) handlePodLogs(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	// /api/pods/{ns}/{name}/logs → ["", "api", "pods", "{ns}", "{name}", "logs"]
	if len(parts) < 6 {
		s.writeError(w, http.StatusBadRequest, "invalid pod logs path")
		return
	}
	namespace := parts[3]
	podName := parts[4]

	ctx := r.Context()
	pod, err := s.store.GetPod(ctx, s.cluster, namespace, podName)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "pod not found")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	reader, err := s.orch.LogsFromPod(ctx, pod, runtime.LogOptions{
		Follow:     true,
		Tail:       "100",
		Timestamps: true,
	})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to get logs")
		return
	}
	defer func() { _ = reader.Close() }()

	buf := make([]byte, 4096)
	for {
		select {
		case <-r.Context().Done():
			return
		default:
		}

		n, readErr := reader.Read(buf)
		if n > 0 {
			lines := strings.Split(string(buf[:n]), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				clean := stripDockerLogHeader(line)
				_, _ = fmt.Fprintf(w, "data: %s\n\n", clean)
				flusher.Flush()
			}
		}
		if readErr != nil {
			return
		}
	}
}

// stripDockerLogHeader removes the 8-byte Docker log stream header if present.
// Docker multiplexed log format: first byte is stream type (1=stdout, 2=stderr),
// bytes 1-3 are padding, bytes 4-7 are length (big-endian uint32).
func stripDockerLogHeader(line string) string {
	if len(line) > 8 && (line[0] == 1 || line[0] == 2) {
		return line[8:]
	}
	return line
}

func (s *Server) handlePodExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	// /api/pods/{ns}/{name}/exec → ["", "api", "pods", "{ns}", "{name}", "exec"]
	if len(parts) < 6 {
		s.writeError(w, http.StatusBadRequest, "invalid pod exec path")
		return
	}
	namespace := parts[3]
	podName := parts[4]

	var req struct {
		Command string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	pod, err := s.store.GetPod(ctx, s.cluster, namespace, podName)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "pod not found")
		return
	}

	var stdout bytes.Buffer
	cmd := []string{"sh", "-c", req.Command}
	result, err := s.orch.ExecInPod(ctx, pod, cmd, runtime.ExecOptions{
		Stdout: &stdout,
	})

	var output string
	if stdout.Len() > 0 {
		output = stripDockerLogHeader(stdout.String())
	} else if result != nil {
		output = stripDockerLogHeader(result.Output)
	}

	exitCode := 0
	if result != nil {
		exitCode = result.ExitCode
	}
	if err != nil && output == "" {
		output = err.Error()
		exitCode = 1
	}

	s.writeJSON(w, map[string]interface{}{
		"output":   output,
		"exitCode": exitCode,
	})
}
