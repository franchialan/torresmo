package v1

// ObjectMeta is standard metadata for all Torresmo resources.
type ObjectMeta struct {
	Name      string            `yaml:"name" json:"name"`
	Namespace string            `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Labels    map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

// TypeMeta describes the API version and kind.
type TypeMeta struct {
	APIVersion string `yaml:"apiVersion" json:"apiVersion"`
	Kind       string `yaml:"kind" json:"kind"`
}

// Cluster represents a Torresmo cluster.
type Cluster struct {
	TypeMeta   `yaml:",inline" json:",inline"`
	ObjectMeta `yaml:"metadata" json:"metadata"`
	Spec       ClusterSpec   `yaml:"spec" json:"spec"`
	Status     ClusterStatus `yaml:"status,omitempty" json:"status,omitempty"`
}

type ClusterSpec struct {
	Nodes      []NodeSpec     `yaml:"nodes" json:"nodes"`
	Runtime    string         `yaml:"runtime,omitempty" json:"runtime,omitempty"`
	Profile    string         `yaml:"profile,omitempty" json:"profile,omitempty"`
	Networking NetworkingSpec `yaml:"networking,omitempty" json:"networking,omitempty"`
	Addons     []AddonRef     `yaml:"addons,omitempty" json:"addons,omitempty"`
}

type NodeSpec struct {
	Role  string `yaml:"role" json:"role"`
	Count int    `yaml:"count,omitempty" json:"count,omitempty"`
}

type NetworkingSpec struct {
	PodCIDR string `yaml:"podCIDR,omitempty" json:"podCIDR,omitempty"`
}

type AddonRef struct {
	Name      string            `yaml:"name" json:"name"`
	Version   string            `yaml:"version,omitempty" json:"version,omitempty"`
	Namespace string            `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Config    map[string]string `yaml:"config,omitempty" json:"config,omitempty"`
}

type ClusterStatus struct {
	Phase   string `yaml:"phase" json:"phase"`
	Nodes   int    `yaml:"nodes" json:"nodes"`
	Ready   int    `yaml:"ready" json:"ready"`
	Message string `yaml:"message,omitempty" json:"message,omitempty"`
}

// Node represents a node (container) in a cluster.
type Node struct {
	TypeMeta   `yaml:",inline" json:",inline"`
	ObjectMeta `yaml:"metadata" json:"metadata"`
	Spec       NodeDetailSpec `yaml:"spec" json:"spec"`
	Status     NodeStatus     `yaml:"status,omitempty" json:"status,omitempty"`
}

type NodeDetailSpec struct {
	Role    string `yaml:"role" json:"role"`
	Cluster string `yaml:"cluster" json:"cluster"`
}

type NodeStatus struct {
	Phase    string `yaml:"phase" json:"phase"`
	Address  string `yaml:"address,omitempty" json:"address,omitempty"`
	CPU      string `yaml:"cpu,omitempty" json:"cpu,omitempty"`
	Memory   string `yaml:"memory,omitempty" json:"memory,omitempty"`
	PodCount int    `yaml:"podCount" json:"podCount"`
}

// Pod represents a workload unit.
type Pod struct {
	TypeMeta   `yaml:",inline" json:",inline"`
	ObjectMeta `yaml:"metadata" json:"metadata"`
	Spec       PodSpec   `yaml:"spec" json:"spec"`
	Status     PodStatus `yaml:"status,omitempty" json:"status,omitempty"`
}

type PodSpec struct {
	Containers []ContainerSpec `yaml:"containers" json:"containers"`
}

type ContainerSpec struct {
	Name    string     `yaml:"name" json:"name"`
	Image   string     `yaml:"image" json:"image"`
	Command []string   `yaml:"command,omitempty" json:"command,omitempty"`
	Args    []string   `yaml:"args,omitempty" json:"args,omitempty"`
	Ports   []PortSpec `yaml:"ports,omitempty" json:"ports,omitempty"`
	Env     []EnvVar   `yaml:"env,omitempty" json:"env,omitempty"`
}

type PortSpec struct {
	ContainerPort int    `yaml:"containerPort" json:"containerPort"`
	Protocol      string `yaml:"protocol,omitempty" json:"protocol,omitempty"`
}

type EnvVar struct {
	Name  string `yaml:"name" json:"name"`
	Value string `yaml:"value" json:"value"`
}

type PodStatus struct {
	Phase    string `yaml:"phase" json:"phase"`
	Node     string `yaml:"node,omitempty" json:"node,omitempty"`
	PodIP    string `yaml:"podIP,omitempty" json:"podIP,omitempty"`
	Restarts int    `yaml:"restarts" json:"restarts"`
	Message  string `yaml:"message,omitempty" json:"message,omitempty"`
}

// Deployment manages a set of pod replicas.
type Deployment struct {
	TypeMeta   `yaml:",inline" json:",inline"`
	ObjectMeta `yaml:"metadata" json:"metadata"`
	Spec       DeploymentSpec   `yaml:"spec" json:"spec"`
	Status     DeploymentStatus `yaml:"status,omitempty" json:"status,omitempty"`
}

type DeploymentSpec struct {
	Replicas int         `yaml:"replicas" json:"replicas"`
	Template PodTemplate `yaml:"template" json:"template"`
}

type PodTemplate struct {
	ObjectMeta `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	Spec       PodSpec `yaml:"spec" json:"spec"`
}

type DeploymentStatus struct {
	Replicas      int `yaml:"replicas" json:"replicas"`
	ReadyReplicas int `yaml:"readyReplicas" json:"readyReplicas"`
}

// Service exposes a set of pods.
type Service struct {
	TypeMeta   `yaml:",inline" json:",inline"`
	ObjectMeta `yaml:"metadata" json:"metadata"`
	Spec       ServiceSpec `yaml:"spec" json:"spec"`
}

type ServiceSpec struct {
	Type     string            `yaml:"type,omitempty" json:"type,omitempty"`
	Ports    []ServicePort     `yaml:"ports" json:"ports"`
	Selector map[string]string `yaml:"selector" json:"selector"`
}

type ServicePort struct {
	Name       string `yaml:"name,omitempty" json:"name,omitempty"`
	Port       int    `yaml:"port" json:"port"`
	TargetPort int    `yaml:"targetPort" json:"targetPort"`
	Protocol   string `yaml:"protocol,omitempty" json:"protocol,omitempty"`
}

// Namespace provides scope for resources.
type Namespace struct {
	TypeMeta   `yaml:",inline" json:",inline"`
	ObjectMeta `yaml:"metadata" json:"metadata"`
	Status     NamespaceStatus `yaml:"status,omitempty" json:"status,omitempty"`
}

type NamespaceStatus struct {
	Phase string `yaml:"phase" json:"phase"`
}

// Environment defines a complete development environment (for torresmo up).
type Environment struct {
	TypeMeta   `yaml:",inline" json:",inline"`
	ObjectMeta `yaml:"metadata" json:"metadata"`
	Spec       EnvironmentSpec `yaml:"spec" json:"spec"`
}

type EnvironmentSpec struct {
	Cluster   ClusterSpec `yaml:"cluster" json:"cluster"`
	Addons    []AddonRef  `yaml:"addons,omitempty" json:"addons,omitempty"`
	Manifests []string    `yaml:"manifests,omitempty" json:"manifests,omitempty"`
}
