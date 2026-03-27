# Roadmap and backlog

Work tracked for future releases and packaging. For **using** dnsplane, see the [README](README.md).

---

## 1. OS packages (RPM, DEB)

- [x] RPM spec (`packaging/dnsplane.spec`): binary under `%{_bindir}`, FHS systemd unit from `systemd/dnsplane.service.packaged`; version macros + `packaging/version.sh` (see [packaging/README.md](packaging/README.md)); CI [.github/workflows/package-rpm.yml](.github/workflows/package-rpm.yml) (Fedora + Rocky).
- [x] Debian packaging (`debian/`): control, rules, changelog, install paths; same version script and packaged unit.
- [ ] Optional: Alpine APK, Homebrew formula, Windows MSI or zip.

## 5. Web UI for Configuration
