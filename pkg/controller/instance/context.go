// Copyright 2025 The Kube Resource Orchestrator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package instance

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/kubernetes-sigs/kro/pkg/dynamiccontroller"
	"github.com/kubernetes-sigs/kro/pkg/metadata"
	"github.com/kubernetes-sigs/kro/pkg/requeue"
	"github.com/kubernetes-sigs/kro/pkg/runtime"
)

type ReconcileContext struct {
	Ctx context.Context
	Log logr.Logger

	// ClusterName identifies which cluster this reconciliation is for.
	// Empty string indicates the local/host cluster.
	ClusterName string

	GVR        schema.GroupVersionResource
	Namespaced bool
	Client     dynamic.Interface
	RestMapper meta.RESTMapper

	// ResourceClient / ResourceMapper target the cluster where the
	// instance's child resources are materialized. By default they alias
	// Client / RestMapper (the instance's own cluster), preserving
	// single-cluster behavior. When the controller is configured to
	// deploy to a separate runtime cluster (e.g. kcp instance, kind
	// workloads), the controller overrides these after construction so
	// child apply / prune / external-ref reads hit the runtime cluster
	// while the instance + its status stay on Client.
	ResourceClient dynamic.Interface
	ResourceMapper meta.RESTMapper

	Labeler metadata.Labeler

	Runtime  runtime.Interface
	Instance *unstructured.Unstructured
	Config   ReconcileConfig

	Mark         *ConditionsMarker
	StateManager *StateManager

	// Watcher is the per-instance watch handle from the coordinator.
	// Use dynamiccontroller.NoopInstanceWatcher{} in tests.
	Watcher dynamiccontroller.InstanceWatcher
}

// NewReconcileContext constructs a ReconcileContext for a single reconciliation cycle.
// It bundles all dependencies needed to reconcile an instance's resources:
//   - clusterName: identifies which cluster this reconciliation is for (empty for local)
//   - client/restMapper: for Kubernetes API operations on the target cluster
//   - labeler: for applying kro metadata labels to resources
//   - rt: the runtime containing resolved resource templates, and helpers to figure out
//     readiness, inclusion etc...
//   - instance: the instance CR being reconciled
//
// It also initializes internal state (Mark for conditions, StateManager for node states).
func NewReconcileContext(
	ctx context.Context,
	log logr.Logger,
	clusterName string,
	gvr schema.GroupVersionResource,
	namespaced bool,
	client dynamic.Interface,
	restMapper meta.RESTMapper,
	labeler metadata.Labeler,
	rt runtime.Interface,
	config ReconcileConfig,
	instance *unstructured.Unstructured,
) *ReconcileContext {
	return &ReconcileContext{
		Ctx:          ctx,
		Log:          log,
		ClusterName:  clusterName,
		GVR:          gvr,
		Namespaced:   namespaced,
		Client:       client,
		RestMapper:   restMapper,
		// Default the resource (runtime) clients to the instance's own
		// cluster. The controller overrides these when deploying child
		// resources to a separate runtime cluster.
		ResourceClient: client,
		ResourceMapper: restMapper,
		Labeler:        labeler,
		Runtime:      rt,
		Instance:     instance,
		Config:       config,
		Mark:         NewConditionsMarkerFor(instance),
		StateManager: newStateManager(),
	}
}

func (rcx *ReconcileContext) delayedRequeue(err error) error {
	if rcx.Config.DefaultRequeueDuration == 0 {
		return requeue.None(err)
	}
	return requeue.NeededAfter(err, rcx.Config.DefaultRequeueDuration)
}

func (rcx *ReconcileContext) InstanceClient() dynamic.ResourceInterface {
	base := rcx.Client.Resource(rcx.GVR)
	if rcx.Namespaced {
		return base.Namespace(rcx.Instance.GetNamespace())
	}
	return base
}

// instanceSSAPatch returns a minimal unstructured object for SSA patches
// targeting the instance. For cluster-scoped instances the namespace key is
// omitted so the API server does not receive an empty string.
func instanceSSAPatch(obj *unstructured.Unstructured) *unstructured.Unstructured {
	md := map[string]interface{}{
		"name": obj.GetName(),
	}
	if ns := obj.GetNamespace(); ns != "" {
		md["namespace"] = ns
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": obj.GetAPIVersion(),
			"kind":       obj.GetKind(),
			"metadata":   md,
		},
	}
}
