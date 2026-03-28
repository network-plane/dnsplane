# Roadmap and backlog

Work tracked for future releases and packaging. For **using** dnsplane, see the [README](README.md).

---

## 1. OS packages (RPM, DEB)

- [x] RPM spec (`packaging/dnsplane.spec`): binary under `%{_bindir}`, FHS systemd unit from `systemd/dnsplane.service.packaged`; version macros + `packaging/version.sh` (see [packaging/README.md](packaging/README.md)); CI [.github/workflows/package-rpm.yml](.github/workflows/package-rpm.yml) (Fedora + Rocky).
- [x] Debian packaging (`debian/`): control, rules, changelog, install paths; same version script and packaged unit.
- [ ] Optional: Alpine APK, Homebrew formula, Windows MSI or zip.

## 2. Authentication
- [ ] Add authentication to the tui client and encryption, it needs to be fully encrypted and authenticated. (this gets enabled in the config, by default it is disabled)
- [ ] Add authentication to the api and encryption, it needs to be fully encrypted and authenticated. (this gets enabled in the config, by default it is disabled)

## 3. Various improvements
- [ ] Add A page in dashboard where the user can run a request, it will have an edit and the user types the IP, Domain they want and choose the type of the request to do, it will do the request as if the user did a dns req to the dns server, it will show a table with all the details, what replied (cache, local, upstream, none) and the time it took to reply and the actual reply. the user should also have a dropdown to select Normal (the request follows the normal path as if it was a client doing a request, then it should have cache, local, upstream, custom) so we do a req to the cache only, the local, upstreams etc for custom you need to ask the dns server to use

## 5. Web UI for Configuration
