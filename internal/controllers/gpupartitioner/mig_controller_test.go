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
	"testing"
	"time"

	partitionermig "github.com/nebuly-ai/nos/internal/partitioning/mig"
	"github.com/nebuly-ai/nos/pkg/api/nos.nebuly.com/v1alpha1"
	"github.com/nebuly-ai/nos/pkg/constant"
	"github.com/nebuly-ai/nos/pkg/gpu"
	gpumig "github.com/nebuly-ai/nos/pkg/gpu/mig"
	"github.com/nebuly-ai/nos/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	fakekube "k8s.io/client-go/kubernetes/fake"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestController_updateNodeGeometry(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))

	testCases := []struct {
		name                    string
		nodeFactory             func() *v1.Node
		requested               map[gpumig.ProfileName]int
		planID                  string
		expectUpdated           bool
		expectErr               bool
		expectedSpecAnnotations map[string]string
		expectedPlanAnnotation  string
	}{
		{
			name:        "creates plan for pending request",
			nodeFactory: newA100NodeWithIdle1gAndUsed3g,
			requested: map[gpumig.ProfileName]int{
				gpumig.Profile2g20gb: 1,
			},
			planID:        "test-plan-id",
			expectUpdated: true,
			expectedSpecAnnotations: map[string]string{
				fmt.Sprintf(v1alpha1.AnnotationGpuSpecFormat, 0, gpumig.Profile1g10gb.String()): "1",
				fmt.Sprintf(v1alpha1.AnnotationGpuSpecFormat, 0, gpumig.Profile2g20gb.String()): "1",
				fmt.Sprintf(v1alpha1.AnnotationGpuSpecFormat, 0, gpumig.Profile3g40gb.String()): "1",
			},
			expectedPlanAnnotation: "test-plan-id",
		},
		{
			name:        "creates plan for multiple pending requests",
			nodeFactory: newA100NodeWithIdle7g,
			requested: map[gpumig.ProfileName]int{
				gpumig.Profile4g40gb: 1,
				gpumig.Profile2g20gb: 1,
				gpumig.Profile1g10gb: 1,
			},
			planID:        "test-plan-id",
			expectUpdated: true,
			expectedSpecAnnotations: map[string]string{
				fmt.Sprintf(v1alpha1.AnnotationGpuSpecFormat, 0, gpumig.Profile1g10gb.String()): "1",
				fmt.Sprintf(v1alpha1.AnnotationGpuSpecFormat, 0, gpumig.Profile2g20gb.String()): "1",
				fmt.Sprintf(v1alpha1.AnnotationGpuSpecFormat, 0, gpumig.Profile4g40gb.String()): "1",
			},
			expectedPlanAnnotation: "test-plan-id",
		},
		{
			name:        "returns false when GPU already exposes requested profile",
			nodeFactory: newA100NodeWithIdle1gAndUsed3g,
			requested: map[gpumig.ProfileName]int{
				gpumig.Profile1g10gb: 1,
			},
			planID:        "unused-plan-id",
			expectUpdated: false,
			expectedSpecAnnotations: map[string]string{
				fmt.Sprintf(v1alpha1.AnnotationGpuSpecFormat, 0, gpumig.Profile1g10gb.String()): "4",
				fmt.Sprintf(v1alpha1.AnnotationGpuSpecFormat, 0, gpumig.Profile3g40gb.String()): "1",
			},
			expectedPlanAnnotation: "",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			node := tt.nodeFactory()

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(node.DeepCopy()).
				Build()

			controller := &Controller{
				Client:      client,
				Scheme:      scheme,
				partitioner: partitionermig.NewPartitioner(client),
				planIDSource: func() string {
					return tt.planID
				},
			}

			updated, err := controller.updateNodeGeometry(ctx, *node, tt.requested)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expectUpdated, updated)

			var patchedNode v1.Node
			require.NoError(t, client.Get(ctx, types.NamespacedName{Name: node.Name}, &patchedNode))

			for key, value := range tt.expectedSpecAnnotations {
				assert.Equal(t, value, patchedNode.Annotations[key], "unexpected value for spec annotation %s", key)
			}

			planAnnotation := patchedNode.Annotations[v1alpha1.AnnotationPartitioningPlan]
			if tt.expectedPlanAnnotation == "" {
				assert.Empty(t, planAnnotation)
			} else {
				assert.Equal(t, tt.expectedPlanAnnotation, planAnnotation)
			}
		})
	}
}

