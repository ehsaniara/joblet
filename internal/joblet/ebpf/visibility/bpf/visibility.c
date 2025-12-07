// SPDX-License-Identifier: GPL-2.0
// eBPF program for job visibility - tracks process execution and network connections
// Filters events by cgroup ID to only monitor job processes

//go:build ignore

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

char LICENSE[] SEC("license") = "GPL";

#define TASK_COMM_LEN 16
#define MAX_ARGS 16
#define MAX_ARG_LEN 128
#define MAX_PATH_LEN 256

// Event types matching Go TelemetryType
#define EVENT_EXEC 1
#define EVENT_CONNECT 2
#define EVENT_ACCEPT 3
#define EVENT_SOCKET_DATA 4
#define EVENT_MMAP 5
#define EVENT_MPROTECT 6

// Exec event data
struct exec_event {
    __u64 timestamp;
    __u64 cgroup_id;
    __u32 pid;
    __u32 ppid;
    __u32 uid;
    __u8 comm[TASK_COMM_LEN];
    __u8 filename[MAX_PATH_LEN];
    __s32 retval;  // Set on exit
};

// Connect event data
struct connect_event {
    __u64 timestamp;
    __u64 cgroup_id;
    __u32 pid;
    __u16 port;
    __u16 family;  // AF_INET or AF_INET6
    __u8 protocol; // IPPROTO_TCP or IPPROTO_UDP
    __u8 pad[3];
    union {
        __u32 v4_addr;
        __u8 v6_addr[16];
    } addr;
};

// Accept event data (incoming connections)
struct accept_event {
    __u64 timestamp;
    __u64 cgroup_id;
    __u32 pid;
    __u16 local_port;
    __u16 remote_port;
    __u16 family;
    __u8 pad[2];
    union {
        __u32 v4_addr;
        __u8 v6_addr[16];
    } remote_addr;
};

// Socket data event (sendto/recvfrom)
struct socket_data_event {
    __u64 timestamp;
    __u64 cgroup_id;
    __u32 pid;
    __u16 port;
    __u16 family;
    __u8 direction;  // 0 = send, 1 = recv
    __u8 protocol;
    __u8 pad[2];
    __u64 bytes;
    union {
        __u32 v4_addr;
        __u8 v6_addr[16];
    } addr;
};

// Mmap event data
struct mmap_event {
    __u64 timestamp;
    __u64 cgroup_id;
    __u32 pid;
    __u32 prot;      // PROT_READ, PROT_WRITE, PROT_EXEC
    __u32 flags;     // MAP_SHARED, MAP_PRIVATE, MAP_ANONYMOUS
    __u32 pad;
    __u64 addr;
    __u64 length;
};

// Mprotect event data
struct mprotect_event {
    __u64 timestamp;
    __u64 cgroup_id;
    __u32 pid;
    __u32 prot;      // New protection flags
    __u64 addr;
    __u64 length;
};

// Map to store cgroup IDs we're monitoring
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, __u64);   // cgroup_id
    __type(value, __u8);  // job marker (1 = monitoring)
} monitored_cgroups SEC(".maps");

// Ring buffer for exec events
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024); // 256KB
} exec_events SEC(".maps");

// Ring buffer for connect events
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024); // 256KB
} connect_events SEC(".maps");

// Ring buffer for accept events
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024); // 256KB
} accept_events SEC(".maps");

// Ring buffer for socket data events
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024); // 256KB
} socket_data_events SEC(".maps");

// Ring buffer for mmap events
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024); // 256KB
} mmap_events SEC(".maps");

// Ring buffer for mprotect events
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024); // 256KB
} mprotect_events SEC(".maps");

// Helper to check if current process is in a monitored cgroup
static __always_inline int is_monitored() {
    __u64 cgroup_id = bpf_get_current_cgroup_id();
    __u8 *marker = bpf_map_lookup_elem(&monitored_cgroups, &cgroup_id);
    return marker != NULL;
}

