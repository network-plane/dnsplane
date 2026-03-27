# Packaging (RPM and Debian)

## Version string (`BASE-SHORTSHA`)

[version.sh](version.sh) prints three values from the git tree:

| Output | Example | Meaning |
| --- | --- | --- |
| `DNSPLANE_VERSION_BASE` | `1.4.175` | Semver core `X.Y.Z` from the latest matching `v*.*.*` tag on `HEAD` (suffixes on the tag name after the third number are ignored, e.g. `v1.4.175-rc1` → `1.4.175`); else [VERSION](../VERSION); else `0.0.0` |
| `DNSPLANE_GIT_SHORT` | `d977f1b` | `git rev-parse --short=7 HEAD` |
| `DNSPLANE_VERSION_FULL` | `1.4.175-d977f1b` | Embedded in the binary (`main.appVersion`) and shown in `%description` / docs |

Usage:

```bash
eval "$(./packaging/version.sh export)"
echo "$DNSPLANE_VERSION_FULL"
./packaging/version.sh full   # print FULL only
```

**RPM:** `Version` is `BASE`; `Release` is `1.SHORTSHA%{?dist}` so the NEVRA is policy-friendly while humans still see `FULL` in the description.

**Debian:** [debian/changelog](../debian/changelog) carries the archive version; the **binary** still reports `FULL` from `version.sh` at build time.

## systemd units

| File | Binary path |
| --- | --- |
| [systemd/dnsplane.service](../systemd/dnsplane.service) | `/usr/local/dnsplane/dnsplane` (manual install) |
| [systemd/dnsplane.service.packaged](../systemd/dnsplane.service.packaged) | `/usr/bin/dnsplane` (RPM/DEB) |

## RPM (Fedora, RHEL-family, openSUSE)

Requirements: `git`, `golang` (see [go.mod](../go.mod); RHEL 9 may need a newer Go from AppStream or module streams), `rpm-build`, `systemd-rpm-macros`.

From the repository root:

```bash
eval "$(./packaging/version.sh export)"
mkdir -p ~/rpmbuild/{SOURCES,SPECS,BUILD,RPMS,SRPMS}
git archive --format=tar --prefix="dnsplane-${DNSPLANE_VERSION_BASE}/" HEAD \
  | gzip -c > ~/rpmbuild/SOURCES/dnsplane-${DNSPLANE_VERSION_BASE}.tar.gz
cp packaging/dnsplane.spec ~/rpmbuild/SPECS/
rpmbuild -ba ~/rpmbuild/SPECS/dnsplane.spec \
  --define "dnsplane_base ${DNSPLANE_VERSION_BASE}" \
  --define "dnsplane_short ${DNSPLANE_GIT_SHORT}"
```

Artifacts under `~/rpmbuild/RPMS/` and `~/rpmbuild/SRPMS/`.

**RHEL / Rocky / Alma:** If the distro’s `golang` is older than required by `go.mod`, install a supported toolchain (vendor docs, EPEL, or upstream Go) before `rpmbuild`.

## Debian / Ubuntu

```bash
sudo apt install build-essential debhelper golang-go git
dpkg-buildpackage -us -uc -b
```

Produces `../dnsplane_*.deb` from the parent directory. Bump `debian/changelog` with `dch` before uploading to a suite.

## CI

GitHub Actions workflow [.github/workflows/package-rpm.yml](../.github/workflows/package-rpm.yml) builds RPMs on Fedora and Rocky Linux with the same spec and version macros.