func TestController_ReconcileAggregatesPodsAndRequeues(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))

	node := newA100NodeWithIdle7g()
	podWithPresentProfile := newPendingUnschedulableMigPod("pod-with-present-profile", map[v1.ResourceName]string{
		gpumig.Profile1g10gb.AsResourceName(): "1",
	})
	podNeedingNewProfile := newPendingUnschedulableMigPod("pod-needing-new-profile", map[v1.ResourceName]string{
		gpumig.Profile2g20gb.AsResourceName(): "1",
	})

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node.DeepCopy(), podWithPresentProfile, podNeedingNewProfile).
		Build()

	kubeClient := fakekube.NewSimpleClientset(node.DeepCopy(), podWithPresentProfile, podNeedingNewProfile)
	controller := NewController(client, kubeClient, scheme, time.Minute, false)
	controller.InjectFunc(func() string { return "test-plan-id" })

	result, err := controller.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: podWithPresentProfile.Namespace,
			Name:      podWithPresentProfile.Name,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, time.Minute, result.RequeueAfter)

	var patchedNode v1.Node
	require.NoError(t, client.Get(context.Background(), types.NamespacedName{Name: node.Name}, &patchedNode))

	assert.Equal(t, "test-plan-id", patchedNode.Annotations[v1alpha1.AnnotationPartitioningPlan])
	expectedKey := fmt.Sprintf(v1alpha1.AnnotationGpuSpecFormat, 0, gpumig.Profile2g20gb.String())
	assert.NotEmpty(t, patchedNode.Annotations[expectedKey], "expected MIG profile to be added for pending pods")
}

func TestController_ReconcileSkipsWhenProfilesAlreadyPresent(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))

	node := newA100NodeWithIdle1gAndUsed3g()
	pod := newPendingUnschedulableMigPod("pod-needing-existing-profile", map[v1.ResourceName]string{
		gpumig.Profile1g10gb.AsResourceName(): "1",
	})

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node.DeepCopy(), pod).
		Build()

	kubeClient := fakekube.NewSimpleClientset(node.DeepCopy(), pod)
	controller := NewController(client, kubeClient, scheme, 30*time.Second, false)
	controller.InjectFunc(func() string {
		t.Fatal("planIDSource should not be called when profiles are already present")
		return ""
	})

	result, err := controller.Reconcile(context.Background(), ctrl.Request{})
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, result.RequeueAfter)

	var patchedNode v1.Node
	require.NoError(t, client.Get(context.Background(), types.NamespacedName{Name: node.Name}, &patchedNode))
	assert.Empty(t, patchedNode.Annotations[v1alpha1.AnnotationPartitioningPlan], "plan should not be set when profiles are already available")
}

func TestController_ReconcileRemovesRepartitioningTaintWhenProfilesAvailable(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))
	require.NoError(t, policyv1.AddToScheme(scheme))

	node := newA100NodeWithIdle1gAndUsed3g()
	node.Spec.Taints = []v1.Taint{{
		Key:    constant.RepartitioningTaintKey,
		Value:  constant.RepartitioningTaintValue,
		Effect: v1.TaintEffectNoSchedule,
	}}
	pod := newPendingUnschedulableMigPod("pod-needing-existing-profile", map[v1.ResourceName]string{
		gpumig.Profile1g10gb.AsResourceName(): "1",
	})

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node.DeepCopy(), pod).
		Build()

	kubeClient := fakekube.NewSimpleClientset(node.DeepCopy(), pod)
	controller := NewController(client, kubeClient, scheme, 30*time.Second, false)

	_, err := controller.Reconcile(context.Background(), ctrl.Request{})
	require.NoError(t, err)

	var patchedNode v1.Node
	require.NoError(t, client.Get(context.Background(), types.NamespacedName{Name: node.Name}, &patchedNode))

	assert.Empty(t, patchedNode.Spec.Taints, "repartitioning taint should be cleared once requested profiles are already present")
	assert.Empty(t, patchedNode.Annotations[v1alpha1.AnnotationPartitioningPlan], "plan should not be set when profiles are already available")
}

