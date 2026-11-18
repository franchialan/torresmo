package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/torresmo-dev/torresmo/internal/storage"

	_ "modernc.org/sqlite"
)

// DefaultDBPath returns the default database path (~/.torresmo/torresmo.db).
func DefaultDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine home directory: %w", err)
	}
	return filepath.Join(home, ".torresmo", "torresmo.db"), nil
}

// Store implements storage.Store using SQLite.
type Store struct {
	db *sql.DB
}

// New opens (or creates) a SQLite database at dbPath and runs migrations.
func New(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return s, nil
}

func (s *Store) migrate() error {
	const clustersTable = `
		CREATE TABLE IF NOT EXISTS clusters (
			name TEXT PRIMARY KEY,
			runtime TEXT NOT NULL,
			profile TEXT NOT NULL DEFAULT 'standard',
			phase TEXT NOT NULL DEFAULT 'Creating',
			node_count INTEGER NOT NULL DEFAULT 0,
			ready_nodes INTEGER NOT NULL DEFAULT 0,
			pod_cidr TEXT NOT NULL DEFAULT '10.244.0.0/16',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);`

	const nodesTable = `
		CREATE TABLE IF NOT EXISTS nodes (
			name TEXT NOT NULL,
			cluster_name TEXT NOT NULL,
			role TEXT NOT NULL,
			container_id TEXT NOT NULL DEFAULT '',
			address TEXT NOT NULL DEFAULT '',
			phase TEXT NOT NULL DEFAULT 'Creating',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			PRIMARY KEY (cluster_name, name),
			FOREIGN KEY (cluster_name) REFERENCES clusters(name) ON DELETE CASCADE
		);`

	const namespacesTable = `
		CREATE TABLE IF NOT EXISTS namespaces (
			name TEXT NOT NULL,
			cluster_name TEXT NOT NULL,
			phase TEXT NOT NULL DEFAULT 'Active',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			PRIMARY KEY (cluster_name, name),
			FOREIGN KEY (cluster_name) REFERENCES clusters(name) ON DELETE CASCADE
		);`

	const podsTable = `
		CREATE TABLE IF NOT EXISTS pods (
			name TEXT NOT NULL,
			cluster_name TEXT NOT NULL,
			namespace TEXT NOT NULL DEFAULT 'default',
			container_id TEXT NOT NULL DEFAULT '',
			image TEXT NOT NULL DEFAULT '',
			command TEXT NOT NULL DEFAULT '[]',
			phase TEXT NOT NULL DEFAULT 'Pending',
			node TEXT NOT NULL DEFAULT '',
			pod_ip TEXT NOT NULL DEFAULT '',
			restarts INTEGER NOT NULL DEFAULT 0,
			labels TEXT NOT NULL DEFAULT '{}',
			env TEXT NOT NULL DEFAULT '[]',
			ports TEXT NOT NULL DEFAULT '[]',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			PRIMARY KEY (cluster_name, namespace, name),
			FOREIGN KEY (cluster_name) REFERENCES clusters(name) ON DELETE CASCADE
		);`

	const deploymentsTable = `
		CREATE TABLE IF NOT EXISTS deployments (
			name TEXT NOT NULL,
			cluster_name TEXT NOT NULL,
			namespace TEXT NOT NULL DEFAULT 'default',
			replicas INTEGER NOT NULL DEFAULT 1,
			ready_replicas INTEGER NOT NULL DEFAULT 0,
			template TEXT NOT NULL DEFAULT '{}',
			labels TEXT NOT NULL DEFAULT '{}',
			selector TEXT NOT NULL DEFAULT '{}',
			phase TEXT NOT NULL DEFAULT 'Progressing',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			PRIMARY KEY (cluster_name, namespace, name),
			FOREIGN KEY (cluster_name) REFERENCES clusters(name) ON DELETE CASCADE
		);`

	const servicesTable = `
		CREATE TABLE IF NOT EXISTS services (
			name TEXT NOT NULL,
			cluster_name TEXT NOT NULL,
			namespace TEXT NOT NULL DEFAULT 'default',
			type TEXT NOT NULL DEFAULT 'ClusterIP',
			ports TEXT NOT NULL DEFAULT '[]',
			selector TEXT NOT NULL DEFAULT '{}',
			cluster_ip TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			PRIMARY KEY (cluster_name, namespace, name),
			FOREIGN KEY (cluster_name) REFERENCES clusters(name) ON DELETE CASCADE
		);`

	const eventsTable = `
		CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			cluster_name TEXT NOT NULL,
			namespace TEXT NOT NULL DEFAULT '',
			kind TEXT NOT NULL,
			name TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT 'Normal',
			reason TEXT NOT NULL DEFAULT '',
			message TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL,
			FOREIGN KEY (cluster_name) REFERENCES clusters(name) ON DELETE CASCADE
		);`

	if _, err := s.db.Exec(clustersTable); err != nil {
		return fmt.Errorf("creating clusters table: %w", err)
	}
	if _, err := s.db.Exec(nodesTable); err != nil {
		return fmt.Errorf("creating nodes table: %w", err)
	}
	if _, err := s.db.Exec(namespacesTable); err != nil {
		return fmt.Errorf("creating namespaces table: %w", err)
	}
	if _, err := s.db.Exec(podsTable); err != nil {
		return fmt.Errorf("creating pods table: %w", err)
	}
	if _, err := s.db.Exec(deploymentsTable); err != nil {
		return fmt.Errorf("creating deployments table: %w", err)
	}
	if _, err := s.db.Exec(servicesTable); err != nil {
		return fmt.Errorf("creating services table: %w", err)
	}
	if _, err := s.db.Exec(eventsTable); err != nil {
		return fmt.Errorf("creating events table: %w", err)
	}

	const addonsTable = `
		CREATE TABLE IF NOT EXISTS addons (
			name TEXT NOT NULL,
			cluster_name TEXT NOT NULL,
			namespace TEXT NOT NULL DEFAULT 'default',
			version TEXT NOT NULL DEFAULT 'latest',
			phase TEXT NOT NULL DEFAULT 'Installing',
			config TEXT NOT NULL DEFAULT '{}',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			PRIMARY KEY (cluster_name, namespace, name),
			FOREIGN KEY (cluster_name) REFERENCES clusters(name) ON DELETE CASCADE
		);`

	if _, err := s.db.Exec(addonsTable); err != nil {
		return fmt.Errorf("creating addons table: %w", err)
	}

	const pipelineRunsTable = `
		CREATE TABLE IF NOT EXISTS pipeline_runs (
			id TEXT PRIMARY KEY,
			cluster_name TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'running',
			started_at INTEGER NOT NULL,
			finished_at INTEGER NOT NULL DEFAULT 0,
			total_targets INTEGER NOT NULL DEFAULT 0,
			passed_targets INTEGER NOT NULL DEFAULT 0,
			failed_targets INTEGER NOT NULL DEFAULT 0,
			file TEXT NOT NULL DEFAULT 'Torresmofile',
			target_filter TEXT NOT NULL DEFAULT ''
		);`

	const pipelineTargetsTable = `
		CREATE TABLE IF NOT EXISTS pipeline_targets (
			run_id TEXT NOT NULL,
			name TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			started_at INTEGER NOT NULL DEFAULT 0,
			finished_at INTEGER NOT NULL DEFAULT 0,
			duration INTEGER NOT NULL DEFAULT 0,
			error TEXT NOT NULL DEFAULT '',
			output TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (run_id, name),
			FOREIGN KEY (run_id) REFERENCES pipeline_runs(id) ON DELETE CASCADE
		);`

	if _, err := s.db.Exec(pipelineRunsTable); err != nil {
		return fmt.Errorf("creating pipeline_runs table: %w", err)
	}
	if _, err := s.db.Exec(pipelineTargetsTable); err != nil {
		return fmt.Errorf("creating pipeline_targets table: %w", err)
	}

	const workflowRunsTable = `
		CREATE TABLE IF NOT EXISTS workflow_runs (
			id TEXT PRIMARY KEY,
			workflow_file TEXT NOT NULL DEFAULT '',
			job TEXT NOT NULL DEFAULT '',
			event TEXT NOT NULL DEFAULT 'push',
			status TEXT NOT NULL DEFAULT 'running',
			started_at INTEGER NOT NULL,
			finished_at INTEGER NOT NULL DEFAULT 0,
			duration_ms INTEGER NOT NULL DEFAULT 0,
			error TEXT NOT NULL DEFAULT '',
			output TEXT NOT NULL DEFAULT ''
		);`

	if _, err := s.db.Exec(workflowRunsTable); err != nil {
		return fmt.Errorf("creating workflow_runs table: %w", err)
	}

	return nil
}

