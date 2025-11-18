# Command-line tool

The `walkai` CLI is an opinionated companion to the platform that focuses on two things: turning Python projects into container images, and interacting with `walk:ai` from the terminal.

**Features**

- Builds OCI images from Python projects without writing Dockerfiles or Kubernetes manifests.
- Uses a `[tool.walkai]` section in `pyproject.toml` to declare entrypoint, OS dependencies, and ignore paths.
- Integrates with the `walk:ai` API to submit jobs using those images, passing inputs, secrets and GPU requirements.

For installation, configuration examples, and detailed usage, see the CLI repository and README:

- GitHub: [walkai-org/walkai-cli](https://github.com/walkai-org/walkai-cli?tab=readme-ov-file#walkai-cli)