func TestController_ReconcileClearsTaintWhenHeadPodSatisfiedButLowerPending(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))
	require.NoError(t, policyv1.AddToScheme(scheme))

	node := newA100NodeWith3211AndUsed2g()
	node.Spec.Taints = []v1.Taint{{
		Key:    constant.RepartitioningTaintKey,
		Value:  constant.RepartitioningTaintValue,
		Effect: v1.TaintEffectNoSchedule,
	}}

	high := newPendingUnschedulableMigPod("high-priority-3g", map[v1.ResourceName]string{
		gpumig.Profile3g40gb.AsResourceName(): "1",
	})
	highPriority := int32(2000)
	high.Spec.Priority = &highPriority
	high.CreationTimestamp = metav1.NewTime(time.Now().Add(-time.Minute))

	low := newPendingUnschedulableMigPod("low-priority-4g", map[v1.ResourceName]string{
		gpumig.Profile4g40gb.AsResourceName(): "1",
	})
	lowPriority := int32(0)
	low.Spec.Priority = &lowPriority
	low.CreationTimestamp = metav1.NewTime(time.Now())

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node.DeepCopy(), high, low).
		Build()
	kubeClient := fakekube.NewSimpleClientset(node.DeepCopy(), high, low)

	controller := NewController(client, kubeClient, scheme, 30*time.Second, false)

	_, err := controller.Reconcile(context.Background(), ctrl.Request{})
	require.NoError(t, err)

	var patchedNode v1.Node
	require.NoError(t, client.Get(context.Background(), types.NamespacedName{Name: node.Name}, &patchedNode))

	assert.Empty(t, patchedNode.Spec.Taints, "repartitioning taint should be cleared once the highest-priority feasible pod is satisfiable")
	assert.Empty(t, patchedNode.Annotations[v1alpha1.AnnotationPartitioningPlan], "plan should not be set when profiles are already available for the head pod")
}

func TestController_ReconcilePlansOnceResourcesAreFreed(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))
	require.NoError(t, policyv1.AddToScheme(scheme))

	node := newA100NodeWithUsed7g()
	pod := newPendingUnschedulableMigPod("pod-needing-1g", map[v1.ResourceName]string{
		gpumig.Profile1g10gb.AsResourceName(): "1",
	})

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node.DeepCopy(), pod).
		Build()

	kubeClient := fakekube.NewSimpleClientset(node.DeepCopy(), pod)
	controller := NewController(client, kubeClient, scheme, 5*time.Second, false)
	controller.InjectFunc(func() string { return "freed-plan-id" })

	// First reconcile: 7g slice is in use, so repartition cannot happen.
	_, err := controller.Reconcile(context.Background(), ctrl.Request{})
	require.NoError(t, err)

	var patchedNode v1.Node
	require.NoError(t, client.Get(context.Background(), types.NamespacedName{Name: node.Name}, &patchedNode))
	assert.Empty(t, patchedNode.Annotations[v1alpha1.AnnotationPartitioningPlan], "plan should not be set while 7g slice is used")

	// Simulate workload completion: 7g slice becomes free.
	patchedNode.Annotations[fmt.Sprintf(v1alpha1.AnnotationGpuStatusFormat, 0, gpumig.Profile7g79gb.String(), resource.StatusUsed)] = "0"
	patchedNode.Annotations[fmt.Sprintf(v1alpha1.AnnotationGpuStatusFormat, 0, gpumig.Profile7g79gb.String(), resource.StatusFree)] = "1"
	require.NoError(t, client.Update(context.Background(), &patchedNode))

	// Second reconcile should now plan a repartition to expose 1g profiles.
	_, err = controller.Reconcile(context.Background(), ctrl.Request{})
	require.NoError(t, err)

	require.NoError(t, client.Get(context.Background(), types.NamespacedName{Name: node.Name}, &patchedNode))
	assert.Equal(t, "freed-plan-id", patchedNode.Annotations[v1alpha1.AnnotationPartitioningPlan])
	expectedKey := fmt.Sprintf(v1alpha1.AnnotationGpuSpecFormat, 0, gpumig.Profile1g10gb.String())
	assert.NotEmpty(t, patchedNode.Annotations[expectedKey], "expected 1g profile to be planned after resources are freed")
}

