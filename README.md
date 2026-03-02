# Nebuly Operating System (nos)

![nos logo](docs/en/docs/img/nos-logo.png)

---

`nos` is the in-cluster component of the `walk:ai` platform. It runs on Kubernetes and is responsible for:
- dynamic GPU partitioning so Pods can share GPUs efficiently
- GPU and workload telemetry export that powers the walk:ai application

## Documentation

Start here: [Overview](https://walkai-org.github.io/docs/nos/overview/)

| Topic | Link |
| --- | --- |
| Prerequisites | [Read](https://walkai-org.github.io/docs/nos/prerequisites/) |
| Installation | [Read](https://walkai-org.github.io/docs/nos/installation/) |
| Dynamic GPU Partitioning | [Read](https://walkai-org.github.io/docs/nos/dynamic-gpu-partitioning/overview/) |
| Elastic Resource Quota | [Read](https://walkai-org.github.io/docs/nos/elastic-resource-quota/overview/) |
| Telemetry | [Read](https://walkai-org.github.io/docs/nos/telemetry/) |
| Helm chart values | [Read](https://walkai-org.github.io/docs/nos/helm-charts/nos/) |
| Troubleshooting | [Dynamic GPU Partitioning](https://walkai-org.github.io/docs/nos/dynamic-gpu-partitioning/troubleshooting/), [Elastic Resource Quota](https://walkai-org.github.io/docs/nos/elastic-resource-quota/troubleshooting/) |

## Why nos

- Improves GPU utilization with dynamic in-cluster partitioning.
- Enables workload-level observability through telemetry exports.
- Integrates directly with Kubernetes-native workflows.

## License

Apache-2.0. See [LICENSE](LICENSE).
