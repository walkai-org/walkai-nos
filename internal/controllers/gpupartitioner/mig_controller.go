/*
 * Copyright 2023 nebuly.com.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package gpupartitioner

import (
	"context"
	partitionermig "github.com/nebuly-ai/nos/internal/partitioning/mig"
	"github.com/nebuly-ai/nos/pkg/api/nos.nebuly.com/v1alpha1"
	"github.com/nebuly-ai/nos/pkg/constant"
	"github.com/nebuly-ai/nos/pkg/gpu"
	gpumig "github.com/nebuly-ai/nos/pkg/gpu/mig"
	podutil "github.com/nebuly-ai/nos/pkg/util/pod"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	framework "k8s.io/kubernetes/pkg/scheduler/framework"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sort"
	"time"
)

// Controller reacts to pending Pods that request MIG resources and, when none of the MIG-enabled nodes currently
// expose the requested MIG profile, it updates the node partitioning so that the profile becomes available.
type Controller struct {
	client.Client
	Scheme          *runtime.Scheme
	partitioner     *partitionermig.Partitioner
	planIDSource    func() string
	requeueInterval time.Duration
}

func NewController(client client.Client, scheme *runtime.Scheme, requeueInterval time.Duration) *Controller {
	return &Controller{
		Client:          client,
		Scheme:          scheme,
		partitioner:     partitionermig.NewPartitioner(client),
		planIDSource:    partitionermig.NewPartitioningPlanID,
		requeueInterval: requeueInterval,
	}
}

//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;patch

func (c *Controller) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	result := ctrl.Result{RequeueAfter: c.requeueInterval}
	nodes, err := c.listMigPartitionedNodes(ctx)
	if err != nil {
		logger.Error(err, "unable to list MIG partitioned nodes")
		return result, err
	}
	if len(nodes) == 0 {
		logger.V(1).Info("no MIG partitioned nodes found, nothing to do")
		return result, nil
	}

	pendingPods, err := c.listPendingMigPods(ctx)
	if err != nil {
		logger.Error(err, "unable to list pending MIG pods")
		return result, err
	}

	sortedPending := sortPodsByPriorityAndAge(pendingPods)
	requestedForPlan := c.buildRequestedProfilesPlan(nodes, sortedPending)
	if len(requestedForPlan) == 0 {
		logger.V(1).Info("no pending unschedulable MIG pods found, skipping")
		return result, nil
	}

	if c.profileAlreadyPresent(nodes, requestedForPlan) {
		logger.V(1).Info("requested MIG profiles already present in the cluster, skipping", "profiles", requestedForPlan)
		return result, nil
	}

	updated, err := c.tryRepartition(ctx, nodes, requestedForPlan)
	if err != nil {
		logger.Error(err, "failed to update MIG partitioning")
		return result, err
	}
	if !updated {
		logger.Info("unable to find a node that can provide requested MIG profiles", "profiles", requestedForPlan)
	}

	return result, nil
}

func (c *Controller) shouldConsiderPod(pod v1.Pod) bool {
	if !podutil.IsPending(pod) {
		return false
	}
	if podutil.IsScheduled(pod) {
		return false
	}
	if !podutil.IsUnschedulable(pod) {
		return false
	}
	return true
}

func (c *Controller) listMigPartitionedNodes(ctx context.Context) ([]v1.Node, error) {
	var nodeList v1.NodeList
	if err := c.List(ctx, &nodeList, client.MatchingLabels{v1alpha1.LabelGpuPartitioning: gpu.PartitioningKindMig.String()}); err != nil {
		return nil, err
	}
	return nodeList.Items, nil
}

func (c *Controller) listPendingMigPods(ctx context.Context) ([]v1.Pod, error) {
	var podList v1.PodList
	if err := c.List(ctx, &podList); err != nil {
		return nil, err
	}

	pending := make([]v1.Pod, 0, len(podList.Items))
	for _, pod := range podList.Items {
		if !c.shouldConsiderPod(pod) {
			continue
		}
		if len(gpumig.GetRequestedProfiles(pod)) == 0 {
			continue
		}
		pending = append(pending, pod)
	}

	return pending, nil
}

func (c *Controller) buildRequestedProfilesPlan(
	nodes []v1.Node,
	pendingPods []v1.Pod,
) map[gpumig.ProfileName]int {
	requested := make(map[gpumig.ProfileName]int)
	lastFeasible := make(map[gpumig.ProfileName]int)

	for _, pod := range pendingPods {
		addPodRequestedProfiles(requested, pod)

		// If current set is already available, keep going to next pod.
		if c.profileAlreadyPresent(nodes, requested) {
			lastFeasible = copyRequestedProfiles(requested)
			continue
		}

		// If we can repartition for this prefix, keep track and continue to see if we can include more pods.
		if c.canRepartition(nodes, requested) {
			lastFeasible = copyRequestedProfiles(requested)
			continue
		}

		// Cannot satisfy this (higher-priority) pod; stop extending the plan.
		break
	}

	return lastFeasible
}

func (c *Controller) canRepartition(nodes []v1.Node, requested map[gpumig.ProfileName]int) bool {
	if len(requested) == 0 {
		return false
	}

	for _, node := range nodes {
		nodeInfo := framework.NewNodeInfo()
		nodeInfo.SetNode(node.DeepCopy())
		migNode, err := gpumig.NewNode(*nodeInfo)
		if err != nil {
			continue
		}

		requiredSlices := make(map[gpu.Slice]int, len(requested))
		for profile, quantity := range requested {
			requiredSlices[profile] = quantity
		}

		clonedNode, ok := migNode.Clone().(*gpumig.Node)
		if !ok {
			continue
		}

		updated, err := clonedNode.UpdateGeometryFor(requiredSlices)
		if err != nil {
			continue
		}
		if updated && providesRequestedProfiles(*clonedNode, requested) {
			return true
		}
	}

	return false
}

func (c *Controller) profileAlreadyPresent(nodes []v1.Node, requested map[gpumig.ProfileName]int) bool {
	for profile, quantity := range requested {
		if c.profilePresentOnAnyNode(nodes, profile, quantity) {
			continue
		}
		return false
	}
	return true
}

func (c *Controller) profilePresentOnAnyNode(nodes []v1.Node, profile gpumig.ProfileName, quantity int) bool {
	for _, node := range nodes {
		nodeInfo := framework.NewNodeInfo()
		nodeInfo.SetNode(node.DeepCopy())
		migNode, err := gpumig.NewNode(*nodeInfo)
		if err != nil {
			continue
		}
		if migNode.Geometry()[profile] >= quantity {
			return true
		}
	}
	return false
}

func (c *Controller) tryRepartition(
	ctx context.Context,
	nodes []v1.Node,
	requested map[gpumig.ProfileName]int,
) (bool, error) {
	logger := log.FromContext(ctx)

	for _, node := range nodes {
		updated, err := c.updateNodeGeometry(ctx, node, requested)
		if err != nil {
			logger.Error(err, "failed updating node MIG geometry", "node", node.Name)
			continue
		}
		if updated {
			logger.Info("updated MIG partitioning", "node", node.Name)
			return true, nil
		}
	}

	return false, nil
}

func (c *Controller) updateNodeGeometry(
	ctx context.Context,
	node v1.Node,
	requested map[gpumig.ProfileName]int,
) (bool, error) {
	nodeInfo := framework.NewNodeInfo()
	nodeInfo.SetNode(node.DeepCopy())
	migNode, err := gpumig.NewNode(*nodeInfo)
	if err != nil {
		return false, err
	}

	requiredSlices := make(map[gpu.Slice]int, len(requested))
	for profile, quantity := range requested {
		requiredSlices[profile] = quantity
	}

	anyUpdated, err := migNode.UpdateGeometryFor(requiredSlices)
	if err != nil || !anyUpdated {
		return false, err
	}
	if !providesRequestedProfiles(migNode, requested) {
		return false, nil
	}

	partitioning := partitionermig.BuildNodePartitioning(migNode)
	planID := c.planIDSource()

	if err := c.partitioner.ApplyPartitioning(ctx, node, planID, partitioning); err != nil {
		return false, err
	}

	return true, nil
}

func (c *Controller) SetupWithManager(mgr ctrl.Manager, name string) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.Pod{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(c)
}

// InjectFunc allows to override the plan ID generator for tests.
func (c *Controller) InjectFunc(f func() string) {
	if f != nil {
		c.planIDSource = f
	}
}

func addPodRequestedProfiles(target map[gpumig.ProfileName]int, pod v1.Pod) {
	for profile, quantity := range gpumig.GetRequestedProfiles(pod) {
		target[profile] += quantity
	}
}

func copyRequestedProfiles(source map[gpumig.ProfileName]int) map[gpumig.ProfileName]int {
	result := make(map[gpumig.ProfileName]int, len(source))
	for profile, quantity := range source {
		result[profile] = quantity
	}
	return result
}

func providesRequestedProfiles(node gpumig.Node, requested map[gpumig.ProfileName]int) bool {
	geometry := node.Geometry()
	for profile, quantity := range requested {
		if geometry[profile] < quantity {
			return false
		}
	}
	return true
}

func sortPodsByPriorityAndAge(pods []v1.Pod) []v1.Pod {
	sorted := append([]v1.Pod(nil), pods...)
	sort.SliceStable(sorted, func(i, j int) bool {
		p1 := podPriorityOrDefault(sorted[i])
		p2 := podPriorityOrDefault(sorted[j])
		if p1 != p2 {
			return p1 > p2
		}

		t1 := sorted[i].CreationTimestamp.Time
		t2 := sorted[j].CreationTimestamp.Time
		if !t1.Equal(t2) {
			return t1.Before(t2)
		}

		return sorted[i].Name < sorted[j].Name
	})
	return sorted
}

func podPriorityOrDefault(pod v1.Pod) int32 {
	if pod.Spec.Priority != nil {
		return *pod.Spec.Priority
	}
	return constant.DefaultPriorityClassValue
}