// Tracepoint for execve syscall enter
SEC("tracepoint/syscalls/sys_enter_execve")
int tracepoint__syscalls__sys_enter_execve(struct trace_event_raw_sys_enter *ctx) {
    if (!is_monitored())
        return 0;

    struct exec_event *event;
    event = bpf_ringbuf_reserve(&exec_events, sizeof(*event), 0);
    if (!event)
        return 0;

    event->timestamp = bpf_ktime_get_ns();
    event->cgroup_id = bpf_get_current_cgroup_id();
    event->pid = bpf_get_current_pid_tgid() >> 32;

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    event->ppid = BPF_CORE_READ(task, real_parent, tgid);
    event->uid = bpf_get_current_uid_gid() & 0xFFFFFFFF;

    bpf_get_current_comm(&event->comm, sizeof(event->comm));

    // Read filename from syscall args
    const char *filename = (const char *)ctx->args[0];
    bpf_probe_read_user_str(&event->filename, sizeof(event->filename), filename);

    event->retval = 0; // Will be set on exit

    bpf_ringbuf_submit(event, 0);
    return 0;
}

// Tracepoint for connect syscall enter
SEC("tracepoint/syscalls/sys_enter_connect")
int tracepoint__syscalls__sys_enter_connect(struct trace_event_raw_sys_enter *ctx) {
    if (!is_monitored())
        return 0;

    struct sockaddr *addr = (struct sockaddr *)ctx->args[1];

    if (!addr)
        return 0;

    // Read address family
    __u16 family;
    bpf_probe_read_user(&family, sizeof(family), &addr->sa_family);

    // Only track IPv4 and IPv6
    if (family != 2 /* AF_INET */ && family != 10 /* AF_INET6 */)
        return 0;

    struct connect_event *event;
    event = bpf_ringbuf_reserve(&connect_events, sizeof(*event), 0);
    if (!event)
        return 0;

    event->timestamp = bpf_ktime_get_ns();
    event->cgroup_id = bpf_get_current_cgroup_id();
    event->pid = bpf_get_current_pid_tgid() >> 32;
    event->family = family;

    if (family == 2) { // AF_INET
        struct sockaddr_in *sin = (struct sockaddr_in *)addr;
        bpf_probe_read_user(&event->port, sizeof(event->port), &sin->sin_port);
        bpf_probe_read_user(&event->addr.v4_addr, sizeof(event->addr.v4_addr), &sin->sin_addr);
    } else { // AF_INET6
        struct sockaddr_in6 *sin6 = (struct sockaddr_in6 *)addr;
        bpf_probe_read_user(&event->port, sizeof(event->port), &sin6->sin6_port);
        bpf_probe_read_user(&event->addr.v6_addr, sizeof(event->addr.v6_addr), &sin6->sin6_addr);
    }

    // Get protocol from socket - simplified, assume TCP for now
    event->protocol = 6; // IPPROTO_TCP

    bpf_ringbuf_submit(event, 0);
    return 0;
}

// Tracepoint for accept4 syscall (accept is implemented via accept4)
SEC("tracepoint/syscalls/sys_exit_accept4")
int tracepoint__syscalls__sys_exit_accept4(struct trace_event_raw_sys_exit *ctx) {
    if (!is_monitored())
        return 0;

    // Only process successful accepts
    if (ctx->ret < 0)
        return 0;

    struct accept_event *event;
    event = bpf_ringbuf_reserve(&accept_events, sizeof(*event), 0);
    if (!event)
        return 0;

    event->timestamp = bpf_ktime_get_ns();
    event->cgroup_id = bpf_get_current_cgroup_id();
    event->pid = bpf_get_current_pid_tgid() >> 32;
    event->family = 2; // Simplified - assume AF_INET
    event->local_port = 0;
    event->remote_port = 0;

    bpf_ringbuf_submit(event, 0);
    return 0;
}

