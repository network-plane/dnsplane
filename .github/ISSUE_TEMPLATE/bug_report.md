---
name: Bug report
about: Report something that broke or behaves incorrectly
title: ''
labels: ''
assignees: ''

---

**What went wrong**  
Short description of the symptom (wrong answer, crash, hang, error message).

**How to reproduce**  
1. `dnsplane` version (from `dnsplane server` or `GET /version`).
2. OS and architecture (e.g. Linux x86_64, macOS arm64).
3. How you run it (e.g. `./dnsplane server`, systemd, container).
4. Minimal config or flags: `dnsplane.json` snippets, or `--config` / `--port` / `--dnsrecords` as relevant.
5. Query or command that triggers the issue (e.g. `dig @127.0.0.1 example.com A`).

**What you expected**  
What should have happened instead.

**Logs / output**  
Relevant lines from `dnsserver.log`, `apiserver.log`, or terminal output (redact tokens and private IPs).

**Extra context**  
Anything else that helps (upstream provider, DNSSEC on/off, DoT/DoH, etc.).
