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
	"github.com/nebuly-ai/nos/internal/controllers/migagent/plan"
	"github.com/nebuly-ai/nos/pkg/api/nos.nebuly.com/v1alpha1"
	"github.com/nebuly-ai/nos/pkg/constant"
	"github.com/nebuly-ai/nos/pkg/gpu"
	"github.com/nebuly-ai/nos/pkg/gpu/mig"
	"github.com/nebuly-ai/nos/pkg/resource"
	migtest "github.com/nebuly-ai/nos/pkg/test/mocks/mig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
	"time"
)

func TestMigActuator_applyDeleteOp(t *testing.T) {
	testCases := []struct {
		name                string
		op                  plan.DeleteOperation
		clientReturnedError gpu.Error

		expectedDeleteCalls uint
		errorExpected       bool
		restartExpected     bool
	}{
		{
			name: "Empty delete operation",
			op: plan.DeleteOperation{
				Resources: make(gpu.DeviceList, 0),
			},
			clientReturnedError: nil,
			expectedDeleteCalls: 0,
			errorExpected:       false,
			restartExpected:     false,
		},
		{
			name: "Delete op success with multiple resources",
			op: plan.DeleteOperation{
				Resources: gpu.DeviceList{
					{
						Device: resource.Device{
							ResourceName: "nvidia.com/mig-1g.10gb",
							DeviceId:     "uid-1",
							Status:       resource.StatusUnknown,
						},
						GpuIndex: 0,
					},
					{
						Device: resource.Device{
							ResourceName: "nvidia.com/mig-1g.10gb",
							DeviceId:     "uid-2",
							Status:       resource.StatusUsed,
						},
						GpuIndex: 0,
					},
					{
						Device: resource.Device{
							ResourceName: "nvidia.com/mig-1g.10gb",
							DeviceId:     "uid-3",
							Status:       resource.StatusFree,
						},
						GpuIndex: 0,
					},
				},
			},
			clientReturnedError: nil,
			expectedDeleteCalls: 1,
			errorExpected:       false,
			restartExpected:     true,
		},
		{
			name: "MIG client returns error",
			op: plan.DeleteOperation{
				Resources: gpu.DeviceList{
					{
						Device: resource.Device{
							ResourceName: "nvidia.com/mig-1g.10gb",
							DeviceId:     "uid-1",
							Status:       resource.StatusFree,
						},
						GpuIndex: 0,
					},
				},
			},
			clientReturnedError: gpu.GenericErr.Errorf("an error"),
			expectedDeleteCalls: 1,
			errorExpected:       true,
			restartExpected:     false,
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			var migClient = migtest.Client{}
			var actuator = MigActuator{migClient: &migClient}
			migClient.ReturnedError = tt.clientReturnedError

			status, deleted := actuator.applyDeleteOp(context.Background(), tt.op)
			if tt.errorExpected {
				assert.Error(t, status.Err)
			}
			if !tt.errorExpected {
				assert.NoError(t, status.Err)
			}
			assert.Equal(t, tt.restartExpected, status.PluginRestartRequired)
			assert.Equal(t, tt.expectedDeleteCalls, migClient.NumCallsDeleteMigResource)
			if tt.clientReturnedError == nil {
				assert.Len(t, deleted, int(tt.expectedDeleteCalls))
			} else {
				assert.Len(t, deleted, 0)
			}
		})
	}
}

type fakeMigClient struct {
	createErrs      []error
	createCallCount int
	deleteCallCount int
}

func (f *fakeMigClient) GetMigDevices(ctx context.Context) (gpu.DeviceList, gpu.Error) {
	return nil, nil
}

func (f *fakeMigClient) GetUsedMigDevices(ctx context.Context) (gpu.DeviceList, gpu.Error) {
	return nil, nil
}

func (f *fakeMigClient) GetAllocatableMigDevices(ctx context.Context) (gpu.DeviceList, gpu.Error) {
	return nil, nil
}

func (f *fakeMigClient) CreateMigDevices(ctx context.Context, profiles mig.ProfileList) (mig.ProfileList, error) {
	f.createCallCount++
	if f.createCallCount <= len(f.createErrs) {
		err := f.createErrs[f.createCallCount-1]
		if err != nil {
			return nil, err
		}
	}
	return profiles, nil
}

func (f *fakeMigClient) DeleteMigDevice(ctx context.Context, device gpu.Device) gpu.Error {
	f.deleteCallCount++
	return nil
}

