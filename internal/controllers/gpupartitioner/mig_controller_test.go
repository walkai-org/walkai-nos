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

	partitionermig "github.com/nebuly-ai/nos/internal/partitioning/mig"
	"github.com/nebuly-ai/nos/pkg/api/nos.nebuly.com/v1alpha1"
	"github.com/nebuly-ai/nos/pkg/constant"
	"github.com/nebuly-ai/nos/pkg/gpu"
	gpumig "github.com/nebuly-ai/nos/pkg/gpu/mig"
	"github.com/nebuly-ai/nos/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
				gpumig.Profile2g20gb: 1,
				gpumig.Profile3g40gb: 1,
				gpumig.Profile7g79gb: 1,
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
