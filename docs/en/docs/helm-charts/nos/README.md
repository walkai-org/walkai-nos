# nos

![Version: 0.1.2](https://img.shields.io/badge/Version-0.1.2-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.1.2](https://img.shields.io/badge/AppVersion-0.1.2-informational?style=flat-square)

The open-source platform for running AI workloads on k8s in an optimized way, both in terms of hardware utilization and workload performance.

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Michele Zanotti | <m.zanotti@nebuly.com> | <github.com/nebuly-ai> |
| Diego Fiori | <d.fiori@nebuly.com> | <github.com/diegofiori> |

## Source Code

* <https://github.com/nebuly-ai/nos>

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| allowDefaultNamespace | bool | `false` | If true allows to deploy `nos` chart in the `default` namespace |
| clusterInfoExporter.affinity | object | `{}` | Affinity rules for the Cluster Info Exporter Pods. |
| clusterInfoExporter.apiClient | object | - | Configuration of the API client namespace, ServiceAccount, RBAC settings and token Secret. |
| clusterInfoExporter.apiClient.adminClusterRoleName | string | `"admin"` | ClusterRole granted by the admin RoleBinding. |
| clusterInfoExporter.apiClient.adminRoleBindingName | string | `"walkai-api-client-admin"` | Name of the RoleBinding that grants the admin ClusterRole inside the API client namespace. |
| clusterInfoExporter.apiClient.createNamespace | bool | `true` | Whether the chart should create the API client namespace. |
| clusterInfoExporter.apiClient.discoveryClusterRoleBindingName | string | `"discovery-minimal-for-walkai-api-client"` | Name of the ClusterRoleBinding that assigns the discovery ClusterRole. |
| clusterInfoExporter.apiClient.discoveryClusterRoleName | string | `"discovery-minimal"` | Name of the discovery ClusterRole granted to the API client. |
| clusterInfoExporter.apiClient.enabled | bool | `true` | Enable or disable provisioning of the API client resources. |
| clusterInfoExporter.apiClient.namespace | string | `"walkai"` | Namespace where the API client ServiceAccount and token Secret live. |
| clusterInfoExporter.apiClient.serviceAccountName | string | `"api-client"` | Name of the API client ServiceAccount. |
| clusterInfoExporter.apiClient.tokenSecretName | string | `"api-client-permanent-token"` | Name of the long-lived ServiceAccount token Secret for the API client. |
| clusterInfoExporter.config.endpoint | string | `""` | API endpoint that receives cluster information payloads (exporter deploys only when set). |
| clusterInfoExporter.config.httpTimeout | string | `"10s"` | HTTP timeout for report requests. |
| clusterInfoExporter.config.interval | string | `"10s"` | Interval between reports (e.g. 10s, 5m). |
| clusterInfoExporter.enabled | bool | `false` | Enable or disable the Cluster Info Exporter DaemonSet (requires `clusterInfoExporter.config.endpoint`). |
| clusterInfoExporter.fullnameOverride | string | `""` | Overrides the fully qualified name of the Cluster Info Exporter resources. |
| clusterInfoExporter.image.pullPolicy | string | `"IfNotPresent"` | Image pull policy of the Cluster Info Exporter container. |
| clusterInfoExporter.image.repository | string | `"ghcr.io/walkai-org/nos-cluster-info-exporter"` | Repository of the Cluster Info Exporter image. |
| clusterInfoExporter.image.tag | string | `""` | Overrides the Cluster Info Exporter image tag whose default is the chart appVersion. |
| clusterInfoExporter.nameOverride | string | `""` | Overrides the name of the Cluster Info Exporter resources. |
| clusterInfoExporter.nodeSelector | object | `{}` | Node selector for the Cluster Info Exporter Pods. |
| clusterInfoExporter.podAnnotations | object | `{}` | Annotations added to the Cluster Info Exporter Pods. |
| clusterInfoExporter.podSecurityContext | object | `{"runAsNonRoot":true}` | Pod security context for the Cluster Info Exporter Pods. |
| clusterInfoExporter.resources | object | `{"limits":{"cpu":"200m","memory":"128Mi"},"requests":{"cpu":"50m","memory":"64Mi"}}` | Resource requests and limits for the Cluster Info Exporter container. |
| clusterInfoExporter.secret.apiToken | string | `""` | Value used when creating the API token Secret (ignored when an existing Secret is used). |
| clusterInfoExporter.secret.create | bool | `false` | When true the chart creates the Secret that stores the API token. |
| clusterInfoExporter.secret.existingSecret | string | `""` | Use an already existing Secret instead of creating a new one. |
| clusterInfoExporter.secret.key | string | `"apiToken"` | Key that stores the token inside the Secret. |
| clusterInfoExporter.secret.name | string | `"cluster-info-exporter-secrets"` | Name of the Secret that stores the API token. |
| clusterInfoExporter.securityContext | object | `{"allowPrivilegeEscalation":false,"readOnlyRootFilesystem":true}` | Container security context for the Cluster Info Exporter. |
| clusterInfoExporter.serviceAccount.annotations | object | `{}` | Extra annotations added to the Cluster Info Exporter ServiceAccount. |
| clusterInfoExporter.serviceAccount.create | bool | `true` | Whether to create the Cluster Info Exporter ServiceAccount. |
| clusterInfoExporter.serviceAccount.name | string | `""` | Overrides the Cluster Info Exporter ServiceAccount name. |
| clusterInfoExporter.tolerations | list | `[{"effect":"NoSchedule","key":"node-role.kubernetes.io/master","operator":"Exists"},{"effect":"NoSchedule","key":"node-role.kubernetes.io/control-plane","operator":"Exists"}]` | Tolerations applied to the Cluster Info Exporter Pods. |
| gpuPartitioner.affinity | object | `{}` | Sets the affinity config of the GPU Partitioner Pod. |
| gpuPartitioner.requeueIntervalSeconds | int | `10` | Interval at which the GPU partitioner reconciler wakes up even without new Pod events |
| gpuPartitioner.devicePlugin.config.name | string | `"nos-device-plugin-configs"` | Name of the ConfigMap containing the NVIDIA Device Plugin configuration files. It must be equal to the value "devicePlugin.config.name" of the Helm chart used for deploying the NVIDIA GPU Operator. |
| gpuPartitioner.devicePlugin.config.namespace | string | `"nebuly-nvidia"` | Namespace of the ConfigMap containing the NVIDIA Device Plugin configuration files. It must be equal to the namespace where the Nebuly NVIDIA Device Plugin has been deployed to. |
| gpuPartitioner.devicePlugin.configUpdateDelaySeconds | int | `5` | Duration of the delay between when the new partitioning config is computed and when it is sent to the NVIDIA device plugin. Since the config is provided to the plugin as a mounted ConfigMap, this delay is required to ensure that the updated ConfigMap is propagated to the mounted volume. |
| gpuPartitioner.enabled | bool | `true` | Enable or disable the `nos gpu partitioner` |
| gpuPartitioner.fullnameOverride | string | `""` |  |
| gpuPartitioner.image.pullPolicy | string | `"IfNotPresent"` | Sets the GPU Partitioner Docker image pull policy. |
| gpuPartitioner.image.repository | string | `"ghcr.io/walkai-org/nos-gpu-partitioner"` | Sets the GPU Partitioner Docker image. |
| gpuPartitioner.image.tag | string | `""` | Overrides the GPU Partitioner image tag whose default is the chart appVersion. |
| gpuPartitioner.knownMigGeometries | list | - | List that associates GPU models to the respective allowed MIG configurations |
| gpuPartitioner.kubeRbacProxy | object | - | Configuration of the [Kube RBAC Proxy](https://github.com/brancz/kube-rbac-proxy), which runs as sidecar of all the GPU Partitioner components Pods. |
| gpuPartitioner.leaderElection.enabled | bool | `true` | Enables/Disables the leader election of the GPU Partitioner controller manager. |
| gpuPartitioner.logLevel | int | `0` | The level of log of the GPU Partitioner. Zero corresponds to `info`, while values greater or equal than 1 corresponds to higher debug levels. **Must be >= 0**. |
| gpuPartitioner.migAgent | object | - | Configuration of the MIG Agent component of the GPU Partitioner. |
| gpuPartitioner.migAgent.image.pullPolicy | string | `"IfNotPresent"` | Sets the MIG Agent Docker image pull policy. |
| gpuPartitioner.migAgent.image.repository | string | `"ghcr.io/walkai-org/nos-mig-agent"` | Sets the MIG Agent Docker image. |
| gpuPartitioner.migAgent.image.tag | string | `""` | Overrides the MIG Agent image tag whose default is the chart appVersion. |
| gpuPartitioner.migAgent.logLevel | int | `0` | The level of log of the MIG Agent. Zero corresponds to `info`, while values greater or equal than 1 corresponds to higher debug levels. **Must be >= 0**. |
| gpuPartitioner.migAgent.reportConfigIntervalSeconds | int | `10` | Interval at which the mig-agent will report to k8s the MIG partitioning status of the GPUs of the Node |
| gpuPartitioner.migAgent.resources | object | `{"limits":{"cpu":"100m","memory":"128Mi"}}` | Sets the resource requests and limits of the MIG Agent container. |
| gpuPartitioner.migAgent.tolerations | list | `[{"effect":"NoSchedule","key":"kubernetes.azure.com/scalesetpriority","operator":"Equal","value":"spot"}]` | Sets the tolerations of the MIG Agent Pod. |
| gpuPartitioner.nameOverride | string | `""` |  |
| gpuPartitioner.nodeSelector | object | `{}` | Sets the nodeSelector config of the GPU Partitioner Pod. |
| gpuPartitioner.podAnnotations | object | `{}` | Sets the annotations of the GPU Partitioner Pod. |
| gpuPartitioner.podSecurityContext | object | `{"runAsNonRoot":true,"runAsUser":1000}` | Sets the security context of the GPU partitioner Pod. |
| gpuPartitioner.replicaCount | int | `1` | Number of replicas of the gpu-manager Pod. |
| gpuPartitioner.resources | object | `{"limits":{"cpu":"500m","memory":"128Mi"},"requests":{"cpu":"10m","memory":"64Mi"}}` | Sets the resource limits and requests of the GPU partitioner container. |
| gpuPartitioner.scheduler.config.name | string | `"nos-scheduler-config"` | Name of the ConfigMap containing the k8s scheduler configuration file. If not specified or the ConfigMap does not exist, the GPU partitioner will use the default k8s scheduler profile. |
| gpuPartitioner.tolerations | list | `[]` | Sets the tolerations of the GPU Partitioner Pod. |
| nvidiaGpuResourceMemoryGB | int | `32` | Defines how many GB of memory each nvidia.com/gpu resource has. |
| shareTelemetry | bool | `true` | If true, shares with Nebuly telemetry data collected only during the Chart installation |