func TestController_ReconcileSkipsLowerPriorityWhenHigherCannotBeSatisfied(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))
	require.NoError(t, policyv1.AddToScheme(scheme))

	node := newA30NodeWithoutMigDevices()
	extraHigh := int32(3000)
	medium := int32(1000)

	highPriorityPod := newPendingUnschedulableMigPod("high-priority-unsupported", map[v1.ResourceName]string{
		gpumig.Profile7g79gb.AsResourceName(): "1",
	})
	highPriorityPod.Spec.Priority = &extraHigh
	highPriorityPod.CreationTimestamp = metav1.NewTime(time.Now().Add(-time.Minute))

	lowPriorityPod := newPendingUnschedulableMigPod("low-priority-supported", map[v1.ResourceName]string{
		gpumig.Profile1g6gb.AsResourceName(): "1",
	})
	lowPriorityPod.Spec.Priority = &medium
	lowPriorityPod.CreationTimestamp = metav1.NewTime(time.Now())

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node.DeepCopy(), highPriorityPod, lowPriorityPod).
		Build()

	kubeClient := fakekube.NewSimpleClientset(node.DeepCopy(), highPriorityPod, lowPriorityPod)
	controller := NewController(client, kubeClient, scheme, time.Minute, false)
	controller.InjectFunc(func() string { return "plan-id-should-not-be-set" })

	_, err := controller.Reconcile(context.Background(), ctrl.Request{})
	require.NoError(t, err)

	var patchedNode v1.Node
	require.NoError(t, client.Get(context.Background(), types.NamespacedName{Name: node.Name}, &patchedNode))
	assert.Empty(t, patchedNode.Annotations[v1alpha1.AnnotationPartitioningPlan], "plan should not be created when higher priority pod cannot be satisfied")
}

func TestController_ReconcileOrdersByAgeWithinPriority(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))
	require.NoError(t, policyv1.AddToScheme(scheme))

	node := newA30NodeWithoutMigDevices()
	medium := int32(1000)

	olderPod := newPendingUnschedulableMigPod("older-pod", map[v1.ResourceName]string{
		gpumig.Profile4g24gb.AsResourceName(): "1",
	})
	olderPod.Spec.Priority = &medium
	olderPod.CreationTimestamp = metav1.NewTime(time.Now().Add(-2 * time.Minute))

	youngerPod := newPendingUnschedulableMigPod("younger-pod", map[v1.ResourceName]string{
		gpumig.Profile7g79gb.AsResourceName(): "1",
	})
	youngerPod.Spec.Priority = &medium
	youngerPod.CreationTimestamp = metav1.NewTime(time.Now())

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node.DeepCopy(), youngerPod, olderPod).
		Build()

	kubeClient := fakekube.NewSimpleClientset(node.DeepCopy(), youngerPod, olderPod)
	controller := NewController(client, kubeClient, scheme, time.Minute, false)
	controller.InjectFunc(func() string { return "age-ordered-plan" })

	_, err := controller.Reconcile(context.Background(), ctrl.Request{})
	require.NoError(t, err)

	var patchedNode v1.Node
	require.NoError(t, client.Get(context.Background(), types.NamespacedName{Name: node.Name}, &patchedNode))
	assert.Equal(t, "age-ordered-plan", patchedNode.Annotations[v1alpha1.AnnotationPartitioningPlan])
	expectedKey := fmt.Sprintf(v1alpha1.AnnotationGpuSpecFormat, 0, gpumig.Profile4g24gb.String())
	assert.NotEmpty(t, patchedNode.Annotations[expectedKey], "expected plan to prioritize older pod within the same priority class")
}

func TestController_ReconcilePreemptsLowerPriorityWhenProfileIsFullyUsed(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))
	require.NoError(t, policyv1.AddToScheme(scheme))

	node := newA100NodeWithUsed7g()

	lowPriority := int32(1000)
	highPriority := int32(2000)

	running := newRunningMigPodOnNode("running-7g", node.Name, map[v1.ResourceName]string{
		gpumig.Profile7g79gb.AsResourceName(): "1",
	}, lowPriority)

	pending := newPendingUnschedulableMigPod("pending-7g-high", map[v1.ResourceName]string{
		gpumig.Profile7g79gb.AsResourceName(): "1",
	})
	pending.Spec.Priority = &highPriority

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node.DeepCopy(), running, pending).
		Build()

	kubeClient := fakekube.NewSimpleClientset(node.DeepCopy(), running, pending)
	controller := NewController(client, kubeClient, scheme, 30*time.Second, true)

	evicted := false
	controller.evictFunc = func(_ context.Context, pods []v1.Pod) error {
		if len(pods) > 0 && pods[0].Name == running.Name {
			evicted = true
		}
		return nil
	}

	_, err := controller.Reconcile(context.Background(), ctrl.Request{})
	require.NoError(t, err)

	var taintedNode v1.Node
	require.NoError(t, client.Get(context.Background(), types.NamespacedName{Name: node.Name}, &taintedNode))
	require.True(t, nodeHasRepartitioningTaint(taintedNode), "expected repartitioning taint during preemption")

	require.True(t, evicted, "expected eviction to be issued")
}