func (f *fakeMigClient) DeleteAllExcept(ctx context.Context, resources gpu.DeviceList) error {
	return nil
}

type fakeDevicePluginClient struct{}

func (fakeDevicePluginClient) Restart(ctx context.Context, nodeName string, timeout time.Duration) error {
	return nil
}

type fakeDevicePluginClientError struct {
	err error
}

func (f fakeDevicePluginClientError) Restart(ctx context.Context, nodeName string, timeout time.Duration) error {
	return f.err
}

func TestMigActuator_apply_RollbackDeletedResources(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))

	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
		},
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node.DeepCopy()).Build()

	migClient := &fakeMigClient{
		createErrs: []error{
			gpu.GenericErr.Errorf("boom"),
			nil,
		},
	}
	actuator := MigActuator{
		Client:       k8sClient,
		migClient:    migClient,
		devicePlugin: fakeDevicePluginClient{},
		nodeName:     node.Name,
	}
	plan := plan.MigConfigPlan{
		DeleteOperations: plan.DeleteOperationList{
			{
				Resources: gpu.DeviceList{
					{
						Device: resource.Device{
							ResourceName: "nvidia.com/mig-1g.10gb",
							DeviceId:     "uid-1",
							Status:       resource.StatusFree,
						},
						GpuIndex: 0,
					},
				},
			},
		},
		CreateOperations: plan.CreateOperationList{
			{
				MigProfile: mig.Profile{GpuIndex: 0, Name: mig.Profile3g40gb},
				Quantity:   1,
			},
		},
	}

	_, err := actuator.apply(context.Background(), plan)
	assert.Error(t, err)
	assert.Equal(t, 2, migClient.createCallCount)
	assert.Equal(t, 1, migClient.deleteCallCount)
}

func TestMigActuator_apply_ResetSpecOnUsedDelete(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)

	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
			Annotations: map[string]string{
				fmt.Sprintf(v1alpha1.AnnotationGpuSpecFormat, 0, "3g.40gb"): "1",
				v1alpha1.AnnotationPartitioningPlan:                         "plan-123",
			},
		},
	}

	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node).Build()
	migClient := &fakeMigClient{}

	actuator := MigActuator{
		Client:       k8sClient,
		migClient:    migClient,
		devicePlugin: fakeDevicePluginClient{},
		nodeName:     "node-1",
	}

	planWithUsedDelete := plan.MigConfigPlan{
		DeleteOperations: plan.DeleteOperationList{
			{
				Resources: gpu.DeviceList{
					{
						Device: resource.Device{
							ResourceName: "nvidia.com/mig-3g.40gb",
							DeviceId:     "used-3g",
							Status:       resource.StatusUsed,
						},
						GpuIndex: 0,
					},
				},
			},
		},
		CreateOperations: plan.CreateOperationList{
			{
				MigProfile: mig.Profile{GpuIndex: 0, Name: mig.Profile4g40gb},
				Quantity:   1,
			},
		},
	}

	_, err := actuator.apply(context.Background(), planWithUsedDelete)
	assert.NoError(t, err)
	assert.Equal(t, 0, migClient.deleteCallCount)
	assert.Equal(t, 0, migClient.createCallCount)

	var updated v1.Node
	err = k8sClient.Get(context.Background(), client.ObjectKey{Name: "node-1"}, &updated)
	assert.NoError(t, err)
	assert.Empty(t, updated.Annotations[v1alpha1.AnnotationPartitioningPlan])
	for k := range updated.Annotations {
		assert.NotContains(t, k, v1alpha1.AnnotationGpuSpecPrefix)
	}
}

func TestMigActuator_apply_ClearsTaintOnCreateFailure(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))

	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
			Labels: map[string]string{
				v1.LabelHostname: "node-1",
			},
		},
		Spec: v1.NodeSpec{
			Taints: []v1.Taint{{
				Key:    constant.RepartitioningTaintKey,
				Value:  constant.RepartitioningTaintValue,
				Effect: v1.TaintEffectNoSchedule,
			}},
		},
	}

	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node.DeepCopy()).Build()
	migClient := &fakeMigClient{createErrs: []error{fmt.Errorf("boom")}}

	actuator := MigActuator{
		Client:       k8sClient,
		migClient:    migClient,
		devicePlugin: fakeDevicePluginClient{},
		nodeName:     node.Name,
	}

	p := plan.MigConfigPlan{
		CreateOperations: plan.CreateOperationList{
			{
				MigProfile: mig.Profile{GpuIndex: 0, Name: mig.Profile1g10gb},
				Quantity:   1,
			},
		},
	}

	_, err := actuator.apply(context.Background(), p)
	assert.Error(t, err)

	var updated v1.Node
	require.NoError(t, k8sClient.Get(context.Background(), client.ObjectKey{Name: node.Name}, &updated))
	for _, tnt := range updated.Spec.Taints {
		require.NotEqual(t, constant.RepartitioningTaintKey, tnt.Key, "repartitioning taint should be cleared on apply failure")
	}
}

