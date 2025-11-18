---
hide:

- toc

---

# Prerequisites

1. Kubernetes version 1.23 or newer
2. [NVIDIA GPU Operator](#NVIDIA-GPU-Operator)
3. [Cert Manager](https://github.com/cert-manager/cert-manager) (optional, but recommended)

## NVIDIA GPU Operator

Before installing `nos`, you must enable GPU support in your Kubernetes cluster, and we recommend using the NVIDIA GPU Operator for that

You can install the [NVIDIA GPU Operator](https://github.com/NVIDIA/gpu-operator) as follows:

```bash
helm install --wait --generate-name \
     -n gpu-operator --create-namespace \
     nvidia/gpu-operator --version v22.9.0 \
     --set driver.enabled=true \
     --set migManager.enabled=false \
     --set mig.strategy=mixed \
     --set toolkit.enabled=true
```

Note that the GPU Operator will automatically install a recent version of NVIDIA Drivers and CUDA on all the GPU-enabled nodes of your cluster, so you don't have to manually install them.

For further information you can refer to the [NVIDIA GPU Operator Documentation](https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/getting-started.html).