func TestController_ReconcileRecreatesSpecWhenMissing(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))

	node := newA100NodeStatusOnly7g()
	pod := newPendingUnschedulableMigPod("pod-needing-2g", map[v1.ResourceName]string{
		gpumig.Profile2g20gb.AsResourceName(): "1",
	})

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node.DeepCopy(), pod).
		Build()

	kubeClient := fakekube.NewSimpleClientset(node.DeepCopy(), pod)
	controller := NewController(client, kubeClient, scheme, 30*time.Second, false)
	controller.InjectFunc(func() string { return "recreated-plan-id" })

	_, err := controller.Reconcile(context.Background(), ctrl.Request{})
	require.NoError(t, err)

	var patchedNode v1.Node
	require.NoError(t, client.Get(context.Background(), types.NamespacedName{Name: node.Name}, &patchedNode))

	assert.Equal(t, "recreated-plan-id", patchedNode.Annotations[v1alpha1.AnnotationPartitioningPlan])
	specKey := fmt.Sprintf(v1alpha1.AnnotationGpuSpecFormat, 0, gpumig.Profile2g20gb.String())
	assert.Equal(t, "1", patchedNode.Annotations[specKey], "expected spec annotation for requested profile to be recreated")
}

func newA100NodeWithIdle7g() *v1.Node {
	annotations := map[string]string{
		fmt.Sprintf(v1alpha1.AnnotationGpuStatusFormat, 0, gpumig.Profile1g10gb.String(), resource.StatusFree): "7",
		fmt.Sprintf(v1alpha1.AnnotationGpuSpecFormat, 0, gpumig.Profile1g10gb.String()):                        "7",
	}

	return &v1.Node{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Node",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-a100",
			Labels: map[string]string{
				constant.LabelNvidiaProduct:   gpu.GPUModel_A100_PCIe_80GB.String(),
				constant.LabelNvidiaCount:     "1",
				v1alpha1.LabelGpuPartitioning: gpu.PartitioningKindMig.String(),
			},
			Annotations: annotations,
		},
	}
}

func newA100NodeWithUsed7g() *v1.Node {
	annotations := map[string]string{
		fmt.Sprintf(v1alpha1.AnnotationGpuStatusFormat, 0, gpumig.Profile7g79gb.String(), resource.StatusUsed): "1",
		fmt.Sprintf(v1alpha1.AnnotationGpuSpecFormat, 0, gpumig.Profile7g79gb.String()):                        "1",
	}

	return &v1.Node{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Node",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-a100-used-7g",
			Labels: map[string]string{
				constant.LabelNvidiaProduct:   gpu.GPUModel_A100_PCIe_80GB.String(),
				constant.LabelNvidiaCount:     "1",
				v1alpha1.LabelGpuPartitioning: gpu.PartitioningKindMig.String(),
			},
			Annotations: annotations,
		},
	}
}

func newA100NodeStatusOnly7g() *v1.Node {
	annotations := map[string]string{
		fmt.Sprintf(v1alpha1.AnnotationGpuStatusFormat, 0, gpumig.Profile7g79gb.String(), resource.StatusFree): "1",
	}

	return &v1.Node{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Node",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-a100-status-only",
			Labels: map[string]string{
				constant.LabelNvidiaProduct:   gpu.GPUModel_A100_PCIe_80GB.String(),
				constant.LabelNvidiaCount:     "1",
				v1alpha1.LabelGpuPartitioning: gpu.PartitioningKindMig.String(),
			},
			Annotations: annotations,
		},
	}
}

