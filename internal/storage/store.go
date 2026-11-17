package storage

import "context"

// Store provides persistence for cluster state.
type Store interface {
	SaveCluster(ctx context.Context, cluster *ClusterRecord) error
	GetCluster(ctx context.Context, name string) (*ClusterRecord, error)
	ListClusters(ctx context.Context) ([]ClusterRecord, error)
	DeleteCluster(ctx context.Context, name string) error

	SaveNode(ctx context.Context, node *NodeRecord) error
	GetNode(ctx context.Context, clusterName, nodeName string) (*NodeRecord, error)
	ListNodes(ctx context.Context, clusterName string) ([]NodeRecord, error)
	DeleteNode(ctx context.Context, clusterName, nodeName string) error

	// Namespace operations
	SaveNamespace(ctx context.Context, ns *NamespaceRecord) error
	GetNamespace(ctx context.Context, clusterName, name string) (*NamespaceRecord, error)
	ListNamespaces(ctx context.Context, clusterName string) ([]NamespaceRecord, error)
	DeleteNamespace(ctx context.Context, clusterName, name string) error

	// Pod operations
	SavePod(ctx context.Context, pod *PodRecord) error
	GetPod(ctx context.Context, clusterName, namespace, name string) (*PodRecord, error)
	ListPods(ctx context.Context, clusterName, namespace string) ([]PodRecord, error)
	ListAllPods(ctx context.Context, clusterName string) ([]PodRecord, error)
	DeletePod(ctx context.Context, clusterName, namespace, name string) error

	// Deployment operations
	SaveDeployment(ctx context.Context, dep *DeploymentRecord) error
	GetDeployment(ctx context.Context, clusterName, namespace, name string) (*DeploymentRecord, error)
	ListDeployments(ctx context.Context, clusterName, namespace string) ([]DeploymentRecord, error)
	DeleteDeployment(ctx context.Context, clusterName, namespace, name string) error

	// Service operations
	SaveService(ctx context.Context, svc *ServiceRecord) error
	GetService(ctx context.Context, clusterName, namespace, name string) (*ServiceRecord, error)
	ListServices(ctx context.Context, clusterName, namespace string) ([]ServiceRecord, error)
	DeleteService(ctx context.Context, clusterName, namespace, name string) error

	// Event operations
	SaveEvent(ctx context.Context, event *EventRecord) error
	ListEvents(ctx context.Context, clusterName, namespace string, limit int) ([]EventRecord, error)
	ListEventsByResource(ctx context.Context, clusterName, kind, name string, limit int) ([]EventRecord, error)

	// Addon operations
	SaveAddon(ctx context.Context, addon *AddonRecord) error
	GetAddon(ctx context.Context, clusterName, namespace, name string) (*AddonRecord, error)
	ListAddons(ctx context.Context, clusterName string) ([]AddonRecord, error)
	DeleteAddon(ctx context.Context, clusterName, namespace, name string) error

	Close() error
}

type ClusterRecord struct {
	Name       string
	Runtime    string
	Profile    string
	Phase      string
	NodeCount  int
	ReadyNodes int
	PodCIDR    string
	CreatedAt  int64
	UpdatedAt  int64
}

type NodeRecord struct {
	Name        string
	ClusterName string
	Role        string
	ContainerID string
	Address     string
	Phase       string
	CreatedAt   int64
	UpdatedAt   int64
}

type NamespaceRecord struct {
	Name        string
	ClusterName string
	Phase       string // Active, Terminating
	CreatedAt   int64
	UpdatedAt   int64
}

type PodRecord struct {
	Name        string
	ClusterName string
	Namespace   string
	ContainerID string
	Image       string
	Command     string // JSON-encoded []string
	Phase       string // Pending, Running, Succeeded, Failed
	Node        string // node name where scheduled
	PodIP       string
	Restarts    int
	Labels      string // JSON-encoded map[string]string
	Env         string // JSON-encoded []EnvVar
	Ports       string // JSON-encoded []PortSpec
	CreatedAt   int64
	UpdatedAt   int64
}

type DeploymentRecord struct {
	Name          string
	ClusterName   string
	Namespace     string
	Replicas      int
	ReadyReplicas int
	Template      string // JSON-encoded PodSpec
	Labels        string // JSON-encoded map[string]string
	Selector      string // JSON-encoded map[string]string
	Phase         string // Available, Progressing, Failed
	CreatedAt     int64
	UpdatedAt     int64
}

type ServiceRecord struct {
	Name        string
	ClusterName string
	Namespace   string
	Type        string // ClusterIP, NodePort
	Ports       string // JSON-encoded []ServicePort
	Selector    string // JSON-encoded map[string]string
	ClusterIP   string
	CreatedAt   int64
	UpdatedAt   int64
}

type EventRecord struct {
	ID          int64
	ClusterName string
	Namespace   string
	Kind        string // Pod, Deployment, Service, Node
	Name        string // resource name
	Type        string // Normal, Warning
	Reason      string
	Message     string
	CreatedAt   int64
}

type AddonRecord struct {
	Name        string
	ClusterName string
	Namespace   string
	Version     string
	Phase       string // Installing, Running, Failed, Stopped
	Config      string // JSON-encoded map[string]string
	CreatedAt   int64
	UpdatedAt   int64
}
