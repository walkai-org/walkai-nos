# Installation

!!! warning
  
    Before proceeding with `nos` installation, please make sure to meet the requirements 
    described in the [Prerequisites](prerequisites.md) page.

You can install `nos` using Helm 3 (recommended).
You can find all the available configuration values in the Chart [documentation](helm-charts-README.md).

```bash
helm install oci://ghcr.io/walkai-org/helm-charts/nos \                                                                      
  --version 0.0.2 \
  --namespace nos-system \
  --generate-name \
  --create-namespace
```

Alternatively, you can use Kustomize by cloning the repository and running `make deploy`.