func (s *Store) SaveCluster(ctx context.Context, cluster *storage.ClusterRecord) error {
	const q = `
		INSERT INTO clusters
			(name, runtime, profile, phase, node_count, ready_nodes, pod_cidr, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			runtime = excluded.runtime,
			profile = excluded.profile,
			phase = excluded.phase,
			node_count = excluded.node_count,
			ready_nodes = excluded.ready_nodes,
			pod_cidr = excluded.pod_cidr,
			updated_at = excluded.updated_at`

	_, err := s.db.ExecContext(ctx, q,
		cluster.Name,
		cluster.Runtime,
		cluster.Profile,
		cluster.Phase,
		cluster.NodeCount,
		cluster.ReadyNodes,
		cluster.PodCIDR,
		cluster.CreatedAt,
		cluster.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("saving cluster %q: %w", cluster.Name, err)
	}
	return nil
}

func (s *Store) GetCluster(ctx context.Context, name string) (*storage.ClusterRecord, error) {
	const q = `
		SELECT name, runtime, profile, phase, node_count, ready_nodes, pod_cidr, created_at, updated_at
		FROM clusters WHERE name = ?`

	var c storage.ClusterRecord
	err := s.db.QueryRowContext(ctx, q, name).Scan(
		&c.Name,
		&c.Runtime,
		&c.Profile,
		&c.Phase,
		&c.NodeCount,
		&c.ReadyNodes,
		&c.PodCIDR,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("cluster %q not found", name)
	}
	if err != nil {
		return nil, fmt.Errorf("getting cluster %q: %w", name, err)
	}
	return &c, nil
}

