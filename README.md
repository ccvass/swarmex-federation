# Swarmex Federation

Multi-cluster service replication across clouds for Docker Swarm.

Part of [Swarmex](https://github.com/ccvass/swarmex) — enterprise-grade orchestration for Docker Swarm.

## What It Does

Replicates services across multiple Docker Swarm clusters in different clouds. When a service is updated in the source cluster, the change is automatically synced to all configured target clusters, enabling multi-cloud redundancy.

## Labels

```yaml
deploy:
  labels:
    swarmex.federation.replicate: "true"     # Enable cross-cluster replication
    swarmex.federation.clusters: "gcp,azure" # Target clusters to replicate to
```

Environment variables configure cluster connections:

```bash
FEDERATION_CLUSTER_GCP=tcp://10.0.1.5:2376
FEDERATION_CLUSTER_AZURE=tcp://10.0.2.5:2376
```

## How It Works

1. Watches for services with federation labels in the local cluster.
2. On service create or update, connects to each target cluster via Docker API.
3. Creates or updates the service in remote clusters with matching spec.
4. Syncs image updates across all federated clusters.
5. Handles cluster connectivity failures gracefully with retry logic.

## Quick Start

```bash
docker service update \
  --label-add swarmex.federation.replicate=true \
  --label-add swarmex.federation.clusters=gcp \
  my-app
```

## Verified

AWS→GCP cross-cloud replication confirmed. Image update synced to remote cluster.

## License

Apache-2.0
