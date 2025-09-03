# Process Isolation in Joblet

Joblet provides complete process isolation using Linux PID namespaces, ensuring that jobs run in their own isolated process environment with complete separation from the host system.

## How Process Isolation Works

When you run a job with `rnx run`, Joblet:

1. **Creates a PID namespace** - The job gets its own process space
2. **Makes job process PID 1** - Your command becomes the init process in the namespace
3. **Isolates process visibility** - Jobs can only see their own process tree
4. **Preserves parent-child relationships** - Child processes are visible with proper PIDs

## Process Hierarchy

In the isolated namespace:
- **PID 1**: Your main job command (acts as init)
- **PID 2+**: Any child processes spawned by your job
- **No system processes**: Host system processes are completely hidden

## Examples

### Example 1: Simple Process with Children

Create a job that spawns multiple child processes:

```bash
rnx run --runtime=python-3.11-ml bash -c "sleep 30 & sleep 40 & ps aux"
```

**Output:**
```
USER         PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
0              1  1.0  0.1   4364  2916 ?        S    05:40   0:00 /usr/bin/bash -c sleep 30 & sleep 40 & ps aux
0              7  0.0  0.0   2792   280 ?        S    05:40   0:00 sleep 30
0              8  0.0  0.0   2792   280 ?        S    05:40   0:00 sleep 40
0              9  0.0  0.1   7064  2300 ?        R    05:40   0:00 ps aux
```

**Process Tree:**
- **PID 1**: Main bash process (parent)
- **PID 7**: First sleep process (child)
- **PID 8**: Second sleep process (child)
- **PID 9**: ps command (child)

### Example 2: Complex Multi-Process Job

Create a more complex job with different types of processes:

```bash
rnx run --runtime=python-3.11-ml bash -c "echo 'Starting processes...' && sleep 60 & echo 'Sleep started' && python3 -c 'import time; [print(f\"Python process {i}\") or time.sleep(1) for i in range(5)]' & echo 'Python started' && ps aux && echo 'Waiting for processes...' && wait"
```

**Process Tree Shows:**
- **PID 1**: Main bash process (parent)
- **PID 6-7**: Subshell processes
- **PID 8**: sleep 60 (background child)
- **PID 9**: python3 process (background child)
- **PID 10**: ps aux command (foreground child)

### Example 3: Process Synchronization

Create processes and wait for them to complete:

```bash
rnx run --runtime=python-3.11-ml bash -c "(sleep 10; echo 'Child 1 done') & (sleep 15; echo 'Child 2 done') & echo 'Both children started' && ps aux && wait && echo 'All processes completed'"
```

This demonstrates:
- Background processes with subshells
- Process synchronization using `wait`
- Complete process lifecycle visibility

## Key Benefits

### 1. **Authentic Isolated Experience**
- Job process naturally becomes PID 1
- Child processes get sequential PIDs (2, 3, 4, etc.)
- No fake process renumbering or filtering

### 2. **Complete Process Tree Visibility**
- See all processes spawned by your job
- Parent-child relationships are preserved
- Real-time process monitoring with `ps`, `top`, etc.

### 3. **Perfect Isolation**
- No host system processes visible
- Cannot see other jobs' processes
- Complete process namespace separation

### 4. **Standard Process Management**
- Use standard tools: `ps`, `kill`, `jobs`, `wait`
- Signal handling works normally
- Process groups and sessions work as expected

## Process Management Commands

All standard Linux process management commands work within the isolated namespace:

```bash
# View all processes in your job
rnx run --runtime=python-3.11-ml ps aux

# Monitor processes in real-time
rnx run --runtime=python-3.11-ml top

# Create background processes
rnx run --runtime=python-3.11-ml bash -c "long-running-task &"

# Wait for background processes
rnx run --runtime=python-3.11-ml bash -c "background-task & wait"

# Kill specific processes (by PID within namespace)
rnx run --runtime=python-3.11-ml bash -c "sleep 60 & kill %1"
```

## Technical Implementation

Joblet achieves process isolation through:

1. **Linux PID Namespaces**: Creates isolated process space
2. **Process Replacement**: Uses `exec` to replace init with job command
3. **Filesystem Isolation**: Remounts `/proc` to show only namespace processes
4. **cgroup Integration**: Resource limits apply to entire process tree

## Comparison with Container Solutions

| Feature | Joblet | Traditional Containers |
|---------|--------|----------------------|
| Process becomes PID 1 | ✅ | ✅ |
| Child process visibility | ✅ | ✅ |
| Host process isolation | ✅ | ✅ |
| Standard process tools | ✅ | ✅ |
| Resource limits | ✅ | ✅ |
| Network isolation | ✅ | ✅ |
| Lightweight execution | ✅ | ❌ |
| No image management | ✅ | ❌ |

## Best Practices

### 1. **Process Cleanup**
Always ensure child processes are cleaned up:
```bash
# Good: Wait for all background processes
rnx run --runtime=python-3.11-ml bash -c "task1 & task2 & wait"

# Good: Trap signals for cleanup
rnx run --runtime=python-3.11-ml bash -c "trap 'kill $(jobs -p)' EXIT; task1 & task2 & wait"
```

### 2. **Resource Management**
Monitor process resource usage:
```bash
# Monitor memory and CPU usage
rnx run --runtime=python-3.11-ml bash -c "memory-intensive-task & top -p \$!"
```

### 3. **Error Handling**
Handle process failures gracefully:
```bash
# Check background process status
rnx run --runtime=python-3.11-ml bash -c "risky-task & wait \$! || echo 'Task failed'"
```

## Security Implications

Process isolation provides strong security boundaries:

- **No process interference**: Jobs cannot affect other jobs' processes
- **No host visibility**: Cannot see or interact with host system processes  
- **Resource isolation**: Process limits are enforced at the cgroup level
- **Signal isolation**: Cannot send signals to processes outside the namespace

This ensures complete process-level security between jobs and from the host system.