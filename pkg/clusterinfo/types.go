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

package clusterinfo

import "time"

// Snapshot represents the payload sent to the remote API.
type Snapshot struct {
	Timestamp time.Time      `json:"ts"`
	GPUs      []GPUInventory `json:"gpus"`
	Pods      []PodSummary   `json:"pods"`
}

// GPUInventory contains aggregated information for a specific MIG profile.
type GPUInventory struct {
	Profile   string `json:"gpu"`
	Allocated int    `json:"allocated"`
	Available int    `json:"available"`
}

// PodSummary contains high-level information about a Pod that requests MIG resources.
type PodSummary struct {
	Name       string     `json:"name"`
	Namespace  string     `json:"namespace"`
	Status     string     `json:"status"`
	GPU        string     `json:"gpu"`
	StartTime  *time.Time `json:"start_time"`
	FinishTime *time.Time `json:"finish_time"`
}
