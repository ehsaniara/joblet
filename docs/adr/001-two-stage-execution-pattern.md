# ADR-001: Two-Stage Execution Pattern

## Status

Accepted

## Context

When we started building Joblet, we faced a fundamental challenge: how do you create a system that can securely execute
arbitrary commands while maintaining complete isolation? Traditional approaches would require separate binaries for the
server and the isolated processes, or complex helper scripts that manage the isolation setup. We needed something
cleaner.

The problem became even more interesting when we considered deployment and maintenance. Having multiple binaries means
version mismatches are possible. It means more complex deployment scripts. It means your isolated processes might be
running different code than your server expects.

## Decision

We decided to implement a two-stage execution pattern where the same binary operates in different modes based on
environment variables. When Joblet starts, it checks `JOBLET_MODE` - if it's "server", it runs as the gRPC server. If
it's "init", it becomes an isolated job executor.

This means when the server spawns a job, it's actually spawning itself with different environment variables and
namespace configurations. The spawned process then reads its configuration from the environment and executes the
requested command in complete isolation.

## Consequences

### The Good

This approach turned out to be brilliant for several reasons. First, there's only one binary to deploy, update, and
manage. When you update Joblet, everything updates together - no risk of version drift between components.

The security implications are also positive. Since the init process is the same trusted binary, there's no external
dependency that could be compromised or replaced. The attack surface is minimized because we control the entire
execution path.

Testing became significantly easier too. We can test both server and init modes in the same test suite, and we know
they're using identical code paths for shared functionality.

### The Trade-offs

Of course, there are some trade-offs. The binary is larger than it would be if we split the functionality. Every
isolated job carries the full server code even though it only uses the init portion. In practice, this hasn't been an
issue - modern systems have plenty of memory, and the binary is still relatively small.

There's also some code complexity in managing the dual nature of the application. We need to be careful about
initialization paths and ensure that server-specific code doesn't run in init mode and vice versa.

### The Unexpected Benefits

What we didn't anticipate was how this pattern would simplify debugging. When something goes wrong in an isolated job,
we can run the exact same binary locally in init mode with the same parameters. No need to replicate a complex
environment or wonder if the helper scripts are different.

It also made our CI/CD pipeline beautifully simple. One build artifact, one set of tests, one deployment process. The
operational simplicity has been worth any minor trade-offs in binary size.

## Learn More

See [DESIGN.md](/docs/DESIGN.md) for the complete system architecture
and [RUNTIME_DESIGN.md](/docs/RUNTIME_DESIGN.md) for how this pattern extends to runtime
management.