func (s *Store) ListClusters(ctx context.Context) ([]storage.ClusterRecord, error) {
	const q = `
		SELECT name, runtime, profile, phase, node_count, ready_nodes, pod_cidr, created_at, updated_at
		FROM clusters ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("listing clusters: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var clusters []storage.ClusterRecord
	for rows.Next() {
		var c storage.ClusterRecord
		if err := rows.Scan(
			&c.Name,
			&c.Runtime,
			&c.Profile,
			&c.Phase,
			&c.NodeCount,
			&c.ReadyNodes,
			&c.PodCIDR,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning cluster row: %w", err)
		}
		clusters = append(clusters, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating cluster rows: %w", err)
	}
	return clusters, nil
}

func (s *Store) DeleteCluster(ctx context.Context, name string) error {
	// Explicitly delete nodes first for safety, even though CASCADE should handle it.
	if _, err := s.db.ExecContext(ctx, "DELETE FROM nodes WHERE cluster_name = ?", name); err != nil {
		return fmt.Errorf("deleting nodes for cluster %q: %w", name, err)
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM clusters WHERE name = ?", name); err != nil {
		return fmt.Errorf("deleting cluster %q: %w", name, err)
	}
	return nil
}

func (s *Store) SaveNode(ctx context.Context, node *storage.NodeRecord) error {
	const q = `
		INSERT INTO nodes
			(name, cluster_name, role, container_id, address, phase, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(cluster_name, name) DO UPDATE SET
			role = excluded.role,
			container_id = excluded.container_id,
			address = excluded.address,
			phase = excluded.phase,
			updated_at = excluded.updated_at`

	_, err := s.db.ExecContext(ctx, q,
		node.Name,
		node.ClusterName,
		node.Role,
		node.ContainerID,
		node.Address,
		node.Phase,
		node.CreatedAt,
		node.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("saving node %q in cluster %q: %w", node.Name, node.ClusterName, err)
	}
	return nil
}

func (s *Store) GetNode(ctx context.Context, clusterName, nodeName string) (*storage.NodeRecord, error) {
	const q = `
		SELECT name, cluster_name, role, container_id, address, phase, created_at, updated_at
		FROM nodes WHERE cluster_name = ? AND name = ?`

	var n storage.NodeRecord
	err := s.db.QueryRowContext(ctx, q, clusterName, nodeName).Scan(
		&n.Name,
		&n.ClusterName,
		&n.Role,
		&n.ContainerID,
		&n.Address,
		&n.Phase,
		&n.CreatedAt,
		&n.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("node %q not found in cluster %q", nodeName, clusterName)
	}
	if err != nil {
		return nil, fmt.Errorf("getting node %q in cluster %q: %w", nodeName, clusterName, err)
	}
	return &n, nil
}

func (s *Store) ListNodes(ctx context.Context, clusterName string) ([]storage.NodeRecord, error) {
	const q = `
		SELECT name, cluster_name, role, container_id, address, phase, created_at, updated_at
		FROM nodes WHERE cluster_name = ? ORDER BY name`

	rows, err := s.db.QueryContext(ctx, q, clusterName)
	if err != nil {
		return nil, fmt.Errorf("listing nodes for cluster %q: %w", clusterName, err)
	}
	defer func() { _ = rows.Close() }()

	var nodes []storage.NodeRecord
	for rows.Next() {
		var n storage.NodeRecord
		if err := rows.Scan(
			&n.Name,
			&n.ClusterName,
			&n.Role,
			&n.ContainerID,
			&n.Address,
			&n.Phase,
			&n.CreatedAt,
			&n.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning node row: %w", err)
		}
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating node rows: %w", err)
	}
	return nodes, nil
}

func (s *Store) DeleteNode(ctx context.Context, clusterName, nodeName string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM nodes WHERE cluster_name = ? AND name = ?", clusterName, nodeName)
	if err != nil {
		return fmt.Errorf("deleting node %q in cluster %q: %w", nodeName, clusterName, err)
	}
	return nil
}

// --- Namespace operations ---

func (s *Store) SaveNamespace(ctx context.Context, ns *storage.NamespaceRecord) error {
	const q = `
		INSERT INTO namespaces
			(name, cluster_name, phase, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(cluster_name, name) DO UPDATE SET
			phase = excluded.phase,
			updated_at = excluded.updated_at`

	_, err := s.db.ExecContext(ctx, q,
		ns.Name,
		ns.ClusterName,
		ns.Phase,
		ns.CreatedAt,
		ns.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("saving namespace %q in cluster %q: %w", ns.Name, ns.ClusterName, err)
	}
	return nil
}

func (s *Store) GetNamespace(ctx context.Context, clusterName, name string) (*storage.NamespaceRecord, error) {
	const q = `
		SELECT name, cluster_name, phase, created_at, updated_at
		FROM namespaces WHERE cluster_name = ? AND name = ?`

	var ns storage.NamespaceRecord
	err := s.db.QueryRowContext(ctx, q, clusterName, name).Scan(
		&ns.Name,
		&ns.ClusterName,
		&ns.Phase,
		&ns.CreatedAt,
		&ns.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("namespace %q not found in cluster %q", name, clusterName)
	}
	if err != nil {
		return nil, fmt.Errorf("getting namespace %q in cluster %q: %w", name, clusterName, err)
	}
	return &ns, nil
}

func (s *Store) ListNamespaces(ctx context.Context, clusterName string) ([]storage.NamespaceRecord, error) {
	const q = `
		SELECT name, cluster_name, phase, created_at, updated_at
		FROM namespaces WHERE cluster_name = ? ORDER BY name`

	rows, err := s.db.QueryContext(ctx, q, clusterName)
	if err != nil {
		return nil, fmt.Errorf("listing namespaces for cluster %q: %w", clusterName, err)
	}
	defer func() { _ = rows.Close() }()

	var namespaces []storage.NamespaceRecord
	for rows.Next() {
		var ns storage.NamespaceRecord
		if err := rows.Scan(
			&ns.Name,
			&ns.ClusterName,
			&ns.Phase,
			&ns.CreatedAt,
			&ns.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning namespace row: %w", err)
		}
		namespaces = append(namespaces, ns)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating namespace rows: %w", err)
	}
	return namespaces, nil
}

func (s *Store) DeleteNamespace(ctx context.Context, clusterName, name string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM namespaces WHERE cluster_name = ? AND name = ?", clusterName, name)
	if err != nil {
		return fmt.Errorf("deleting namespace %q in cluster %q: %w", name, clusterName, err)
	}
	return nil
}

// --- Pod operations ---

func (s *Store) SavePod(ctx context.Context, pod *storage.PodRecord) error {
	const q = `
		INSERT INTO pods
			(name, cluster_name, namespace, container_id, image, command, phase, node, pod_ip, restarts, labels, env, ports, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(cluster_name, namespace, name) DO UPDATE SET
			container_id = excluded.container_id,
			image = excluded.image,
			command = excluded.command,
			phase = excluded.phase,
			node = excluded.node,
			pod_ip = excluded.pod_ip,
			restarts = excluded.restarts,
			labels = excluded.labels,
			env = excluded.env,
			ports = excluded.ports,
			updated_at = excluded.updated_at`

	_, err := s.db.ExecContext(ctx, q,
		pod.Name,
		pod.ClusterName,
		pod.Namespace,
		pod.ContainerID,
		pod.Image,
		pod.Command,
		pod.Phase,
		pod.Node,
		pod.PodIP,
		pod.Restarts,
		pod.Labels,
		pod.Env,
		pod.Ports,
		pod.CreatedAt,
		pod.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("saving pod %q in cluster %q namespace %q: %w", pod.Name, pod.ClusterName, pod.Namespace, err)
	}
	return nil
}

func (s *Store) GetPod(ctx context.Context, clusterName, namespace, name string) (*storage.PodRecord, error) {
	const q = `
		SELECT name, cluster_name, namespace, container_id, image, command, phase, node, pod_ip, restarts, labels, env, ports, created_at, updated_at
		FROM pods WHERE cluster_name = ? AND namespace = ? AND name = ?`

	var p storage.PodRecord
	err := s.db.QueryRowContext(ctx, q, clusterName, namespace, name).Scan(
		&p.Name,
		&p.ClusterName,
		&p.Namespace,
		&p.ContainerID,
		&p.Image,
		&p.Command,
		&p.Phase,
		&p.Node,
		&p.PodIP,
		&p.Restarts,
		&p.Labels,
		&p.Env,
		&p.Ports,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("pod %q not found in cluster %q namespace %q", name, clusterName, namespace)
	}
	if err != nil {
		return nil, fmt.Errorf("getting pod %q in cluster %q namespace %q: %w", name, clusterName, namespace, err)
	}
	return &p, nil
}

func (s *Store) ListPods(ctx context.Context, clusterName, namespace string) ([]storage.PodRecord, error) {
	if namespace == "" {
		return s.ListAllPods(ctx, clusterName)
	}

	const q = `
		SELECT name, cluster_name, namespace, container_id, image, command, phase, node, pod_ip, restarts, labels, env, ports, created_at, updated_at
		FROM pods WHERE cluster_name = ? AND namespace = ? ORDER BY name`

	return s.scanPods(ctx, q, clusterName, namespace)
}

func (s *Store) ListAllPods(ctx context.Context, clusterName string) ([]storage.PodRecord, error) {
	const q = `
		SELECT name, cluster_name, namespace, container_id, image, command, phase, node, pod_ip, restarts, labels, env, ports, created_at, updated_at
		FROM pods WHERE cluster_name = ? ORDER BY namespace, name`

	return s.scanPods(ctx, q, clusterName)
}

func (s *Store) scanPods(ctx context.Context, query string, args ...any) ([]storage.PodRecord, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var pods []storage.PodRecord
	for rows.Next() {
		var p storage.PodRecord
		if err := rows.Scan(
			&p.Name,
			&p.ClusterName,
			&p.Namespace,
			&p.ContainerID,
			&p.Image,
			&p.Command,
			&p.Phase,
			&p.Node,
			&p.PodIP,
			&p.Restarts,
			&p.Labels,
			&p.Env,
			&p.Ports,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning pod row: %w", err)
		}
		pods = append(pods, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating pod rows: %w", err)
	}
	return pods, nil
}

func (s *Store) DeletePod(ctx context.Context, clusterName, namespace, name string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM pods WHERE cluster_name = ? AND namespace = ? AND name = ?", clusterName, namespace, name)
	if err != nil {
		return fmt.Errorf("deleting pod %q in cluster %q namespace %q: %w", name, clusterName, namespace, err)
	}
	return nil
}

// --- Deployment operations ---

func (s *Store) SaveDeployment(ctx context.Context, dep *storage.DeploymentRecord) error {
	const q = `
		INSERT INTO deployments
			(name, cluster_name, namespace, replicas, ready_replicas, template, labels, selector, phase, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(cluster_name, namespace, name) DO UPDATE SET
			replicas = excluded.replicas,
			ready_replicas = excluded.ready_replicas,
			template = excluded.template,
			labels = excluded.labels,
			selector = excluded.selector,
			phase = excluded.phase,
			updated_at = excluded.updated_at`

	_, err := s.db.ExecContext(ctx, q,
		dep.Name,
		dep.ClusterName,
		dep.Namespace,
		dep.Replicas,
		dep.ReadyReplicas,
		dep.Template,
		dep.Labels,
		dep.Selector,
		dep.Phase,
		dep.CreatedAt,
		dep.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("saving deployment %q in cluster %q namespace %q: %w", dep.Name, dep.ClusterName, dep.Namespace, err)
	}
	return nil
}

func (s *Store) GetDeployment(ctx context.Context, clusterName, namespace, name string) (*storage.DeploymentRecord, error) {
	const q = `
		SELECT name, cluster_name, namespace, replicas, ready_replicas, template, labels, selector, phase, created_at, updated_at
		FROM deployments WHERE cluster_name = ? AND namespace = ? AND name = ?`

	var d storage.DeploymentRecord
	err := s.db.QueryRowContext(ctx, q, clusterName, namespace, name).Scan(
		&d.Name,
		&d.ClusterName,
		&d.Namespace,
		&d.Replicas,
		&d.ReadyReplicas,
		&d.Template,
		&d.Labels,
		&d.Selector,
		&d.Phase,
		&d.CreatedAt,
		&d.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("deployment %q not found in cluster %q namespace %q", name, clusterName, namespace)
	}
	if err != nil {
		return nil, fmt.Errorf("getting deployment %q in cluster %q namespace %q: %w", name, clusterName, namespace, err)
	}
	return &d, nil
}

func (s *Store) ListDeployments(ctx context.Context, clusterName, namespace string) ([]storage.DeploymentRecord, error) {
	var q string
	var args []interface{}
	if namespace == "" {
		q = `SELECT name, cluster_name, namespace, replicas, ready_replicas, template, labels, selector, phase, created_at, updated_at
		FROM deployments WHERE cluster_name = ? ORDER BY namespace, name`
		args = []interface{}{clusterName}
	} else {
		q = `SELECT name, cluster_name, namespace, replicas, ready_replicas, template, labels, selector, phase, created_at, updated_at
		FROM deployments WHERE cluster_name = ? AND namespace = ? ORDER BY name`
		args = []interface{}{clusterName, namespace}
	}

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("listing deployments for cluster %q: %w", clusterName, err)
	}
	defer func() { _ = rows.Close() }()

	var deployments []storage.DeploymentRecord
	for rows.Next() {
		var d storage.DeploymentRecord
		if err := rows.Scan(
			&d.Name,
			&d.ClusterName,
			&d.Namespace,
			&d.Replicas,
			&d.ReadyReplicas,
			&d.Template,
			&d.Labels,
			&d.Selector,
			&d.Phase,
			&d.CreatedAt,
			&d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning deployment row: %w", err)
		}
		deployments = append(deployments, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating deployment rows: %w", err)
	}
	return deployments, nil
}

func (s *Store) DeleteDeployment(ctx context.Context, clusterName, namespace, name string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM deployments WHERE cluster_name = ? AND namespace = ? AND name = ?", clusterName, namespace, name)
	if err != nil {
		return fmt.Errorf("deleting deployment %q in cluster %q namespace %q: %w", name, clusterName, namespace, err)
	}
	return nil
}

// --- Service operations ---

func (s *Store) SaveService(ctx context.Context, svc *storage.ServiceRecord) error {
	const q = `
		INSERT INTO services
			(name, cluster_name, namespace, type, ports, selector, cluster_ip, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(cluster_name, namespace, name) DO UPDATE SET
			type = excluded.type,
			ports = excluded.ports,
			selector = excluded.selector,
			cluster_ip = excluded.cluster_ip,
			updated_at = excluded.updated_at`

	_, err := s.db.ExecContext(ctx, q,
		svc.Name,
		svc.ClusterName,
		svc.Namespace,
		svc.Type,
		svc.Ports,
		svc.Selector,
		svc.ClusterIP,
		svc.CreatedAt,
		svc.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("saving service %q in cluster %q namespace %q: %w", svc.Name, svc.ClusterName, svc.Namespace, err)
	}
	return nil
}

func (s *Store) GetService(ctx context.Context, clusterName, namespace, name string) (*storage.ServiceRecord, error) {
	const q = `
		SELECT name, cluster_name, namespace, type, ports, selector, cluster_ip, created_at, updated_at
		FROM services WHERE cluster_name = ? AND namespace = ? AND name = ?`

	var svc storage.ServiceRecord
	err := s.db.QueryRowContext(ctx, q, clusterName, namespace, name).Scan(
		&svc.Name,
		&svc.ClusterName,
		&svc.Namespace,
		&svc.Type,
		&svc.Ports,
		&svc.Selector,
		&svc.ClusterIP,
		&svc.CreatedAt,
		&svc.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("service %q not found in cluster %q namespace %q", name, clusterName, namespace)
	}
	if err != nil {
		return nil, fmt.Errorf("getting service %q in cluster %q namespace %q: %w", name, clusterName, namespace, err)
	}
	return &svc, nil
}

func (s *Store) ListServices(ctx context.Context, clusterName, namespace string) ([]storage.ServiceRecord, error) {
	var q string
	var args []interface{}
	if namespace == "" {
		q = `SELECT name, cluster_name, namespace, type, ports, selector, cluster_ip, created_at, updated_at
		FROM services WHERE cluster_name = ? ORDER BY namespace, name`
		args = []interface{}{clusterName}
	} else {
		q = `SELECT name, cluster_name, namespace, type, ports, selector, cluster_ip, created_at, updated_at
		FROM services WHERE cluster_name = ? AND namespace = ? ORDER BY name`
		args = []interface{}{clusterName, namespace}
	}

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("listing services for cluster %q: %w", clusterName, err)
	}
	defer func() { _ = rows.Close() }()

	var services []storage.ServiceRecord
	for rows.Next() {
		var svc storage.ServiceRecord
		if err := rows.Scan(
			&svc.Name,
			&svc.ClusterName,
			&svc.Namespace,
			&svc.Type,
			&svc.Ports,
			&svc.Selector,
			&svc.ClusterIP,
			&svc.CreatedAt,
			&svc.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning service row: %w", err)
		}
		services = append(services, svc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating service rows: %w", err)
	}
	return services, nil
}

func (s *Store) DeleteService(ctx context.Context, clusterName, namespace, name string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM services WHERE cluster_name = ? AND namespace = ? AND name = ?", clusterName, namespace, name)
	if err != nil {
		return fmt.Errorf("deleting service %q in cluster %q namespace %q: %w", name, clusterName, namespace, err)
	}
	return nil
}

// --- Event operations ---

func (s *Store) SaveEvent(ctx context.Context, event *storage.EventRecord) error {
	const q = `
		INSERT INTO events
			(cluster_name, namespace, kind, name, type, reason, message, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, q,
		event.ClusterName,
		event.Namespace,
		event.Kind,
		event.Name,
		event.Type,
		event.Reason,
		event.Message,
		event.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("saving event for %s/%s in cluster %q: %w", event.Kind, event.Name, event.ClusterName, err)
	}
	return nil
}

func (s *Store) ListEvents(ctx context.Context, clusterName, namespace string, limit int) ([]storage.EventRecord, error) {
	if namespace == "" {
		const q = `
			SELECT id, cluster_name, namespace, kind, name, type, reason, message, created_at
			FROM events WHERE cluster_name = ?
			ORDER BY created_at DESC LIMIT ?`
		return s.scanEvents(ctx, q, clusterName, limit)
	}
	const q = `
		SELECT id, cluster_name, namespace, kind, name, type, reason, message, created_at
		FROM events WHERE cluster_name = ? AND namespace = ?
		ORDER BY created_at DESC LIMIT ?`
	return s.scanEvents(ctx, q, clusterName, namespace, limit)
}

func (s *Store) ListEventsByResource(ctx context.Context, clusterName, kind, name string, limit int) ([]storage.EventRecord, error) {
	const q = `
		SELECT id, cluster_name, namespace, kind, name, type, reason, message, created_at
		FROM events WHERE cluster_name = ? AND kind = ? AND name = ?
		ORDER BY created_at DESC LIMIT ?`

	return s.scanEvents(ctx, q, clusterName, kind, name, limit)
}

func (s *Store) scanEvents(ctx context.Context, query string, args ...any) ([]storage.EventRecord, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var events []storage.EventRecord
	for rows.Next() {
		var e storage.EventRecord
		if err := rows.Scan(
			&e.ID,
			&e.ClusterName,
			&e.Namespace,
			&e.Kind,
			&e.Name,
			&e.Type,
			&e.Reason,
			&e.Message,
			&e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning event row: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating event rows: %w", err)
	}
	return events, nil
}

// --- Addon operations ---

func (s *Store) SaveAddon(ctx context.Context, addon *storage.AddonRecord) error {
	const q = `
		INSERT INTO addons
			(name, cluster_name, namespace, version, phase, config, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(cluster_name, namespace, name) DO UPDATE SET
			version = excluded.version,
			phase = excluded.phase,
			config = excluded.config,
			updated_at = excluded.updated_at`

	_, err := s.db.ExecContext(ctx, q,
		addon.Name,
		addon.ClusterName,
		addon.Namespace,
		addon.Version,
		addon.Phase,
		addon.Config,
		addon.CreatedAt,
		addon.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("saving addon %q in cluster %q namespace %q: %w", addon.Name, addon.ClusterName, addon.Namespace, err)
	}
	return nil
}

func (s *Store) GetAddon(ctx context.Context, clusterName, namespace, name string) (*storage.AddonRecord, error) {
	const q = `
		SELECT name, cluster_name, namespace, version, phase, config, created_at, updated_at
		FROM addons WHERE cluster_name = ? AND namespace = ? AND name = ?`

	var a storage.AddonRecord
	err := s.db.QueryRowContext(ctx, q, clusterName, namespace, name).Scan(
		&a.Name,
		&a.ClusterName,
		&a.Namespace,
		&a.Version,
		&a.Phase,
		&a.Config,
		&a.CreatedAt,
		&a.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("addon %q not found in cluster %q namespace %q", name, clusterName, namespace)
	}
	if err != nil {
		return nil, fmt.Errorf("getting addon %q in cluster %q namespace %q: %w", name, clusterName, namespace, err)
	}
	return &a, nil
}

func (s *Store) ListAddons(ctx context.Context, clusterName string) ([]storage.AddonRecord, error) {
	const q = `
		SELECT name, cluster_name, namespace, version, phase, config, created_at, updated_at
		FROM addons WHERE cluster_name = ? ORDER BY namespace, name`

	rows, err := s.db.QueryContext(ctx, q, clusterName)
	if err != nil {
		return nil, fmt.Errorf("listing addons for cluster %q: %w", clusterName, err)
	}
	defer func() { _ = rows.Close() }()

	var addons []storage.AddonRecord
	for rows.Next() {
		var a storage.AddonRecord
		if err := rows.Scan(
			&a.Name,
			&a.ClusterName,
			&a.Namespace,
			&a.Version,
			&a.Phase,
			&a.Config,
			&a.CreatedAt,
			&a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning addon row: %w", err)
		}
		addons = append(addons, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating addon rows: %w", err)
	}
	return addons, nil
}

func (s *Store) DeleteAddon(ctx context.Context, clusterName, namespace, name string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM addons WHERE cluster_name = ? AND namespace = ? AND name = ?", clusterName, namespace, name)
	if err != nil {
		return fmt.Errorf("deleting addon %q in cluster %q namespace %q: %w", name, clusterName, namespace, err)
	}
	return nil
}

// --- Pipeline operations ---

func (s *Store) SavePipelineRun(ctx context.Context, run *storage.PipelineRunRecord) error {
	const q = `
		INSERT INTO pipeline_runs
			(id, cluster_name, status, started_at, finished_at, total_targets, passed_targets, failed_targets, file, target_filter)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status = excluded.status,
			finished_at = excluded.finished_at,
			total_targets = excluded.total_targets,
			passed_targets = excluded.passed_targets,
			failed_targets = excluded.failed_targets`

	_, err := s.db.ExecContext(ctx, q,
		run.ID,
		run.ClusterName,
		run.Status,
		run.StartedAt,
		run.FinishedAt,
		run.TotalTargets,
		run.PassedTargets,
		run.FailedTargets,
		run.File,
		run.TargetFilter,
	)
	if err != nil {
		return fmt.Errorf("saving pipeline run %q: %w", run.ID, err)
	}
	return nil
}

func (s *Store) GetPipelineRun(ctx context.Context, id string) (*storage.PipelineRunRecord, error) {
	const q = `
		SELECT id, cluster_name, status, started_at, finished_at, total_targets, passed_targets, failed_targets, file, target_filter
		FROM pipeline_runs WHERE id = ?`

	var r storage.PipelineRunRecord
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&r.ID,
		&r.ClusterName,
		&r.Status,
		&r.StartedAt,
		&r.FinishedAt,
		&r.TotalTargets,
		&r.PassedTargets,
		&r.FailedTargets,
		&r.File,
		&r.TargetFilter,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("pipeline run %q not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("getting pipeline run %q: %w", id, err)
	}
	return &r, nil
}

func (s *Store) ListPipelineRuns(ctx context.Context, limit int) ([]storage.PipelineRunRecord, error) {
	const q = `
		SELECT id, cluster_name, status, started_at, finished_at, total_targets, passed_targets, failed_targets, file, target_filter
		FROM pipeline_runs ORDER BY started_at DESC LIMIT ?`

	rows, err := s.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("listing pipeline runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var runs []storage.PipelineRunRecord
	for rows.Next() {
		var r storage.PipelineRunRecord
		if err := rows.Scan(
			&r.ID,
			&r.ClusterName,
			&r.Status,
			&r.StartedAt,
			&r.FinishedAt,
			&r.TotalTargets,
			&r.PassedTargets,
			&r.FailedTargets,
			&r.File,
			&r.TargetFilter,
		); err != nil {
			return nil, fmt.Errorf("scanning pipeline run row: %w", err)
		}
		runs = append(runs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating pipeline run rows: %w", err)
	}
	return runs, nil
}

func (s *Store) DeletePipelineRun(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM pipeline_runs WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting pipeline run %q: %w", id, err)
	}
	return nil
}

func (s *Store) SavePipelineTarget(ctx context.Context, target *storage.PipelineTargetRecord) error {
	const q = `
		INSERT INTO pipeline_targets
			(run_id, name, status, started_at, finished_at, duration, error, output)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(run_id, name) DO UPDATE SET
			status = excluded.status,
			started_at = excluded.started_at,
			finished_at = excluded.finished_at,
			duration = excluded.duration,
			error = excluded.error,
			output = excluded.output`

	_, err := s.db.ExecContext(ctx, q,
		target.RunID,
		target.Name,
		target.Status,
		target.StartedAt,
		target.FinishedAt,
		target.Duration,
		target.Error,
		target.Output,
	)
	if err != nil {
		return fmt.Errorf("saving pipeline target %q for run %q: %w", target.Name, target.RunID, err)
	}
	return nil
}

func (s *Store) ListPipelineTargets(ctx context.Context, runID string) ([]storage.PipelineTargetRecord, error) {
	const q = `
		SELECT run_id, name, status, started_at, finished_at, duration, error, output
		FROM pipeline_targets WHERE run_id = ? ORDER BY started_at`

	rows, err := s.db.QueryContext(ctx, q, runID)
	if err != nil {
		return nil, fmt.Errorf("listing pipeline targets for run %q: %w", runID, err)
	}
	defer func() { _ = rows.Close() }()

	var targets []storage.PipelineTargetRecord
	for rows.Next() {
		var t storage.PipelineTargetRecord
		if err := rows.Scan(
			&t.RunID,
			&t.Name,
			&t.Status,
			&t.StartedAt,
			&t.FinishedAt,
			&t.Duration,
			&t.Error,
			&t.Output,
		); err != nil {
			return nil, fmt.Errorf("scanning pipeline target row: %w", err)
		}
		targets = append(targets, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating pipeline target rows: %w", err)
	}
	return targets, nil
}

// --- Workflow operations ---

func (s *Store) SaveWorkflowRun(ctx context.Context, run *storage.WorkflowRunRecord) error {
	const q = `
		INSERT INTO workflow_runs
			(id, workflow_file, job, event, status, started_at, finished_at, duration_ms, error, output)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status = excluded.status,
			finished_at = excluded.finished_at,
			duration_ms = excluded.duration_ms,
			error = excluded.error,
			output = excluded.output`

	_, err := s.db.ExecContext(ctx, q,
		run.ID,
		run.WorkflowFile,
		run.Job,
		run.Event,
		run.Status,
		run.StartedAt,
		run.FinishedAt,
		run.DurationMs,
		run.Error,
		run.Output,
	)
	if err != nil {
		return fmt.Errorf("saving workflow run %q: %w", run.ID, err)
	}
	return nil
}

func (s *Store) GetWorkflowRun(ctx context.Context, id string) (*storage.WorkflowRunRecord, error) {
	const q = `
		SELECT id, workflow_file, job, event, status, started_at, finished_at, duration_ms, error, output
		FROM workflow_runs WHERE id = ?`

	var r storage.WorkflowRunRecord
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&r.ID, &r.WorkflowFile, &r.Job, &r.Event, &r.Status,
		&r.StartedAt, &r.FinishedAt, &r.DurationMs, &r.Error, &r.Output,
	)
	if err != nil {
		return nil, fmt.Errorf("workflow run %q not found", id)
	}
	return &r, nil
}

func (s *Store) ListWorkflowRuns(ctx context.Context, limit int) ([]storage.WorkflowRunRecord, error) {
	const q = `
		SELECT id, workflow_file, job, event, status, started_at, finished_at, duration_ms, error, output
		FROM workflow_runs ORDER BY started_at DESC LIMIT ?`

	rows, err := s.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("listing workflow runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var runs []storage.WorkflowRunRecord
	for rows.Next() {
		var r storage.WorkflowRunRecord
		if err := rows.Scan(
			&r.ID, &r.WorkflowFile, &r.Job, &r.Event, &r.Status,
			&r.StartedAt, &r.FinishedAt, &r.DurationMs, &r.Error, &r.Output,
		); err != nil {
			return nil, fmt.Errorf("scanning workflow run row: %w", err)
		}
		runs = append(runs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating workflow run rows: %w", err)
	}
	return runs, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}
