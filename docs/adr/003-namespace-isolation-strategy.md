# ADR-003: Linux Namespace Isolation Strategy

## Status

Accepted

## Context

Security in a job execution system isn't optional - it's the foundation everything else builds on. When we designed
Joblet's isolation strategy, we had to balance several competing concerns: security, performance, compatibility, and
operational simplicity.

The traditional approach would be to use all available Linux namespaces - PID, mount, network, IPC, UTS, user, and
cgroup. Full isolation, like what Docker does. But we're not building another container runtime. We're building a job
execution system, and that changes the calculus significantly.

The critical realization came when we looked at our users' actual needs. They weren't trying to run isolated
microservices. They were running data processing jobs, build tasks, and system maintenance scripts. Most of these jobs
run untrusted code and should be kept off the host network stack, but one class - runtime builds - genuinely needs
outbound internet access to download packages during the build. That tension is what drove a job-type-aware decision
rather than a single blanket policy.

## Decision

We chose a selective, job-type-aware namespace isolation strategy. Production jobs (submitted through JobService via
`rnx job run`) isolate PID, mount, IPC, UTS, cgroup, AND network namespaces. Runtime-build jobs (submitted through
RuntimeService via `rnx runtime build`) isolate everything except the network namespace, which they deliberately share
with the host so package managers can reach the internet during the build.

Here's the reasoning for each choice:

- **PID namespace (isolated)**: Jobs can't see or signal host processes. Essential for security.
- **Mount namespace (isolated)**: Jobs get their own filesystem view through chroot. Critical for controlling file
  access.
- **IPC namespace (isolated)**: Prevents jobs from accessing host IPC resources. Security win with minimal downside.
- **UTS namespace (isolated)**: Jobs can have their own hostname. Nice for clarity, no compatibility impact.
- **Cgroup namespace (isolated)**: Jobs can't see or modify host cgroup settings. Essential for resource isolation.
- **Network namespace (isolated for production jobs, shared for runtime builds)**: Production jobs get their own
  network namespace (`CLONE_NEWNET`) and reach the outside world through Joblet's bridge networking. Runtime-build jobs
  share the host network stack so `apt`, `pip`, and `npm` can fetch dependencies during the build. This was the
  controversial one.

The network namespace decision was deliberate and carefully considered. Isolating production job networking means
complexity - bridge networks, NAT, port mapping, DNS configuration - but Joblet manages that for you, and keeping
untrusted job code off the host network stack is a real security boundary worth the cost. Runtime builds are a
controlled, admin/maintainer-initiated exception where internet access during the build outweighs network isolation.

## Consequences

### The Good

Isolating production job networking keeps untrusted job code off the host network stack. A job can't sniff host
traffic, can't bind to arbitrary host ports, and can't interfere with host network services. Outbound traffic is
routed through Joblet's bridge networking, and named networks (`--network=...`) let operators segment workloads into
separate security zones, while `--network=none` removes networking entirely for the most sensitive jobs.

From an operational perspective, Joblet manages the bridge, NAT, and DNS setup, so for most jobs the isolation is
transparent - they get outbound access without any per-job configuration.

Runtime builds keep the host network stack on purpose. Building a runtime means downloading packages, which needs
working internet access with the host's DNS and routing. Because runtime builds are admin/maintainer-initiated and run
in the builder environment, sharing the host network here is a controlled, deliberate exception rather than the default.

### The Trade-offs

Isolated networking is more moving parts than sharing the host stack: bridges, NAT rules, and DNS all have to be set
up and maintained. We accept that cost because network isolation is a real security boundary for the untrusted code
production jobs run.

The runtime-build exception means build jobs are NOT network-isolated - they can reach the host network stack. We
accept this because builds are initiated by trusted roles and their explicit purpose is to pull in external
dependencies.

### The Mitigations

Network isolation is reinforced by the other namespace isolations and the privilege model:

- Jobs can't see host processes, so they can't attack services directly
- Jobs can't access the host filesystem beyond their chroot, so they can't steal credentials
- Jobs are resource-limited through cgroups, so they can't DoS the host
- Jobs run with limited capabilities, reducing what they can do
- **Jobs run as unprivileged user (nobody/65534)** - Even if a job escapes the chroot, it cannot elevate privileges or
  damage the host system. This privilege dropping happens after isolation setup but before the job command executes.
  Note that Joblet does not use a user namespace (`CLONE_NEWUSER`); the user context is isolated by this post-setup
  privilege drop, not by UID remapping.

For the runtime-build exception, the same process, filesystem, and privilege isolations still apply - only the network
namespace is shared.

### The Unexpected Benefits

Driving network isolation off the job type kept the design simple: production jobs are secure by default, and the one
place that genuinely needs host networking - runtime builds - opts out explicitly rather than every job having to opt
in. Operators who need stricter segmentation compose named networks and `--network=none` on top of the default
isolation without touching the daemon.

## Learn More

See [DESIGN.md](/docs/DESIGN.md) for the complete isolation architecture
and [HOST_PROTECTION_GUARANTEES.md](/docs/HOST_PROTECTION_GUARANTEES.md) for security implications.