func newPendingUnschedulableMigPod(name string, requests map[v1.ResourceName]string) *v1.Pod {
	resources := make(v1.ResourceList, len(requests))
	for resourceName, quantity := range requests {
		resources[resourceName] = k8sresource.MustParse(quantity)
	}

	return &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:  "test",
				Image: "busybox",
				Resources: v1.ResourceRequirements{
					Requests: resources,
					Limits:   resources,
				},
			}},
		},
		Status: v1.PodStatus{
			Phase: v1.PodPending,
			Conditions: []v1.PodCondition{{
				Type:   v1.PodScheduled,
				Status: v1.ConditionFalse,
				Reason: v1.PodReasonUnschedulable,
			}},
		},
	}
}

func newRunningMigPodOnNode(name, nodeName string, requests map[v1.ResourceName]string, priority int32) *v1.Pod {
	resources := make(v1.ResourceList, len(requests))
	for resourceName, quantity := range requests {
		resources[resourceName] = k8sresource.MustParse(quantity)
	}

	return &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			NodeName: nodeName,
			Priority: &priority,
			Containers: []v1.Container{{
				Name:  "test",
				Image: "busybox",
				Resources: v1.ResourceRequirements{
					Requests: resources,
					Limits:   resources,
				},
			}},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}
}

func newA100NodeWithIdle1gAndUsed3g() *v1.Node {
	annotations := map[string]string{
		fmt.Sprintf(v1alpha1.AnnotationGpuStatusFormat, 0, gpumig.Profile1g10gb.String(), resource.StatusFree): "4",
		fmt.Sprintf(v1alpha1.AnnotationGpuStatusFormat, 0, gpumig.Profile3g40gb.String(), resource.StatusUsed): "1",
		fmt.Sprintf(v1alpha1.AnnotationGpuSpecFormat, 0, gpumig.Profile1g10gb.String()):                        "4",
		fmt.Sprintf(v1alpha1.AnnotationGpuSpecFormat, 0, gpumig.Profile3g40gb.String()):                        "1",
	}

	return &v1.Node{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Node",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-a100",
			Labels: map[string]string{
				constant.LabelNvidiaProduct:   gpu.GPUModel_A100_PCIe_80GB.String(),
				constant.LabelNvidiaCount:     "1",
				v1alpha1.LabelGpuPartitioning: gpu.PartitioningKindMig.String(),
			},
			Annotations: annotations,
		},
	}
}

func newA100NodeWith3211AndUsed2g() *v1.Node {
	annotations := map[string]string{
		fmt.Sprintf(v1alpha1.AnnotationGpuStatusFormat, 0, gpumig.Profile3g40gb.String(), resource.StatusFree): "1",
		fmt.Sprintf(v1alpha1.AnnotationGpuStatusFormat, 0, gpumig.Profile2g20gb.String(), resource.StatusUsed): "1",
		fmt.Sprintf(v1alpha1.AnnotationGpuStatusFormat, 0, gpumig.Profile1g10gb.String(), resource.StatusFree): "2",
		fmt.Sprintf(v1alpha1.AnnotationGpuSpecFormat, 0, gpumig.Profile3g40gb.String()):                        "1",
		fmt.Sprintf(v1alpha1.AnnotationGpuSpecFormat, 0, gpumig.Profile2g20gb.String()):                        "1",
		fmt.Sprintf(v1alpha1.AnnotationGpuSpecFormat, 0, gpumig.Profile1g10gb.String()):                        "2",
	}

	return &v1.Node{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Node",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-a100-3211-used-2g",
			Labels: map[string]string{
				constant.LabelNvidiaProduct:   gpu.GPUModel_A100_PCIe_80GB.String(),
				constant.LabelNvidiaCount:     "1",
				v1alpha1.LabelGpuPartitioning: gpu.PartitioningKindMig.String(),
			},
			Annotations: annotations,
		},
	}
}

func newA30NodeWithoutMigDevices() *v1.Node {
	return &v1.Node{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Node",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-a30",
			Labels: map[string]string{
				constant.LabelNvidiaProduct:   gpu.GPUModel_A30.String(),
				constant.LabelNvidiaCount:     "1",
				v1alpha1.LabelGpuPartitioning: gpu.PartitioningKindMig.String(),
			},
			Annotations: map[string]string{},
		},
	}
}
