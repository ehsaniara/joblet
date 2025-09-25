# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) for the Joblet project. ADRs document the key architectural
decisions made during development, including the context, the decision itself, and its consequences.

## What is an ADR?

An Architecture Decision Record captures an important architectural decision made along with its context and
consequences. ADRs help future developers understand not just *what* we built, but *why* we built it that way.

## ADR Index

| ADR                                               | Title                                   | Status   | Summary                                                                                        |
|---------------------------------------------------|-----------------------------------------|----------|------------------------------------------------------------------------------------------------|
| [001](001-two-stage-execution-pattern.md)         | Two-Stage Execution Pattern             | Accepted | Single binary operates as both server and isolated job executor based on environment variables |
| [002](002-workflow-vs-job-separation.md)          | Workflow vs Individual Job Architecture | Accepted | Complete separation of job and workflow services for clarity and optimization                  |
| [003](003-namespace-isolation-strategy.md)        | Linux Namespace Isolation Strategy      | Accepted | Selective namespace isolation with shared networking for compatibility                         |
| [004](004-self-contained-runtime-architecture.md) | Self-Contained Runtime Architecture     | Accepted | Each runtime includes complete filesystem for perfect isolation                                |
| [005](005-async-log-persistence.md)               | Asynchronous Log Persistence System     | Accepted | Rate-decoupled logging system for high-performance job execution                               |
| [006](006-embedded-certificates.md)               | Embedded Certificate Architecture       | Accepted | TLS certificates embedded in configuration files for operational simplicity                    |
| [007](007-cgroups-v2-resource-management.md)      | Cgroups v2 for Resource Management      | Accepted | Modern cgroups v2 for clean, predictable resource control                                      |
| [008](008-gpu-support-architecture.md)            | GPU Support Architecture                | Accepted | Native Linux kernel features for GPU isolation without container runtimes                      |
| [009](009-seccomp-syscall-filtering.md)           | Seccomp Syscall Filtering               | Proposed | Kernel-level syscall filtering for defense-in-depth security                                   |

## ADR Template

When creating a new ADR, use this template:

```markdown
# ADR-XXX: [Title]

## Status

[Accepted|Deprecated|Superseded|Proposed]

## Context

[Describe the problem, why a decision needed to be made, what forces were at play, and what led to this decision point. Write this in a narrative style - tell the story of why this decision was necessary.]

## Decision

[Clearly state the decision that was made. What did we choose to do? Be specific and technical where necessary, but keep it readable.]

## Consequences

### The Good

[What positive outcomes resulted from this decision? What problems did it solve? What benefits did we gain?]

### The Trade-offs

[What did we give up? What became more complex? What limitations did we accept? Be honest about the downsides.]

### The Unexpected Benefits

[What positive outcomes emerged that we didn't anticipate? What serendipitous advantages did we discover?]

## Learn More

[Links to related documentation, code, or external resources]
```

## Contributing

When adding a new ADR:

1. Use the next number in sequence (e.g., 008)
2. Follow the template above
3. Update this README with a link to your ADR
4. Write in a narrative, human style - tell the story
5. Be honest about trade-offs and mistakes
6. Include real examples where possible

## Why We Write ADRs

Writing ADRs serves several purposes:

1. **Future Understanding**: New team members can understand not just the code, but the reasoning behind it
2. **Decision History**: We can track how our architecture evolved over time
3. **Learning Tool**: Failed decisions are as valuable as successful ones for learning
4. **Confidence**: Well-documented decisions give confidence that choices were thoughtful, not arbitrary

## Related Documentation

- [DESIGN.md](/docs/DESIGN.md) - Complete system design documentation
- [IMPLEMENTATION_SUMMARY.md](/docs/IMPLEMENTATION_SUMMARY.md) - Implementation details
