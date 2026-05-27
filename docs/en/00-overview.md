# Overview

`kiro_waf` is a single-server protection product for Ubuntu 22.04 LTS. It is
designed for security providers who need licensing, support, updates, and clear
customer/server ownership without building a complex cluster.

The MVP does not use SQL. Configuration, licenses, provider key lists, server
records, health reports, incidents, and update manifests are stored as YAML,
JSON, or JSONL files. Signed files are used where integrity matters.

## Goals

- Drop bad traffic as early as possible.
- Protect conntrack, proxy, app workers, logs, and application databases from
  overload.
- Support `server` and `full` operating modes.
- Support Cloudflare Free as an optional website front layer.
- Keep provider operations simple and file-based.

