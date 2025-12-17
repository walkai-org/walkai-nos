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

package clusterinfo

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nebuly-ai/nos/pkg/api/nos.nebuly.com/v1alpha1"
	"github.com/nebuly-ai/nos/pkg/gpu/mig"
	nosresource "github.com/nebuly-ai/nos/pkg/resource"
	v1 "k8s.io/api/core/v1"
	kresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_buildGPUInventory(t *testing.T) {
	t.Parallel()
	nodes := []v1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-a",
				Annotations: map[string]string{
					fmt.Sprintf(v1alpha1.AnnotationGpuStatusFormat, 0, "1g.10gb", nosresource.StatusFree): "2",
					fmt.Sprintf(v1alpha1.AnnotationGpuStatusFormat, 0, "1g.10gb", nosresource.StatusUsed): "1",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-b",
				Annotations: map[string]string{
					fmt.Sprintf(v1alpha1.AnnotationGpuStatusFormat, 1, "1g.10gb", nosresource.StatusFree): "1",
					fmt.Sprintf(v1alpha1.AnnotationGpuStatusFormat, 0, "3g.40gb", nosresource.StatusUsed): "1",
				},
			},
		},
	}

	got := buildGPUInventory(nodes, nil)
	want := []GPUInventory{
		{
			Profile:   "1g.10gb",
			Allocated: 1,
			Available: 3,
		},
		{
			Profile:   "3g.40gb",
			Allocated: 1,
			Available: 0,
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("unexpected diff (-want +got):\n%s", diff)
	}
}

func Test_filterCompletedPods(t *testing.T) {
	t.Parallel()
	now := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	terminatedPod := func(name string, phase v1.PodPhase, finishedAt time.Time) v1.Pod {
		return v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Status: v1.PodStatus{
				Phase: phase,
				ContainerStatuses: []v1.ContainerStatus{
					{
						Name: "c",
						State: v1.ContainerState{
							Terminated: &v1.ContainerStateTerminated{
								FinishedAt: metav1.NewTime(finishedAt),
							},
						},
					},
				},
			},
		}
	}
	pods := []v1.Pod{
		terminatedPod("old-succeeded", v1.PodSucceeded, now.Add(-6*time.Minute)),
		terminatedPod("recent-succeeded", v1.PodSucceeded, now.Add(-4*time.Minute)),
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "running",
			},
			Status: v1.PodStatus{
				Phase: v1.PodRunning,
			},
		},
		terminatedPod("old-failed", v1.PodFailed, now.Add(-9*time.Minute)),
	}

	got := filterCompletedPods(pods, now, completedPodRetention)
	gotNames := make([]string, 0, len(got))
	for _, pod := range got {
		gotNames = append(gotNames, pod.Name)
	}
	want := []string{"recent-succeeded", "running"}
	if diff := cmp.Diff(want, gotNames); diff != "" {
		t.Fatalf("unexpected diff (-want +got):\n%s", diff)
	}
}

func Test_buildGPUInventory_FallbackToCapacity(t *testing.T) {
	t.Parallel()
	nodes := []v1.Node{
		{
			Status: v1.NodeStatus{
				Capacity: v1.ResourceList{
					mig.Profile1g10gb.AsResourceName(): kresource.MustParse("3"),
					mig.Profile2g20gb.AsResourceName(): kresource.MustParse("1"),
				},
			},
		},
	}
	pods := []v1.Pod{
		{
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								mig.Profile1g10gb.AsResourceName(): kresource.MustParse("2"),
							},
						},
					},
				},
			},
		},
		{
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								mig.Profile1g10gb.AsResourceName(): kresource.MustParse("1"),
								mig.Profile2g20gb.AsResourceName(): kresource.MustParse("1"),
							},
						},
					},
				},
			},
		},
	}
	got := buildGPUInventory(nodes, pods)
	want := []GPUInventory{
		{
			Profile:   "1g.10gb",
			Allocated: 3,
			Available: 0,
		},
		{
			Profile:   "2g.20gb",
			Allocated: 1,
			Available: 0,
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("unexpected diff (-want +got):\n%s", diff)
	}
}

func Test_buildPodSummaries(t *testing.T) {
	t.Parallel()
	bigStart := time.Date(2023, 1, 1, 15, 4, 5, 0, time.UTC)
	bigStartMeta := metav1.NewTime(bigStart)
	mediumStart := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	mediumStartMeta := metav1.NewTime(mediumStart)
	mediumFinish := mediumStart.Add(30 * time.Minute)
	mediumFinishMeta := metav1.NewTime(mediumFinish)
	pods := []v1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "small-job-1",
				Namespace: "walkai",
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name: "c",
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								mig.Profile1g10gb.AsResourceName(): kresource.MustParse("1"),
							},
						},
					},
				},
			},
			Status: v1.PodStatus{
				Phase: v1.PodPending,
				ContainerStatuses: []v1.ContainerStatus{
					{
						Name: "c",
						State: v1.ContainerState{
							Waiting: &v1.ContainerStateWaiting{
								Reason: "ContainerCreating",
							},
						},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "big-job-1",
				Namespace: "walkai",
			},
			Status: v1.PodStatus{
				Phase:     v1.PodRunning,
				StartTime: &bigStartMeta,
				ContainerStatuses: []v1.ContainerStatus{
					{
						Name: "c",
						State: v1.ContainerState{
							Running: &v1.ContainerStateRunning{},
						},
					},
				},
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name: "c",
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								mig.Profile3g40gb.AsResourceName(): kresource.MustParse("2"),
							},
						},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "medium-job-1",
				Namespace: "walkai",
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name: "c",
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								mig.Profile2g20gb.AsResourceName(): kresource.MustParse("1"),
							},
						},
					},
				},
			},
			Status: v1.PodStatus{
				Phase:     v1.PodSucceeded,
				StartTime: &mediumStartMeta,
				ContainerStatuses: []v1.ContainerStatus{
					{
						Name: "c",
						State: v1.ContainerState{
							Terminated: &v1.ContainerStateTerminated{
								FinishedAt: mediumFinishMeta,
							},
						},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "no-gpu",
				Namespace: "walkai",
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name: "c",
					},
				},
			},
			Status: v1.PodStatus{
				Phase: v1.PodRunning,
			},
		},
	}

	bigStartUTC := bigStart.UTC()
	mediumStartUTC := mediumStart.UTC()
	mediumFinishUTC := mediumFinish.UTC()
	got := buildPodSummaries(pods)
	want := []PodSummary{
		{
			Name:       "big-job-1",
			Namespace:  "walkai",
			Status:     "Running",
			GPU:        "3g.40gb x2",
			StartTime:  &bigStartUTC,
			FinishTime: nil,
		},
		{
			Name:       "medium-job-1",
			Namespace:  "walkai",
			Status:     "Succeeded",
			GPU:        "2g.20gb",
			StartTime:  &mediumStartUTC,
			FinishTime: &mediumFinishUTC,
		},
		{
			Name:       "small-job-1",
			Namespace:  "walkai",
			Status:     "ContainerCreating",
			GPU:        "1g.10gb",
			StartTime:  nil,
			FinishTime: nil,
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("unexpected diff (-want +got):\n%s", diff)
	}
}
