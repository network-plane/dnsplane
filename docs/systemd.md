# Running as a systemd service

dnsplane ships two unit templates; pick the one that matches how you installed the binary.

| Install type | Unit file | `ExecStart` binary path |
| --- | --- | --- |
| **Manual** (tarball / `go install`) | [`systemd/dnsplane.service`](../systemd/dnsplane.service) | `/usr/local/dnsplane/dnsplane` |
| **OS package** (RPM / DEB) | [`systemd/dnsplane.service.packaged`](../systemd/dnsplane.service.packaged) → installed as `/lib/systemd/system/dnsplane.service` | `/usr/bin/dnsplane` |

Both use the same config and data paths under `/etc/dnsplane/` (`--config`, `--dnsservers`, `--dnsrecords`, `--cache`) and `/run/dnsplane/` for the server socket. dnsplane does not use `/etc` unless you use one of these units or pass those paths yourself.

## Manual install (`/usr/local/dnsplane`)

1. Install the binary at `/usr/local/dnsplane/dnsplane`.
2. `cp systemd/dnsplane.service /etc/systemd/system/`.
3. `mkdir -p /etc/dnsplane`.
4. `systemctl daemon-reload && systemctl enable --now dnsplane.service`.

## Package install (`/usr/bin`)

Packages install the unit from `dnsplane.service.packaged` as `dnsplane.service` (see [packaging/README.md](../packaging/README.md)).

When the service runs, it creates default `dnsplane.json` and JSON data files under `/etc/dnsplane/` if they are missing. Ensure the service user can write there on first start. For an unprivileged `dnsplane` user, data under `/var/lib/dnsplane`, and `User=` / `Group=` / `StateDirectory=`, see the comments in [`systemd/dnsplane.service`](../systemd/dnsplane.service) (same applies using `/usr/bin/dnsplane` in `ExecStart`).
