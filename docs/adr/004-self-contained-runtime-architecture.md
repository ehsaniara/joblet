# ADR-004: Self-Contained Runtime Architecture

## Status

Accepted

## Context

One of Joblet's most powerful features is the ability to run jobs in different runtime environments - Python for ML
workloads, Java for enterprise applications, Node.js for web services. The question was: how do we provide these
runtimes in an isolated environment without the complexity of traditional container images?

Initially, we considered a hybrid approach where runtimes would provide their specific binaries (like Python or Java),
and we'd mount system binaries from the host. This seemed efficient - why duplicate `/bin/bash` for every runtime?

But then we started hitting edge cases. What if the host has an incompatible version of a system library? What if a
Python script shells out to a command that exists on the development machine but not on the production server? What
about glibc version mismatches between what Python was compiled against and what the host provides?

The nail in the coffin was when we tried to run a Python ML runtime on a minimal Alpine-based host. The glibc vs musl
incompatibility made it clear: we needed runtimes to be completely self-contained.

## Decision

Each runtime is a complete, self-contained filesystem tree. When you install a Python runtime, you get Python *and* all
the system binaries and libraries it might need. Every runtime includes its own `/bin`, `/lib`, `/usr`, everything.

During job execution, we mount only from the runtime's isolated directory. No mixing with host files. The job sees a
consistent, predictable environment regardless of what's installed on the host.

The structure is beautifully simple:

```
/opt/joblet/runtimes/python-3.11-ml/
├── isolated/          # Complete root filesystem
│   ├── bin/          # bash, sh, ls, etc.
│   ├── lib/          # All system libraries
│   ├── usr/bin/      # Python lives here
│   └── ...
└── runtime.yml        # Simple mount configuration
```

## Consequences

### The Good

The isolation is perfect. A job running in the Python runtime sees exactly the same environment whether it's running on
Ubuntu, RHEL, or a custom-built Linux. No surprises, no "works on my machine" problems.

Debugging became trivial. If something's wrong with a runtime, you can literally chroot into its isolated directory and
poke around. Everything the job sees is right there.

Runtime creation is straightforward. You set up a minimal base system, install your runtime-specific software, copy it
all into the isolated directory, done. No complex layering, no dependency resolution with the host.

### The Trade-offs

Yes, there's duplication. Every runtime has its own copy of bash, ls, and other common utilities. A Python runtime might
be 300MB instead of 100MB. But storage is cheap, and the predictability is worth it.

Runtime installation takes longer because we're copying more files. But this happens once, during setup, not during job
execution. We'll take a slower installation for faster, more reliable job execution any day.

Building runtimes requires more thought. You need to ensure all dependencies are included. But this forced us to be
explicit about what each runtime needs, which actually improved our understanding of the dependencies.

### The Unexpected Benefits

The self-contained approach enabled some features we didn't initially plan for. Want to run an ancient Python 2.7 job
that needs old glibc? No problem, that runtime can have whatever libraries it needs. Want to test the new Python 3.13
alpha? Install it in a runtime without touching the host system.

It also made security auditing easier. Each runtime is a fixed filesystem tree that can be scanned, verified, and
signed. No dynamic resolution of host libraries means no unexpected security holes from host package updates.

The approach also simplified our CI/CD pipeline for runtimes. We build them in containers, test them in containers, then
just tar up the isolated directory. No complex packaging, no dependency specifications, just files.

### Real-World Example

Here's what happened when a user needed to run both Python 3.11 ML workloads and legacy Python 2.7 scripts:

```bash
# Build modern ML runtime
rnx runtime build ./examples/python-3.11-ml/runtime.yaml

# Build legacy runtime (with old glibc)
rnx runtime build ./examples/python-2.7-legacy/runtime.yaml

# Both work perfectly, in complete isolation
rnx job run --runtime=python-3.11-ml python -c "import torch; print(torch.__version__)"
rnx job run --runtime=python-2.7-legacy python -c "print 'Hello from Python 2'"
```

No conflicts, no environment variables to juggle, no system package conflicts. It just works.

## Learn More

See [builder-runtime-final.md](/docs/design/builder-runtime-final.md) for implementation details
and [runtime_design_doc.md](/docs/runtime_design_doc.md) for the complete runtime system design.