# Jobs

In `walk:ai`, a **Job** is the main unit of work you submit to the platform. It describes *what* should run on the cluster, *with which resources*, and *where outputs should go*.

A Job is eventually turned into a Kubernetes Job plus supporting resources, but the platform hides most of those details.

## What a Job contains

When you create a Job (from the web application or the CLI), you typically specify:

- **Container image**: the image that will be executed. This is usually built with the `walkai` CLI from a Python project.
- **GPU partition**: the fraction of GPU you want for the Job (for example `1g.10gb`, `3g.40gb`, etc.), which is enforced by `walk:ai nos` using MIG.
- **Storage for outputs**: how much space the Job should have available for writing results. This is mounted inside the container and later uploaded to object storage.
- **Inputs**: references to input data already stored in object storage that should be made available to the Job.
- **Secrets**: a list of platform-managed secrets that will be injected as environment variables.

The application takes this specification and turns it into a Kubernetes Job with the right volumes, environment and GPU requests.

## Job lifecycle

From the user’s perspective, a Job goes through a simple lifecycle:

1. **Submitted** – the Job is created via the API, CLI or web console.
2. **Pending** – the platform is waiting for enough capacity (CPU, memory, GPU slice) to schedule it.
3. **Running** – the container is executing on a node, using the requested GPU partition and storage.
4. **Completed / Failed** – the main container exits successfully or with an error.

At the end of the execution:

- Outputs written to the Job’s output directory are collected by a sidecar and uploaded to object storage.
- Logs from the main container are also collected and stored so they can be inspected later from the application.

## Jobs, runs and scheduling

Internally, a Job can have one or more *runs* associated with it (for example, if you re-run the same Job with the same definition). Each run represents one concrete execution on the cluster, with its own status, start time and outputs.

Scheduling decisions — including how GPU slices are partitioned and which Jobs are admitted first — are handled by the platform together with `walk:ai nos`. Future sections (for example **Priorities & scheduling**) describe how concepts like priority, fairness and preemption affect how Jobs progress through the lifecycle.
