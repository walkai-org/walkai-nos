# Overview

`walk:ai` is an open-source platform for running AI/ML workloads on Kubernetes in an intuitive and optimized way, focused on increasing GPU utilization.

Currently, the platform consists of three componentes:

* [nos](nos/overview.md): a set of services deployed in your cluster that handle dynamic GPU partitioning and send GPU and workload telemetry back to the `walk:ai` application.

* [Application](app/overview.md): a single control plane to submit jobs, inspect runs, manage volumes and secrets, and observe cluster GPU usage through a browser-based UI.

* [Command-line tool](cli/overview.md): a developer-friendly CLI that turns Python projects into container images without writing Dockerfiles or Kubernetes manifests. Also lets you interface with the `walk:ai` platform just like in the browser, but without leaving your terminal.


![](img/gpu-utilization.png)
