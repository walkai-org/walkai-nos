/*
 * Copyright 2026 nebuly.com
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
	"fmt"
	partitionermig "github.com/nebuly-ai/nos/internal/partitioning/mig"
	"github.com/nebuly-ai/nos/pkg/api/nos.nebuly.com/v1alpha1"
	"github.com/nebuly-ai/nos/pkg/gpu"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type NodeController struct {
	client.Client
	Scheme         *runtime.Scheme
	migInitializer *partitionermig.NodeInitializer
}

func NewNodeController(
	client client.Client,
	scheme *runtime.Scheme,
	migInitializer *partitionermig.NodeInitializer,
) NodeController {
	return NodeController{
		Client:         client,
		Scheme:         scheme,
		migInitializer: migInitializer,
	}
}

//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;patch

func (c *NodeController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var node v1.Node
	if err := c.Client.Get(ctx, client.ObjectKey{Name: req.Name}, &node); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !gpu.IsMigPartitioningEnabled(node) {
		return ctrl.Result{}, nil
	}

	if isNodeInitialized(node) {
		logger.V(1).Info("node already initialized", "node", node.Name)
		return ctrl.Result{}, nil
	}

	if _, err := gpu.GetModel(node); err != nil {
		logger.Info("missing GPU model information, skipping", "node", node.Name, "err", err)
		return ctrl.Result{}, nil
	}
	if _, err := gpu.GetCount(node); err != nil {
		logger.Info("missing GPU count information, skipping", "node", node.Name, "err", err)
		return ctrl.Result{}, nil
	}

	if err := c.migInitializer.InitNodePartitioning(ctx, node); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to initialize MIG partitioning for node %s: %w", node.Name, err)
	}

	logger.Info("initialized MIG partitioning", "node", node.Name)
	return ctrl.Result{}, nil
}

func isNodeInitialized(node v1.Node) bool {
	count, err := gpu.GetCount(node)
	if err != nil {
		return false
	}
	_, specAnnotations := gpu.ParseNodeAnnotations(node)
	return count == len(specAnnotations.GroupByGpuIndex())
}

func (c *NodeController) SetupWithManager(mgr ctrl.Manager, name string) error {
	selectorPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      v1alpha1.LabelGpuPartitioning,
			Operator: metav1.LabelSelectorOpExists,
		}},
	})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.Node{}, builder.WithPredicates(selectorPredicate)).
		WithOptions(controller.Options{MaxConcurrentReconciles: 5}).
		Complete(c)
}