// Tracepoint for sendto syscall
SEC("tracepoint/syscalls/sys_enter_sendto")
int tracepoint__syscalls__sys_enter_sendto(struct trace_event_raw_sys_enter *ctx) {
    if (!is_monitored())
        return 0;

    struct socket_data_event *event;
    event = bpf_ringbuf_reserve(&socket_data_events, sizeof(*event), 0);
    if (!event)
        return 0;

    event->timestamp = bpf_ktime_get_ns();
    event->cgroup_id = bpf_get_current_cgroup_id();
    event->pid = bpf_get_current_pid_tgid() >> 32;
    event->direction = 0; // send
    event->bytes = ctx->args[2]; // len argument
    event->protocol = 6; // Assume TCP

    // Try to get destination address from args[4] if provided
    struct sockaddr *addr = (struct sockaddr *)ctx->args[4];
    if (addr) {
        __u16 family;
        bpf_probe_read_user(&family, sizeof(family), &addr->sa_family);
        event->family = family;
        if (family == 2) { // AF_INET
            struct sockaddr_in *sin = (struct sockaddr_in *)addr;
            bpf_probe_read_user(&event->port, sizeof(event->port), &sin->sin_port);
            bpf_probe_read_user(&event->addr.v4_addr, sizeof(event->addr.v4_addr), &sin->sin_addr);
        }
    }

    bpf_ringbuf_submit(event, 0);
    return 0;
}

// Tracepoint for recvfrom syscall
SEC("tracepoint/syscalls/sys_enter_recvfrom")
int tracepoint__syscalls__sys_enter_recvfrom(struct trace_event_raw_sys_enter *ctx) {
    if (!is_monitored())
        return 0;

    struct socket_data_event *event;
    event = bpf_ringbuf_reserve(&socket_data_events, sizeof(*event), 0);
    if (!event)
        return 0;

    event->timestamp = bpf_ktime_get_ns();
    event->cgroup_id = bpf_get_current_cgroup_id();
    event->pid = bpf_get_current_pid_tgid() >> 32;
    event->direction = 1; // recv
    event->bytes = ctx->args[2]; // len argument (buffer size)
    event->protocol = 6; // Assume TCP

    bpf_ringbuf_submit(event, 0);
    return 0;
}

// Tracepoint for mmap syscall
SEC("tracepoint/syscalls/sys_enter_mmap")
int tracepoint__syscalls__sys_enter_mmap(struct trace_event_raw_sys_enter *ctx) {
    if (!is_monitored())
        return 0;

    // mmap args: addr, length, prot, flags, fd, offset
    __u32 prot = (__u32)ctx->args[2];
    __u32 flags = (__u32)ctx->args[3];

    // Only capture executable mappings or file-backed mappings
    // Skip anonymous non-executable mappings to reduce noise
    if (!(prot & 4 /* PROT_EXEC */) && (flags & 0x20 /* MAP_ANONYMOUS */))
        return 0;

    struct mmap_event *event;
    event = bpf_ringbuf_reserve(&mmap_events, sizeof(*event), 0);
    if (!event)
        return 0;

    event->timestamp = bpf_ktime_get_ns();
    event->cgroup_id = bpf_get_current_cgroup_id();
    event->pid = bpf_get_current_pid_tgid() >> 32;
    event->addr = ctx->args[0];
    event->length = ctx->args[1];
    event->prot = prot;
    event->flags = flags;

    bpf_ringbuf_submit(event, 0);
    return 0;
}

// Tracepoint for mprotect syscall
SEC("tracepoint/syscalls/sys_enter_mprotect")
int tracepoint__syscalls__sys_enter_mprotect(struct trace_event_raw_sys_enter *ctx) {
    if (!is_monitored())
        return 0;

    __u32 prot = (__u32)ctx->args[2];

    // Only capture when adding executable permission (security-relevant)
    if (!(prot & 4 /* PROT_EXEC */))
        return 0;

    struct mprotect_event *event;
    event = bpf_ringbuf_reserve(&mprotect_events, sizeof(*event), 0);
    if (!event)
        return 0;

    event->timestamp = bpf_ktime_get_ns();
    event->cgroup_id = bpf_get_current_cgroup_id();
    event->pid = bpf_get_current_pid_tgid() >> 32;
    event->addr = ctx->args[0];
    event->length = ctx->args[1];
    event->prot = prot;

    bpf_ringbuf_submit(event, 0);
    return 0;
}
