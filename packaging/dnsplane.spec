# dnsplane RPM spec — pass macros at build time:
#   rpmbuild -ba dnsplane.spec \
#     --define "dnsplane_base 1.4.175" \
#     --define "dnsplane_short d977f1b"
# Source tarball must extract to dnsplane-%{version}/ (see packaging/README.md).

%global debug_package %{nil}

Name:           dnsplane
Version:        %{dnsplane_base}
Release:        1.%{dnsplane_short}%{?dist}
Summary:        DNS resolver (dnsplane)
License:        GPL-2.0-only
URL:            https://github.com/network-plane/dnsplane
Source0:        %{name}-%{version}.tar.gz

# Minimum Go package version for RPM dependency checks.
# CI passes --define dnsplane_go_min from packaging/version.sh (major.minor floor).
%{!?dnsplane_go_min:%global dnsplane_go_min 1.26}
BuildRequires:  golang >= %{dnsplane_go_min}
BuildRequires:  git
BuildRequires:  systemd-rpm-macros

%define dnsplane_full %{dnsplane_base}-%{dnsplane_short}

%description
dnsplane is a DNS resolver with optional local records, DoT/DoH, and clustering.

Packaged build id (version-git): %{dnsplane_full}
Build expects Go >= %{dnsplane_go_min} per go.mod (use matching toolchain on PATH).

%prep
%autosetup -n %{name}-%{version}

%build
export CGO_ENABLED=0
export GOFLAGS=-buildvcs=false
go version
go build -trimpath -ldflags "-X main.appVersion=%{dnsplane_full}" -o dnsplane .

%install
install -D -p -m 0755 dnsplane %{buildroot}%{_bindir}/dnsplane
install -D -p -m 0644 systemd/dnsplane.service.packaged %{buildroot}%{_unitdir}/dnsplane.service

%files
%license LICENSE
%{_bindir}/dnsplane
%{_unitdir}/dnsplane.service

%post
%systemd_post %{name}.service

%preun
%systemd_preun %{name}.service

%postun
%systemd_postun %{name}.service

%changelog
* Wed Jan 01 2025 dnsplane packaging <https://github.com/network-plane/dnsplane> - 0.0.0-1
- Spec skeleton; real Version/Release come from rpmbuild --define (see packaging/README.md)
