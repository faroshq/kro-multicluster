# KREP-X: Multicluster Support for KRO

## Terminology

- **Multicluster**: Operating across multiple Kubernetes clusters from a single control plane
- **Hub cluster**: The cluster running KRO controller and storing ResourceGraphDefinitions
- **Spoke cluster**: A remote cluster where CRDs and instances are deployed
- **MCR**: multicluster-runtime, the library KRO uses for multicluster support (`sigs.k8s.io/multicluster-runtime`)

## Problem statement

KRO currently operates on a single Kubernetes cluster. Users who manage multiple clusters must deploy and configure KRO separately on each cluster. This creates operational overhead and prevents centralized management of ResourceGraphDefinitions across a fleet of clusters.

Many organizations run workloads across multiple clusters for reasons like:
- Geographic distribution
- Environment separation (dev, staging, prod)
- Team isolation
- High availability

These users need a way to define resources once and deploy them across multiple clusters from a single control plane, where controlplane is a cluster of its own.

All this should be optional, as many users are happy with single-cluster KRO. Multicluster support should be an add-on feature that can be enabled when needed.

## Proposal

Add multicluster support to KRO using a hub-spoke model.

#### Overview

- **Hub cluster**: Runs KRO controller and stores ResourceGraphDefinitions (RGDs)
- **Spoke clusters**: Receive generated CRDs and run instances with their child resources

RGDs live only in the hub. CRDs and instances live in the spokes.
Limit initial implementation to where only DynamicController is multicluster-aware. Child resources are created in the same cluster as their parent instance.

#### Design details

**Cluster discovery via providers**

MCR uses a provider model for cluster discovery. Providers are pluggable components that watch for cluster registration events and supply connection details to KRO.

Available MCR providers include:
- **kubeconfig** (labeled secrets): Watches secrets with a specific label containing kubeconfig data
- **Cluster API**: Discovers clusters managed by Cluster API
- **kind**: Discovers local kind clusters (for development)
- and more

KRO initially supports the **kubeconfig provider** as it's the simplest and most portable. Other providers can be added later as separate features.

Example cluster secret for kubeconfig provider:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cluster-west
  namespace: kro-system
  labels:
    kro.run/cluster: "true"
type: Opaque
data:
  kubeconfig: <base64-encoded-kubeconfig>
```

**Resource distribution**

- RGDs are defined only in the hub cluster
- CRDs generated from RGDs are installed in spoke clusters (managed by KRO)
- Instances are created in spoke clusters
- Child resources are created in the same spoke cluster as their parent instance

**Architecture**

```
┌─────────────────────────────────────────────────────────┐
│                      Hub Cluster                        │
│  ┌─────────────────┐  ┌────────────────┐  ┌───────────┐ │
│  │      RGDs       │  │ KRO Controller │  │  Cluster  │ │
│  │  (definitions)  │──│ (multicluster) │──│  Secrets  │ │
│  └─────────────────┘  └───────┬────────┘  └───────────┘ │
└───────────────────────────────│─────────────────────────┘
                                │
         ┌──────────────────────┼──────────────────────┐
         │                      │                      │
         ▼                      ▼                      ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  Spoke Cluster  │    │  Spoke Cluster  │    │  Spoke Cluster  │
│  ┌───────────┐  │    │  ┌───────────┐  │    │  ┌───────────┐  │
│  │   CRDs    │  │    │  │   CRDs    │  │    │  │   CRDs    │  │
│  │ Instances │  │    │  │ Instances │  │    │  │ Instances │  │
│  │  + Child  │  │    │  │  + Child  │  │    │  │  + Child  │  │
│  │ Resources │  │    │  │ Resources │  │    │  │ Resources │  │
│  └───────────┘  │    │  └───────────┘  │    │  └───────────┘  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

**Backward compatibility**

Multicluster mode is opt-in via `--enable-multicluster` flag. Without this flag, KRO behaves exactly as before (single-cluster mode).

**Risks**

| Risk | Impact | Mitigation |
|------|--------|------------|
| Cluster connectivity issues | Instances may become unmanageable | Retry logic, circuit breakers, clear status reporting |
| Credential management complexity | Security concerns with storing kubeconfigs | Support for external secret management, credential rotation |
| Performance at scale | High load with many clusters | Lazy client creation, connection pooling |
| Dependency on multicluster-runtime | External library changes may break KRO | Pin to specific version, thorough testing |
| Leader election across clusters | Complexity in HA setups | Single leader for all clusters initially |

## Other solutions considered

**Separate KRO per cluster**: Current approach. Simple but creates operational overhead and prevents centralized management. Especially when managing many clusters.

**GitOps-based distribution**: Use tools like ArgoCD to deploy RGDs to multiple clusters. Adds complexity and another tool dependency. Doesn't provide unified status visibility.

## Scoping

#### What is in scope for this proposal?

- Multicluster manager using multicluster-runtime library as replacement for controller-runtime manager
- Secret-based cluster discovery (kubeconfig provider)
- Per-cluster DynamicController instances
- Cluster-aware client factory
- CLI flags for multicluster configuration
- Documentation and examples

#### What is not in scope?

- Cross-cluster resource dependencies (child resources referencing resources in other clusters)
- Cluster API provider integration
- Cluster sharding across multiple KRO instances
- Per-cluster RGD targeting (all clusters get all RGDs)
- Cluster groups or policies

## Testing strategy

#### Requirements

- Kind clusters for local development
- Multiple cluster setup scripts
- Network connectivity between clusters

#### Test plan

- Unit tests: Mock cluster interfaces for controller testing
- Integration tests: Kind provider with multiple local clusters
- E2E tests: Full multicluster deployment scenarios with cluster secrets

## Discussion and notes

- Initial implementation uses kubeconfig provider; Cluster API provider can be added later
- Future work may include cluster targeting in RGD spec to control which clusters receive which definitions
