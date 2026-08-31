# Compatibility

Joblet speaks the `joblet-proto` gRPC contract. The **proto major** is the
compatibility boundary: proto **v1.x** and **v2.x** do **not** interoperate.
Every client must target the same proto major as the running server. Release
versions are tracked by **git tag**; treat the tag as authoritative.

> The **rnx** CLI lives in its own repository,
> [joblet-rnx](https://github.com/ehsaniara/joblet-rnx), and versions
> independently; any rnx release works with any Joblet release on the same
> proto major. (Through Joblet v5.6.x, rnx was bundled in this repo and shared
> its version number.)

## Joblet server ↔ proto

| Joblet server        | joblet-proto |
|----------------------|--------------|
| **v5.0.2+** (current)| v2.x         |
| v4.5.0 – v5.0.1      | v1.x         |

Latest: Joblet server **v5.6.11** ↔ proto **v2.6.0** (the version this server
build depends on).

## Client tools

Client tools derive their compatibility from the proto major they target:

| Consumer     | Current | joblet-proto |
|--------------|---------|--------------|
| rnx CLI      | v6.0.0+ | v2.x         |
| Python SDK   | v2.5.2  | v2.x         |
| MCP Server   | v1.1.4  | v2.x         |
| joblet-admin | v1.0.6  | v2.x         |

The authoritative cross-project matrix and the per-RPC feature timeline live in
[joblet-proto/COMPATIBILITY.md](https://github.com/ehsaniara/joblet-proto/blob/main/COMPATIBILITY.md).
