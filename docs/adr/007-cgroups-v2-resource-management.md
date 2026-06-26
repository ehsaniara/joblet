# ADR-007: Cgroups v2 for Resource Management

## Status

Accepted

## Context

Resource management in a job execution system is like being a bouncer at a club. You need to make sure everyone gets in,
has a good time, but nobody hogs the dance floor or drinks all the champagne. Without proper resource control, one
misbehaving job can ruin the party for everyone.

We had a choice to make: cgroups v1 or v2? The old guard v1 has been around forever, is well-documented, and works
everywhere. But it's also a mess - different controllers in different hierarchies, weird interactions, and that
absolutely bonkers memory controller that nobody fully understands.

Cgroups v2 promised to fix all this with a unified hierarchy and sane semantics. But it was newer, less battle-tested,
and required newer kernels. The safe choice would have been v1. But safe choices don't always lead to better software.

## Decision

We went all-in on cgroups v2. Single unified hierarchy. Clean API. Predictable behavior. Yes, it meant requiring Linux
4.15+, but we decided that was a reasonable requirement for a modern job execution system.

The integration is clean and simple:

```text
/sys/fs/cgroup/
└── joblet.slice/
    └── joblet.service/
        ├── job-1/
        │   ├── memory.max      # Memory limit
        │   ├── cpu.max         # CPU limit
        │   └── io.max          # I/O limit
        └── job-2/
```

Each job gets its own cgroup. Resources are controlled through simple writes to control files. No complex hierarchies,
no controller mounting gymnastics, just straightforward resource management.

## Consequences

### The Good

The unified hierarchy is a thing of beauty. One place to look, one place to configure, one place to monitor. No more
hunting through multiple mount points trying to figure out where the memory controller lives.

Pressure stall information (PSI) in v2 gives us incredible insights. We can tell not just that a job is using memory,
but whether it's under memory pressure. This lets us make smarter scheduling decisions.

The improved memory controller actually works predictably. No more "why did OOM killer trigger when we're under the
limit" mysteries. Memory accounting that includes kernel memory means no more hidden memory usage.

The CPU controller's bandwidth control is more intuitive than v1's shares system. Saying "this job gets 50% of a CPU"
with `cpu.max` is clearer than juggling cpu.shares values.

### The Trade-offs

We did limit ourselves to newer systems. But honestly, if you're running production workloads on ancient kernels, you
have bigger problems than not being able to run Joblet.

Some monitoring tools initially didn't support v2. But by the time we shipped, all the major tools had caught up. And
the ones that hadn't weren't worth using anyway.

The migration from v1 systems requires kernel updates. But cgroups v2 has been the default in major distributions for
years now. We're not exactly bleeding edge here.

### The Implementation Details

We integrate with systemd's delegation model, which was built for v2. Joblet runs as a systemd service with
`Delegate=yes`, giving us a cgroup subtree to manage. This plays nicely with the rest of the system.

Resource limits are straightforward to implement:

```go
// CPU: 50% of one core
echo "50000 100000" > /sys/fs/cgroup/joblet.slice/joblet.service/job-1/cpu.max

// Memory: 512MB
echo "536870912" > /sys/fs/cgroup/joblet.slice/joblet.service/job-1/memory.max

// I/O: 10MB/s read
echo "8:0 rbps=10485760" > /sys/fs/cgroup/joblet.slice/joblet.service/job-1/io.max
```

Clean, simple, understandable.

### The Unexpected Benefits

The v2 freezer controller turned out to be amazing for job management. We can instantly freeze a job (truly pause it,
not just stop scheduling it), inspect its state, and thaw it. This made implementing job suspension trivial.

The unified hierarchy made resource inheritance logical. Settings on parent cgroups properly propagate to children. No
more weird edge cases where CPU limits work but memory limits don't.

Threading and cgroups work sensibly in v2. All threads of a process are in the same cgroup, period. No more weird
split-brain situations that were possible in v1.

### Real-World Impact

Here's a real scenario we encountered: A user ran a memory-leak job that gradually consumed all available memory.

**With cgroups v1**: The job would sometimes escape memory accounting through kernel memory usage, eventually triggering
system-wide OOM killer, taking down random processes.

**With cgroups v2**: The job hit its memory limit, got OOM-killed cleanly, logged the event, and the system continued
running normally. Other jobs weren't even aware anything happened.

That's the difference between "mostly works" and "actually works."

### The Future

Cgroups v2 positions us well for future enhancements. The eBPF integration in v2 opens doors for custom resource
controllers. The pressure metrics enable predictive scheduling. The clean API makes it easy to add new resource types as
they become available.

We made the right choice. V2 isn't just newer, it's better. And building on better foundations leads to better software.

## Learn More

See [DESIGN.md](/docs/DESIGN.md#61-cgroups-v2-integration) for technical integration details and the cgroups v2
documentation at kernel.org for the full story.