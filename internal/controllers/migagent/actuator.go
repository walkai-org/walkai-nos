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

package migagent

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/nebuly-ai/nos/internal/controllers/migagent/plan"
	"github.com/nebuly-ai/nos/pkg/api/nos.nebuly.com/v1alpha1"
	"github.com/nebuly-ai/nos/pkg/constant"
	"github.com/nebuly-ai/nos/pkg/gpu"
	"github.com/nebuly-ai/nos/pkg/gpu/mig"
	"github.com/nebuly-ai/nos/pkg/util/predicate"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
	"time"
)

type MigActuator struct {
	client.Client
	migClient    mig.Client
	sharedState  *SharedState
	nodeName     string
	devicePlugin gpu.DevicePluginClient

	// lastAppliedPlan is the latest applied plan
	lastAppliedPlan *plan.MigConfigPlan
	// lastAppliedStatus is the MIG status of the GPUs at the time when the latest plan was applied
	lastAppliedStatus *gpu.StatusAnnotationList
}

func NewActuator(client client.Client, migClient mig.Client, sharedState *SharedState, nodeName string) MigActuator {
	return MigActuator{
		Client:       client,
		migClient:    migClient,
		nodeName:     nodeName,
		sharedState:  sharedState,
		devicePlugin: gpu.NewDevicePluginClient(client),
	}
}

func (a *MigActuator) newLogger(ctx context.Context) logr.Logger {
	return log.FromContext(ctx).WithName("Actuator")
}

func (a *MigActuator) updateLastApplied(currentPlan plan.MigConfigPlan, currentStatus gpu.StatusAnnotationList) {
	a.lastAppliedPlan = &currentPlan
	a.lastAppliedStatus = &currentStatus
}

//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;patch

func (a *MigActuator) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := a.newLogger(ctx)

	// If we haven't reported the last applied config, requeue and avoid acquiring lock
	if !a.sharedState.AtLeastOneReportSinceLastApply() {
		logger.Info("last applied config hasn't been reported yet, waiting...")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	a.sharedState.Lock()
	defer a.sharedState.Unlock()

	// Retrieve instance
	var instance v1.Node
	if err := a.Client.Get(ctx, client.ObjectKey{Name: req.Name, Namespace: req.Namespace}, &instance); err != nil {
		return ctrl.Result{}, err
	}

	// Update last parsed plan ID
	a.sharedState.lastParsedPlanId = instance.Annotations[v1alpha1.AnnotationPartitioningPlan]

	// Check if reported status already matches spec
	statusAnnotations, specAnnotations := gpu.ParseNodeAnnotations(instance)
	if mig.SpecMatchesStatus(specAnnotations, statusAnnotations) {
		logger.Info("reported status matches desired MIG config, nothing to do")
		return ctrl.Result{}, nil
	}

	// Compute MIG config plan
	configPlan, err := a.plan(ctx, specAnnotations)
	if err != nil {
		return ctrl.Result{}, err
	}

	// At the end of reconcile, update last applied status information
	defer a.updateLastApplied(configPlan, statusAnnotations)

	// Check if plan has to be applied
	if configPlan.IsEmpty() {
		logger.Info("MIG config plan is empty, nothing to do")
		return ctrl.Result{}, nil
	}
	if configPlan.Equal(a.lastAppliedPlan) && statusAnnotations.Equal(*a.lastAppliedStatus) {
		logger.Info("MIG config plan already applied and state hasn't changed, nothing to do")
		return ctrl.Result{}, nil
	}

	// Apply MIG config plan
	res, err := a.apply(ctx, configPlan)
	a.sharedState.OnApplyDone()

	return res, err
}

