# Optional host tuning (Linux)

dnsplane UDP latency is usually dominated by the resolver process. These kernel/host tweaks can reduce **tail latency** or **loss under burst** on small devices (Pi, router, VPS).

## Before changing anything

1. Baseline: `dig @<resolver> example.com +stats` many times; note p50/p95 Query time.
2. Check drops: `netstat -s | grep -i drop` / `ss -u -m` (UDP memory pressure).
3. After changes, re-measure the same workload.

## CPU frequency (often the biggest win on SBCs)

- Set governor to `performance` while benchmarking or if you care about steady low latency:
  - `echo performance | sudo tee /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor`
- Revert to `ondemand` or `schedutil` for power saving when not needed.

## UDP / socket buffers (Linux)

Example **non-persistent** sysctl (tune to your RAM; values are illustrative):

```bash
sudo sysctl -w net.core.rmem_max=8388608
sudo sysctl -w net.core.wmem_max=8388608
```

Persist in `/etc/sysctl.d/99-dnsplane.conf` only after validating. **Do not** set huge buffers on memory-constrained systems.

## File descriptors

Under high concurrency, raise limits for the dnsplane service, e.g. in systemd:

```ini
LimitNOFILE=65535
```

## Containers

- **hostNetwork: true** (or equivalent) avoids extra NAT/bridge latency for DNS UDP.
- Avoid **hard CPU caps** that throttle the resolver process.

## Go runtime

Usually leave `GOMAXPROCS` default. Pinning CPUs rarely helps unless you have measured scheduler contention.

## macOS / Windows

Little to tune for a small DNS server; focus on code and network path.
