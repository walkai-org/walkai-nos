# Configuration

You can customize the GPU Partitioner settings by editing the values file of the [nos](../helm-charts/nos/README.md) Helm chart.
In this section we focus on some of the values that you would typically want to customize.

## Reconciliation interval

The GPU partitioner periodically scans all pending, unschedulable pods that request MIG resources and evaluates whether a different partitioning would make them schedulable. You can control how often this happens with:

- `requeueIntervalSeconds`: how often the controller wakes up even if no new Pod events occur.

Shorter intervals react faster to changes (for example when the scheduler no longer emits events) at the cost of more frequent reconciliation cycles. Longer intervals reduce churn but defer partitioning updates.

## Priority classes

By default the chart provisions four PriorityClasses for GPU workloads:

- `nos-priority-low` (value 0, preemption disabled)
- `nos-priority-medium` (value 1000, preemption disabled, global default)
- `nos-priority-high` (value 2000, preemption disabled)
- `nos-priority-extra-high` (value 3000, preemption disabled)

You can disable their creation or override names/values/descriptions with the `priorityClasses` values in the chart.

## Scheduler configuration

The GPU Partitioner no longer embeds or drives an internal Kubernetes scheduler. The value `gpuPartitioner.scheduler.config` is currently a no-op and is kept in the chart only for backward compatibility. Partitioning decisions are made purely from pending Pods and the allowed MIG geometries described below.

## Available MIG geometries

The GPU Partitioner determines the most proper partitioning plan to apply by considering the possible MIG geometries allowed each of the GPU models present in the cluster.

You can set the MIG geometries supported by each GPU model by editing the `gpuPartitioner.knownMigGeometries` value of the [installation chart](../helm-charts/nos/README.md).

You can edit this file to add new MIG geometries for new GPU models, or to edit the existing ones according to your specific needs. For instance, you can remove some MIG geometries if you don't want to allow them to be used for a certain GPU model.

## How it works

The GPU Partitioner watches pending, unschedulable pods that request MIG profiles. It:

- Sorts those pods by priority (higher first, defaulting missing priorities to Medium) and, within the same priority, by creation time (oldest first).
- Walks that ordered list and aggregates the required MIG profiles, stopping when it reaches a higher-priority pod that cannot be satisfied; lower-priority pods are not planned if a higher-priority pod is blocked.
- Skips planning if the requested profiles already exist on any MIG-enabled node.
- Tries MIG-enabled nodes in turn and updates the geometry of the first node where the missing profiles can be provided without deleting in-use devices (respecting allowed geometries per GPU model).

Moreover, just in the case of MIG partitioning, each specific GPU model allows to create only certain combinations of MIG profiles, which are called MIG geometries, so the GPU partitioner takes this constraint into account when trying to find a new partitioning. The available MIG geometries of each GPU model are defined in the field `gpuPartitioner.knownMigGeometries` field of the Helm chart.

### MIG Partitioning

The actual partitioning specified by the GPU Partitioner for MIG GPUs is performed by the MIG Agent, which is a daemonset running on every node labeled with `nos.nebuly.com/gpu-partitioning: mig` that creates/deletes MIG profiles as requested by the GPU Partitioner.

The MIG Agent exposes to the GPU Partitioner the used/free MIG resources of all the GPUs of the node on which it is running through the following node annotations:

- `nos.nebuly.com/status-gpu-<index>-<mig-profile>-free: <quantity>`
- `nos.nebuly.com/status-gpu-<index>-<mig-profile>-used: <quantity>`

The MIG Agent also watches the node's annotations and, every time there desired MIG partitioning specified by the GPU Partitioner does not match the current state, it tries to apply it by creating and deleting the MIG profiles on the target GPUs. The GPU Partitioner specifies the desired MIG geometry of the GPUs of a node through annotations in the following format:

`nos.nebuly.com/spec-gpu-<index>-<mig-profile>: <quantity>`

Note that in some cases the MIG Agent might not be able to apply the desired MIG geometry specified by the GPU Partitioner. This can happen for two reasons:

1. the MIG Agent never deletes MIG resources being in use by a Pod
2. some MIG geometries require the MIG profiles to be created in a certain order, and due to reason (1) the MIG Agent might not be able to delete and re-create the existing MIG profiles in the order required by the new MIG geometry.

In these cases, the MIG Agent tries to apply the desired partitioning and rolls back to its previous state if it cannot complete the change safely.

For further information regarding NVIDIA MIG and its integration with Kubernetes, please refer to the [NVIDIA MIG User Guide](https://docs.nvidia.com/datacenter/tesla/pdf/NVIDIA_MIG_User_Guide.pdf) and to the [MIG Support in Kubernetes](https://docs.nvidia.com/datacenter/cloud-native/kubernetes/mig-k8s.html) official documentation provided by NVIDIA.

### MPS Partitioning

The creation and deletion of MPS resources is handled by the k8s-device-plugin, which can expose a single GPU as multiple MPS resources according to its configuration.

When allocating a container requesting an MPS resource, the device plugin takes care of injecting theenvironment variables and mounting the volumes required by the container to communicate to the MPS server, making sure that the resource limits defined by the device requested by the container are enforced.

For more information about MPS integration with Kubernetes you can refer to the Nebuly [k8s-device-plugin](https://github.com/nebuly-ai/k8s-device-plugin) documentation.
