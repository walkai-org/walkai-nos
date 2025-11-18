# nos

`walk:ai nos` is the in-cluster component of the platform. It runs on your Kubernetes cluster and is responsible for two main tasks: dynamically partitioning GPUs so that Pods can share them efficiently, and exporting GPU and workload telemetry that powers the `walk:ai` application.

## Origin and architecture

`walk:ai nos` is a fork of [`nos` by nebuly-ai](https://nebuly-ai.github.io/nos/overview/). We reuse its MIG Agent and GPU Partitioner components (deployed as DaemonSets) and extend them to support dynamic, runtime reconfiguration of MIG based on the current workload. On top of that, we add our own telemetry pipeline that continuously publishes cluster and GPU state to the `walk:ai` backend.


## Dynamic GPU partitioning

`walk:ai nos` allows Pods to request fractions of a physical GPU. Instead of assigning entire devices to a single Pod, GPUs are automatically split into MIG slices that can be requested by individual containers (for example, `1g.10gb`, `3g.40gb`, etc.). This lets multiple Pods share the same card and increases overall utilization.

The partitioning is adjusted in real time:

- `walk:ai` continuously watches pending Pods that request GPU fractions.
- Given the current and pending workload, it computes the best MIG configuration it can apply to fit as many of those Pods as possible.
- The MIG Agent and GPU Partitioner then apply that configuration on the nodes, creating or tearing down MIG instances as needed.

Under the hood, the GPU partitioning is implemented using [NVIDIA Multi-Instance GPU (MIG)](partitioning-modes-comparison.md#multi-instance-gpu-mig) and builds directly on the upstream `nos` MIG Agent and GPU Partitioner.

## Telemetry and cluster visibility

Beyond partitioning GPUs, `walk:ai nos` also acts as the telemetry layer of the platform. The in-cluster agents:

- Collect low-level GPU metrics (utilization, memory usage, MIG layout, allocations per Pod).
- Observe Jobs, Pods and their GPU requests.
- Periodically push a summarized view of this state to the `walk:ai` backend, which stores it in a fast, query-optimized datastore.

The web application then uses this data to render near real-time views of:

- How GPUs and MIG slices are currently allocated and utilized.
- Which Jobs and Pods are running or pending, and why.
- How dynamic partitioning decisions are affecting overall cluster efficiency.

In this way, `walk:ai nos` is the bridge between Kubernetes and the higher-level platform: it enforces GPU partitioning decisions on the nodes, and it exposes a coherent picture of the cluster state so that users can understand and trust how their workloads are being scheduled.
