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
microservices. They were running data processing jobs, build tasks, and system maintenance scripts. These jobs often
needed to interact with local services, access specific ports, or communicate with other processes on the host.

## Decision

We chose a selective namespace isolation strategy. We isolate PID, mount, IPC, UTS, and cgroup namespaces, but
deliberately share the network namespace with the host.

Here's the reasoning for each choice:

- **PID namespace (isolated)**: Jobs can't see or signal host processes. Essential for security.
- **Mount namespace (isolated)**: Jobs get their own filesystem view through chroot. Critical for controlling file
  access.
- **IPC namespace (isolated)**: Prevents jobs from accessing host IPC resources. Security win with minimal downside.
- **UTS namespace (isolated)**: Jobs can have their own hostname. Nice for clarity, no compatibility impact.
- **Cgroup namespace (isolated)**: Jobs can't see or modify host cgroup settings. Essential for resource isolation.
- **Network namespace (shared)**: Jobs use the host network stack. This was the controversial one.

The network namespace decision was deliberate and carefully considered. Isolated networking means complexity - bridge
networks, NAT, port mapping, DNS configuration. It means jobs can't easily talk to localhost services. It means
debugging network issues becomes a nightmare.

## Consequences

### The Good

The shared network namespace has been a huge win for usability. Jobs can connect to databases running on localhost. They
can bind to specific ports without port mapping configuration. They can use the host's DNS settings without any setup.
It just works, which is what users expect from a job runner.

From an operational perspective, this choice eliminated an entire class of problems. No troubleshooting bridge networks.
No NAT issues. No "why can't my job reach this service" tickets. Network traffic from jobs appears as regular host
traffic, which means existing monitoring and security tools work without modification.

Performance is better too. No virtual network interfaces, no packet routing between namespaces, no NAT overhead. For
network-heavy jobs, this matters.

### The Trade-offs

Obviously, sharing the network namespace means jobs aren't network-isolated. A job can see all network traffic on the
host (though it still can't access processes or files without permission). A job can potentially interfere with network
services.

We've accepted this trade-off because our threat model is different from a multi-tenant container platform. Joblet users
are running their own code on their own infrastructure. The isolation is about preventing accidents and containing
failures, not about protecting against malicious actors who already have code execution rights.

### The Mitigations

We didn't just accept the network sharing blindly. The other namespace isolations compensate significantly:

- Jobs can't see host processes, so they can't attack services directly
- Jobs can't access the host filesystem beyond their chroot, so they can't steal credentials
- Jobs are resource-limited through cgroups, so they can't DoS the network
- Jobs run with limited capabilities, reducing what they can do even with network access
- **Jobs run as unprivileged user (nobody/65534)** - Even if a job escapes the chroot, it cannot elevate privileges or
  damage the host system. This privilege dropping happens after isolation setup but before the job command executes.

We also made it clear in documentation that Joblet provides process and filesystem isolation, not full container-style
network isolation. Users who need network isolation can run Joblet inside a container or VM.

### The Unexpected Benefits

The simplified networking made some features trivial to implement. Service discovery just works because jobs can talk to
localhost. Distributed tracing works because jobs share the host's network context. Integration with existing
infrastructure required zero network configuration.

It also made Joblet much easier to adopt. Teams could drop it into existing environments without redesigning their
network architecture. No firewall rules to update, no service mesh to configure, no CNI plugins to debug.

## Learn More

See [DESIGN.md](/docs/DESIGN.md) for the complete isolation architecture
and [HOST_PROTECTION_GUARANTEES.md](/docs/HOST_PROTECTION_GUARANTEES.md) for security implications.