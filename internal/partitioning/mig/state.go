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

package mig

import (
	"github.com/nebuly-ai/nos/internal/partitioning/state"
	"github.com/nebuly-ai/nos/pkg/gpu/mig"
	v1 "k8s.io/api/core/v1"
)

func BuildNodePartitioning(node mig.Node) state.NodePartitioning {
	partitioning := state.NodePartitioning{GPUs: make([]state.GPUPartitioning, 0, len(node.GPUs))}

	for _, gpu := range node.GPUs {
		geometry := gpu.GetGeometry()
		resources := make(map[v1.ResourceName]int)
		for slice, quantity := range geometry {
			profile, ok := slice.(mig.ProfileName)
			if !ok || quantity <= 0 {
				continue
			}
			resources[profile.AsResourceName()] = quantity
		}
		partitioning.GPUs = append(partitioning.GPUs, state.GPUPartitioning{
			GPUIndex:  gpu.GetIndex(),
			Resources: resources,
		})
	}

	return partitioning
}