func (a *MigActuator) plan(ctx context.Context, specAnnotations gpu.SpecAnnotationList) (plan.MigConfigPlan, error) {
	logger := a.newLogger(ctx)

	// Compute current state
	migDeviceResources, err := a.migClient.GetMigDevices(ctx)
	if gpu.IgnoreNotFound(err) != nil {
		logger.Error(err, "unable to get MIG device resources")
		return plan.MigConfigPlan{}, err
	}
	// If err is not found, restart the NVIDIA device plugin for updating the resources exposed to k8s
	if gpu.IsNotFound(err) {
		logger.Error(err, "unable to get MIG device resources")
		return plan.MigConfigPlan{}, a.restartNvidiaDevicePlugin(ctx)
	}

	state := plan.NewMigState(migDeviceResources)

	// Check if actual state already matches spec
	if state.Matches(specAnnotations) {
		logger.Info("actual state matches desired MIG config")
		return plan.MigConfigPlan{}, nil
	}

	// Compute MIG config plan
	return plan.NewMigConfigPlan(state, specAnnotations), nil
}

func (a *MigActuator) apply(ctx context.Context, plan plan.MigConfigPlan) (ctrl.Result, error) {
	logger := a.newLogger(ctx)
	logger.Info(
		"applying MIG config plan",
		"createOperations",
		plan.CreateOperations,
		"deleteOperations",
		plan.DeleteOperations,
	)

	// If plan contains used resources, clear spec annotations and skip apply so that the partitioner recomputes.
	if containsUsedResources(plan.DeleteOperations) {
		logger.Info("plan contains used MIG resources in delete operations, resetting spec annotations and skipping apply")
		if err := a.resetNodeSpec(ctx); err != nil {
			logger.Error(err, "unable to reset node spec annotations")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	var restartRequired bool
	var atLeastOneErr bool

	// Apply delete operations first
	deletedProfiles := make(mig.ProfileList, 0)
	for _, op := range plan.DeleteOperations {
		status, deleted := a.applyDeleteOp(ctx, op)
		if status.Err != nil {
			logger.Error(status.Err, "unable to fulfill delete operation", "op", op)
			atLeastOneErr = true
		}
		if status.PluginRestartRequired {
			restartRequired = true
		}
		deletedProfiles = append(deletedProfiles, deleted...)
	}

	// Apply create operations
	status := a.applyCreateOps(ctx, plan.CreateOperations)
	if status.Err != nil {
		logger.Error(status.Err, "unable to fulfill create operations")
		if len(deletedProfiles) > 0 {
			a.rollbackDeletedResources(ctx, deletedProfiles)
		}
		atLeastOneErr = true
	}
	if status.PluginRestartRequired {
		restartRequired = true
	}

	// Restart the NVIDIA device plugin if necessary
	if restartRequired {
		if err := a.restartNvidiaDevicePlugin(ctx); err != nil {
			logger.Error(err, "unable to restart nvidia device plugin")
			a.clearRepartitioningTaint(ctx)
			return ctrl.Result{}, err
		}
	}

	// Check if any error happened
	if atLeastOneErr {
		a.clearRepartitioningTaint(ctx)
		return ctrl.Result{}, fmt.Errorf("at least one operation failed while applying desired MIG config")
	}

	return ctrl.Result{}, nil
}

// restartNvidiaDevicePlugin deletes the Nvidia Device Plugin pod and blocks until it is successfully recreated by
// its daemonset
func (a *MigActuator) restartNvidiaDevicePlugin(ctx context.Context) error {
	logger := log.FromContext(ctx)
	logger.Info("restarting NVIDIA device plugin")
	return a.devicePlugin.Restart(ctx, a.nodeName, 1*time.Minute)
}

func (a *MigActuator) applyDeleteOp(ctx context.Context, op plan.DeleteOperation) (plan.OperationStatus, mig.ProfileList) {
	logger := a.newLogger(ctx)
	var restartRequired bool
	deletedProfiles := make(mig.ProfileList, 0)

	// Delete resources choosing from candidates
	var nDeleted int
	var deleteErrors = make(gpu.ErrorList, 0)
	for _, r := range op.Resources {
		if !r.IsFree() {
			err := fmt.Errorf("resource is not free")
			logger.Error(err, "cannot delete MIG resource", "resource", r)
			continue
		}
		err := a.migClient.DeleteMigDevice(ctx, r)
		if gpu.IgnoreNotFound(err) != nil {
			deleteErrors = append(deleteErrors, err)
			logger.Error(err, "unable to delete MIG resource", "resource", r)
			continue
		}
		if gpu.IsNotFound(err) {
			deleteErrors = append(deleteErrors, err)
			logger.Error(err, "unable to delete MIG resource", "resource", r)
			restartRequired = true
			continue
		}
		logger.Info("deleted MIG resource", "resource", r)
		nDeleted++
		deletedProfiles = append(deletedProfiles, mig.Profile{GpuIndex: r.GpuIndex, Name: mig.GetMigProfileName(r)})
	}

	if nDeleted > 0 {
		restartRequired = true
	}

	if len(deleteErrors) > 0 {
		return plan.OperationStatus{
			PluginRestartRequired: restartRequired,
			Err:                   deleteErrors,
		}, deletedProfiles
	}
	return plan.OperationStatus{
		PluginRestartRequired: restartRequired,
		Err:                   nil,
	}, deletedProfiles
}

func (a *MigActuator) applyCreateOps(ctx context.Context, ops plan.CreateOperationList) plan.OperationStatus {
	logger := a.newLogger(ctx)
	logger.Info("applying create operations", "migProfiles", ops)

	profileList := ops.Flatten()
	created, err := a.migClient.CreateMigDevices(ctx, profileList)
	if err != nil {
		nCreated := len(created)
		return plan.OperationStatus{
			PluginRestartRequired: nCreated > 0,
			Err: fmt.Errorf(
				"could create only %d out of %d MIG resources: %s",
				nCreated,
				len(profileList),
				err,
			),
		}
	}
	return plan.OperationStatus{
		PluginRestartRequired: true,
		Err:                   nil,
	}
}

func (a *MigActuator) rollbackDeletedResources(ctx context.Context, profiles mig.ProfileList) {
	if len(profiles) == 0 {
		return
	}
	logger := a.newLogger(ctx)
	logger.Info("rolling back deleted MIG resources", "count", len(profiles))
	if _, err := a.migClient.CreateMigDevices(ctx, profiles); err != nil {
		logger.Error(err, "unable to recreate deleted MIG resources")
	}
}

func (a *MigActuator) SetupWithManager(mgr ctrl.Manager, controllerName string) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(
			&v1.Node{},
			builder.WithPredicates(
				predicate.ExcludeDelete{},
				predicate.MatchingName{Name: a.nodeName},
				predicate.AnnotationsChangedPredicate{},
			),
		).
		Named(controllerName).
		Complete(a)
}

func containsUsedResources(deleteOps plan.DeleteOperationList) bool {
	for _, op := range deleteOps {
		for _, r := range op.Resources {
			if !r.IsFree() {
				return true
			}
		}
	}
	return false
}

// resetNodeSpec clears the desired MIG spec annotations and partitioning plan on the current node so that the
// GPU partitioner recomputes a new plan based on the latest status.
func (a *MigActuator) resetNodeSpec(ctx context.Context) error {
	var node v1.Node
	if err := a.Client.Get(ctx, types.NamespacedName{Name: a.nodeName}, &node); err != nil {
		return err
	}

	updated := node.DeepCopy()
	for k := range updated.Annotations {
		if strings.HasPrefix(k, v1alpha1.AnnotationGpuSpecPrefix) || k == v1alpha1.AnnotationPartitioningPlan {
			delete(updated.Annotations, k)
		}
	}

	return a.Client.Patch(ctx, updated, client.MergeFrom(&node))
}

func (a *MigActuator) clearRepartitioningTaint(ctx context.Context) {
	logger := a.newLogger(ctx)
	var node v1.Node
	if err := a.Client.Get(ctx, types.NamespacedName{Name: a.nodeName}, &node); err != nil {
		logger.Error(err, "unable to fetch node while clearing repartitioning taint")
		return
	}

	original := node.DeepCopy()
	newTaints := make([]v1.Taint, 0, len(node.Spec.Taints))
	var changed bool
	for _, t := range node.Spec.Taints {
		if t.Key == constant.RepartitioningTaintKey {
			changed = true
			continue
		}
		newTaints = append(newTaints, t)
	}

	if !changed {
		return
	}

	node.Spec.Taints = newTaints
	if err := a.Client.Patch(ctx, &node, client.MergeFrom(original)); err != nil {
		logger.Error(err, "unable to clear repartitioning taint")
	}
}
