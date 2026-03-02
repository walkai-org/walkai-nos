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

//go:build !nvml

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

package nvml

import (
	"github.com/go-logr/logr"
	"github.com/nebuly-ai/nos/pkg/gpu"
)

type noopClient struct{}

func NewClient(_ logr.Logger) Client {
	return noopClient{}
}

func (noopClient) GetGpuIndex(string) (int, gpu.Error) {
	return 0, nvmlDisabledError()
}

func (noopClient) GetMigDeviceGpuIndex(string) (int, gpu.Error) {
	return 0, nvmlDisabledError()
}

func (noopClient) DeleteMigDevice(string) gpu.Error {
	return nvmlDisabledError()
}

func (noopClient) CreateMigDevices([]string, int) gpu.Error {
	return nvmlDisabledError()
}

func (noopClient) GetMigEnabledGPUs() ([]int, gpu.Error) {
	return nil, nvmlDisabledError()
}

func (noopClient) DeleteAllMigDevicesExcept([]string) error {
	return nvmlDisabledError()
}

func nvmlDisabledError() gpu.Error {
	return gpu.GenericErr.Errorf("NVML support disabled: rebuild with the 'nvml' build tag")
}
