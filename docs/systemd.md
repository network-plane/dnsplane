# Running as a systemd service

A systemd unit file is provided under `systemd/dnsplane.service`. It runs the binary from `/usr/local/dnsplane/` with the **server** command and passes config and data paths under `/etc/dnsplane/` (`--config` plus `--dnsservers`, `--dnsrecords`, `--cache`). dnsplane does not use `/etc` unless you use this unit or pass those paths yourself.

1. Install the binary: place the `dnsplane` executable at `/usr/local/dnsplane/dnsplane`.
2. Copy the unit file: `cp systemd/dnsplane.service /etc/systemd/system/`.
3. Create the config directory: `mkdir -p /etc/dnsplane`.
4. Reload and enable: `systemctl daemon-reload && systemctl enable --now dnsplane.service`.

When the service runs, it will create default `dnsplane.json` and JSON data files in `/etc/dnsplane/` if they are missing, because the unit file passes those paths. Ensure the service user (e.g. root) can write to that directory for the first start. To run as an unprivileged user, see the comments in `systemd/dnsplane.service`: create a `dnsplane` user, set `User=`/`Group=`, add `StateDirectory=dnsplane`, and use data paths under `/var/lib/dnsplane`.
