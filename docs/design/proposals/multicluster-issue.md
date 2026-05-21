# Feature Request: Multicluster Support for KRO

**Problem Statement**:

KRO currently operates on a single Kubernetes cluster. Users who manage multiple clusters must deploy and configure KRO separately on each cluster. This creates operational overhead and prevents centralized management of ResourceGraphDefinitions across a fleet of clusters.

Organizations run workloads across multiple clusters for geographic distribution, environment separation, team isolation, or high availability. These users need a way to define resources once and deploy them across multiple clusters from a single control plane.

**Proposed Solution**:

Add multicluster support to KRO using a hub-spoke model:

- Hub cluster: Runs KRO controller and stores ResourceGraphDefinitions (RGDs)
- Spoke clusters: Receive generated CRDs and run instances with their child resources

Key design points:
- Use `multicluster-runtime` (MCR) library as a drop-in replacement for `controller-runtime`
- Cluster discovery via pluggable providers (starting with kubeconfig secrets)
- RGDs defined only in hub, CRDs and instances live in spokes
- Opt-in via `--enable-multicluster` flag (backward compatible)

**Alternatives Considered**:

- Separate KRO per cluster: Current approach. Simple but creates operational overhead and prevents centralized management. And clusters can't be just controlplanes, and not all clusters can be controlplanes, so this is not a viable long-term solution.
- GitOps-based distribution: Use ArgoCD to deploy RGDs to multiple clusters. Adds complexity and another tool dependency. No unified status visibility. Still required per-cluster KRO instances.

**Additional Context**:

**What is multicluster for KRO?**

> Proposed definition: Multicluster for KRO means the ability to manage ResourceGraphDefinition instances and their child resources across multiple Kubernetes clusters from a single KRO control plane. Keeping RGDs centralized in a hub cluster while distributing CRDs and instances to spoke clusters where workloads run.

Specifically:
- One hub, many spokes: A single KRO controller runs in a hub cluster and connects to multiple spoke clusters
- Definitions stay central: RGDs are authored and stored only in the hub cluster
- Resources are distributed: CRDs and instances live in spoke clusters where workloads run
- Same-cluster locality: All child resources of an instance are created in the same spoke cluster as the instance itself (no cross-cluster dependencies)

What multicluster is **not** for KRO:
- Federation or replication of RGDs across clusters
- Cross-cluster resource references (e.g., a Deployment in cluster-A referencing a ConfigMap in cluster-B)
- Running multiple KRO controllers that coordinate with each other
- Cluster lifecycle management (creating/deleting clusters)

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

Terminology:
- Hub cluster: The cluster running KRO controller and storing RGDs
- Spoke cluster: A remote cluster where CRDs and instances are deployed
- MCR: multicluster-runtime (`sigs.k8s.io/multicluster-runtime`)

Initial scope:
- Kubeconfig provider for cluster discovery
- DynamicController multicluster awareness
- Cluster-aware client factory

Out of scope for initial implementation:
- Cross-cluster resource dependencies
- Cluster API provider
- Per-cluster RGD targeting

---

## Discussion and notes

- Mangirdas Judeikis Proposed this on Feb 18, 2028 via
  in the community meeting & POC work - https://github.com/mjudeikis/kro/tree/mcr.poc

* Please vote on this issue by adding a 👍 [reaction](https://blog.github.com/2016-03-10-add-reactions-to-pull-requests-issues-and-comments/) to the original issue