func TestMigActuator_apply_ClearsTaintWhenPluginRestartFails(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))

	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
			Labels: map[string]string{
				v1.LabelHostname: "node-1",
			},
		},
		Spec: v1.NodeSpec{
			Taints: []v1.Taint{{
				Key:    constant.RepartitioningTaintKey,
				Value:  constant.RepartitioningTaintValue,
				Effect: v1.TaintEffectNoSchedule,
			}},
		},
	}

	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node.DeepCopy()).Build()
	migClient := &fakeMigClient{}

	actuator := MigActuator{
		Client:       k8sClient,
		migClient:    migClient,
		devicePlugin: fakeDevicePluginClientError{err: fmt.Errorf("restart failed")},
		nodeName:     node.Name,
	}

	p := plan.MigConfigPlan{
		CreateOperations: plan.CreateOperationList{
			{
				MigProfile: mig.Profile{GpuIndex: 0, Name: mig.Profile1g10gb},
				Quantity:   1,
			},
		},
	}

	_, err := actuator.apply(context.Background(), p)
	assert.Error(t, err)

	var updated v1.Node
	require.NoError(t, k8sClient.Get(context.Background(), client.ObjectKey{Name: node.Name}, &updated))
	for _, tnt := range updated.Spec.Taints {
		require.NotEqual(t, constant.RepartitioningTaintKey, tnt.Key, "repartitioning taint should be cleared on plugin restart failure")
	}
}

//func TestMigActuator_applyCreateOps(t *testing.T) {
//	testCases := []struct {
//		name                string
//		ops                 plan.CreateOperationList
//		clientReturnedError gpu.Error
//
//		expectedCreateCalls uint
//		errorExpected       bool
//		restartExpected     bool
//	}{
//		{
//			name:                "Empty list",
//			ops:                 plan.CreateOperationList{},
//			clientReturnedError: nil,
//			expectedCreateCalls: 0,
//			errorExpected:       false,
//			restartExpected:     false,
//		},
//		{
//			name: "Empty create operation",
//			ops: plan.CreateOperationList{
//				{
//					MigProfile: mig.ProfileName{
//						GpuIndex: 0,
//						GetName:     "1g.10gb",
//					},
//					Quantity: 0,
//				},
//			},
//			clientReturnedError: nil,
//			expectedCreateCalls: 0,
//			errorExpected:       false,
//			restartExpected:     false,
//		},
//		{
//			name: "MIG client returns error",
//			op: plan.CreateOperation{
//				MigProfile: mig.ProfileName{
//					GpuIndex: 0,
//					GetName:     "1g.10gb",
//				},
//				Quantity: 1,
//			},
//			clientReturnedError: gpu.GenericErr.Errorf("an error"),
//			expectedCreateCalls: 1,
//			errorExpected:       true,
//			restartExpected:     false,
//		},
//		{
//			name: "Create success, quantity > 1",
//			op: plan.CreateOperation{
//				MigProfile: mig.ProfileName{
//					GpuIndex: 0,
//					GetName:     "1g.10gb",
//				},
//				Quantity: 4,
//			},
//			clientReturnedError: nil,
//			expectedCreateCalls: 4,
//			errorExpected:       false,
//			restartExpected:     true,
//		},
//	}
//
//	var migClient = migtest.Client{}
//	var actuator = MigActuator{migClient: &migClient}
//
//	for _, tt := range testCases {
//		migClient.Reset()
//		migClient.ReturnedError = tt.clientReturnedError
//		t.Run(tt.name, func(t *testing.T) {
//			status := actuator.applyCreateOps(context.TODO(), tt.op)
//			if tt.errorExpected {
//				assert.Error(t, status.Err)
//			}
//			if !tt.errorExpected {
//				assert.NoError(t, status.Err)
//			}
//			assert.Equal(t, tt.restartExpected, status.PluginRestartRequired)
//			assert.Equal(t, tt.expectedCreateCalls, migClient.NumCallsCreateMigResources)
//		})
//	}
//}
