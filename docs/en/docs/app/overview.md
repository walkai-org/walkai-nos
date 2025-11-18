# Application

The `walk:ai` application is the control plane of the platform. It exposes an HTTP API and a browser-based console to submit and monitor jobs, manage data, and inspect how GPUs are being used across the cluster.

## Features

The web console provides a UI for working with the platform without touching YAML or `kubectl`. From the browser you can:

- Submit new jobs by choosing a container image, GPU slice, storage size and inputs/secrets.
- Inspect runs, see their status, drill into logs, view and download their outputs.
- Create and manage volumes used by jobs.
- Create, list and delete secrets that are later injected into Pods as environment variables.
- Explore near real-time views of GPU usage and MIG layouts, powered by the telemetry sent by `walk:ai nos`.


