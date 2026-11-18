#!/bin/bash
set -e

echo "[torresmo-node] Starting node..."
echo "[torresmo-node] Role: ${TORRESMO_ROLE:-worker}"
echo "[torresmo-node] Cluster: ${TORRESMO_CLUSTER:-unknown}"

# Start Docker daemon in background
# The DinD image provides dockerd-entrypoint.sh
nohup dockerd-entrypoint.sh dockerd \
    --storage-driver=overlay2 \
    --host=unix:///var/run/docker.sock \
    > /var/log/dockerd.log 2>&1 &

# Wait for Docker daemon to be ready
echo "[torresmo-node] Waiting for Docker daemon..."
MAX_WAIT=30
for i in $(seq 1 $MAX_WAIT); do
    if docker info > /dev/null 2>&1; then
        echo "[torresmo-node] Docker daemon is ready"
        break
    fi
    if [ "$i" -eq "$MAX_WAIT" ]; then
        echo "[torresmo-node] ERROR: Docker daemon failed to start"
        cat /var/log/dockerd.log
        exit 1
    fi
    sleep 1
done

# Report node info
echo "[torresmo-node] Docker version: $(docker version --format '{{.Server.Version}}' 2>/dev/null || echo 'unknown')"
echo "[torresmo-node] Node IP: $(hostname -i 2>/dev/null || echo 'unknown')"
echo "[torresmo-node] Node ready!"

# Keep the container running
# The Torresmo control plane will exec commands into this container
# to manage pods (docker run inside this DinD node)
exec tail -f /var/log/dockerd.log
