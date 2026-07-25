# ADR-015: Four-Role Authorization Model

## Status

**Implemented** (July 2026)

## Context

Joblet authorizes gRPC clients by mapping the mTLS client certificate's
Organizational Unit (OU) to a role. The original model had two roles: `admin`
(everything) and `viewer` (read-only). Real deployments have more than two
kinds of clients:

- People who administer nodes and own shared infrastructure.
- CI/CD automation that provisions runtimes, networks, and volumes
  declaratively and must not acquire destructive permissions along the way.
- Engineers who run jobs but have no reason to modify shared infrastructure.
- Reporting and observability tooling that only reads logs and telemetry.

The two-role model forced the middle two groups into `admin`. The gap was
amplified by the operation vocabulary: although granular operation constants
existed (`create_network`, `create_volume`, ...), the services gated volume,
network, and runtime mutation behind the generic `run_job` operation, so any
client that could run a job could also build or remove every runtime, network,
and volume on the node. Monitoring endpoints performed no role check at all:
any certificate signed by the CA could stream system metrics.

## Decision

Adopt four roles, carried in the client certificate OU (case-insensitive):

| Role         | Intended for                           | Permissions                                                                                          |
|--------------|----------------------------------------|------------------------------------------------------------------------------------------------------|
| `admin`      | Operators who own the node             | All operations, including removing runtimes, networks, and volumes                                   |
| `maintainer` | Automation with deterministic outcomes | Everything developer can do, plus build runtimes, validate runtime YAML, create networks and volumes |
| `developer`  | Engineers running jobs                 | Run, stop, and delete jobs; test runtimes; all read operations                                       |
| `reader`     | Reporting and observability            | Read-only: jobs, logs, status, runtime/network/volume listings, live metrics, historical queries     |

Supporting decisions:

- **Removal is admin-only.** Maintainer provisions but never destroys shared
  infrastructure. Removing a runtime, network, or volume affects every user of
  the node and stays with the operator who owns it.
- **`viewer` remains a legacy alias for `reader`.** Existing certificates keep
  working; new certificates use `reader`.
- **Every service checks an operation that names what it does.** Volume,
  network, runtime, and monitoring handlers now pass their own operations
  (`create_volume`, `remove_network`, `build_runtime`, `get_metrics`, ...)
  instead of borrowing job operations. Roles are defined as cumulative
  operation sets (reader ⊂ developer ⊂ maintainer ⊂ admin) in
  `internal/joblet/auth/grpc_authorization.go`.
- **Monitoring requires a role.** `GetSystemStatus` and `StreamSystemMetrics`
  authorize against `get_metrics`, which every role holds; a certificate with
  no recognized role OU is denied.
- **Both installers generate all four roles and per-role distribution
  artifacts.** `certs_gen_embedded.sh` and `certs_gen_with_secretsmanager.sh`
  produce one client certificate per role, an operator `rnx-config.yml` with
  every role's node (admin key included, so it stays on the server), and one
  self-contained `rnx-config-<role>.yml` per role for handing to that role's
  users. The config file is the credential; role separation only holds if each
  party receives only its own file. The AWS variant also stores each role's
  pair in Secrets Manager as `joblet/client-cert-<role>` and
  `joblet/client-key-<role>`, so client IAM policies can be scoped per role.
  The pre-existing unsuffixed `joblet/client-cert` and `joblet/client-key`
  names continue to hold the admin pair, keeping existing deployments and the
  `pre-setup.sh` seeding flow working unchanged.

Authorization remains method-level: a role either may or may not call an
operation. Constraining *which* runtimes, networks, or volumes a developer job
may reference is argument-level policy and is out of scope for this decision.

## Consequences

**Positive:**

- Automation credentials no longer imply destructive power; a leaked
  maintainer certificate cannot remove runtimes, networks, or volumes.
- Job-running credentials no longer imply infrastructure mutation.
- Read-only integrations (dashboards, reporting) have a first-class role, and
  monitoring endpoints are inside the authorization model.
- The operation vocabulary matches service behavior, so future per-operation
  decisions are one-line changes to a role's operation set.

**Negative:**

- Existing admin certificates used by automation should be reissued as
  `maintainer` or `developer` to realize the benefit; nothing forces this.
- Clients holding admin certificates for convenience lose nothing, so the
  migration relies on operators adopting the narrower roles.

**Neutral:**

- The role boundary is only as strong as job sandboxing underneath it; mount
  and privilege hardening are tracked independently of this decision.
