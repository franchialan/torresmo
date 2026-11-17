package dashboard

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/torresmo-dev/torresmo/internal/cluster"
	"github.com/torresmo-dev/torresmo/internal/orchestrator"
	"github.com/torresmo-dev/torresmo/internal/storage"
)

//go:embed static/*
var staticFiles embed.FS

// Server serves the web dashboard.
type Server struct {
	clusterMgr *cluster.Manager
	orch       *orchestrator.Orchestrator
	store      storage.Store
	cluster    string
	addr       string
}

func NewServer(mgr *cluster.Manager, orch *orchestrator.Orchestrator, store storage.Store, clusterName, addr string) *Server {
	return &Server{
		clusterMgr: mgr,
		orch:       orch,
		store:      store,
		cluster:    clusterName,
		addr:       addr,
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/cluster", s.handleCluster)
	mux.HandleFunc("/api/nodes", s.handleNodes)
	mux.HandleFunc("/api/pods", s.handlePods)
	mux.HandleFunc("/api/services", s.handleServices)
	mux.HandleFunc("/api/deployments", s.handleDeployments)
	mux.HandleFunc("/api/namespaces", s.handleNamespaces)
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/api/addons", s.handleAddons)
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
		server.Shutdown(shutdownCtx)
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
	json.NewEncoder(w).Encode(data)
}

func (s *Server) writeError(w http.ResponseWriter, status int, msg string) {
	s.setCORS(w)
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// --- Response types ---

type OverviewResponse struct {
	Cluster    *ClusterInfo    `json:"cluster"`
	Nodes      []NodeInfo      `json:"nodes"`
	Pods       []PodInfo       `json:"pods"`
	Services   []ServiceInfo   `json:"services"`
	Namespaces []NamespaceInfo `json:"namespaces"`
	Events     []EventInfo     `json:"events"`
	Addons     []AddonInfo     `json:"addons"`
	Timestamp  string          `json:"timestamp"`
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

	s.writeJSON(w, resp)
}
