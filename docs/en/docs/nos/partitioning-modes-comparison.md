# Multi-instance GPU (MIG)

Multi-instance GPU (MIG) is a technology available on NVIDIA Ampere or more recent architectures that allows to securely partition a GPU into separate GPU instances for CUDA applications, each fully isolated with its own high-bandwidth memory, cache, and compute cores.

The isolated GPU slices are called MIG devices, and they are named adopting a format that indicates the compute and memory resources of the device. For example, 2g.20gb corresponds to a GPU slice with 20 GB of memory.

MIG does not allow to create GPU slices of custom sizes and quantity, as each GPU model only supports a [specific set of MIG profiles](https://docs.nvidia.com/datacenter/tesla/mig-user-guide/#supported-profiles).
This reduces the degree of granularity with which you can partition the GPUs.
Additionally, the MIG devices must be created respecting certain placement rules, which further limits flexibility of use.

MIG is the GPU sharing approach that offers the highest level of isolation among processes.
However, it lacks in flexibility and it is compatible only with few GPU architectures (Ampere and Hopper).

You can find out more on how MIG technology works in the official [NVIDIA MIG User Guide](https://docs.nvidia.com/datacenter/tesla/mig-user-guide/).