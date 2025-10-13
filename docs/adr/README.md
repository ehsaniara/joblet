# Architecture Decision Records (ADR)

This directory contains Architecture Decision Records (ADRs) for Joblet. ADRs document important architectural
decisions, their context, and consequences.

## What is an ADR?

An Architecture Decision Record captures a single architectural decision and its rationale. Each ADR describes:

- **Context**: The situation and forces at play
- **Decision**: The architectural decision made
- **Consequences**: The results and trade-offs of the decision

## ADR Format

Each ADR follows this structure:

- **Title**: Short descriptive title
- **Status**: Proposed, Accepted, Deprecated, or Superseded
- **Date**: When the decision was made
- **Context**: Background and problem statement
- **Decision**: What was decided and why
- **Consequences**: Positive, negative, and neutral impacts
- **Alternatives Considered**: Other options that were evaluated

## ADR Index

| Number                                              | Title                                         | Status   | Date       |
|-----------------------------------------------------|-----------------------------------------------|----------|------------|
| [001](001-two-stage-execution-pattern.md)           | Two-Stage Execution Pattern                   | Accepted | 2024-09-22 |
| [002](002-workflow-vs-job-separation.md)            | Workflow vs Job Separation                    | Accepted | 2024-09-22 |
| [003](003-namespace-isolation-strategy.md)          | Namespace Isolation Strategy                  | Accepted | 2024-09-22 |
| [004](004-self-contained-runtime-architecture.md)   | Self-Contained Runtime Architecture           | Accepted | 2025-09-*  |
| [005](005-async-log-persistence.md)                 | Async Log Persistence                         | Accepted | 2024-09-22 |
| [006](006-embedded-certificates.md)                 | Embedded Certificates                         | Accepted | 2024-09-22 |
| [007](007-cgroups-v2-resource-management.md)        | cgroups v2 Resource Management                | Accepted | 2024-09-22 |
| [008](008-gpu-support-architecture.md)              | GPU Support Architecture                      | Accepted | 2024-09-*  |
| [009](009-seccomp-syscall-filtering.md)             | Seccomp Syscall Filtering                     | Accepted | 2024-09-*  |
| [010](010-collect-jobs-metrics.md)                  | Collect Jobs Metrics                          | Accepted | 2024-10-*  |
| [011](011-cqrs-architecture-with-joblet-persist.md) | CQRS Architecture with joblet-persist Service | Accepted | 2025-10-*  |

## Creating a New ADR

1. Copy the template from the most recent ADR
2. Use the next sequential number (e.g., 0002 or 011)
3. Follow the naming convention: `NNNN-short-descriptive-title.md`
4. Fill in all sections with relevant information
5. Update this README with the new entry

## Guidelines

- **Be concise**: ADRs should be readable in 5-10 minutes
- **Be specific**: Include concrete examples and code snippets
- **Be honest**: Document negative consequences and trade-offs
- **Be complete**: Include alternatives considered and why they were rejected
- **Be timely**: Write ADRs when decisions are made, not retrospectively

## Resources

- [ADR Tools and Resources](https://adr.github.io/)
- [Michael Nygard's ADR template](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions)
