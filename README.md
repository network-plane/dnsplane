# dnsplane
![dnsplane](https://github.com/user-attachments/assets/38214dcd-ca33-41ce-a88f-7edad7d85822)

A non standard DNS Server with multiple management interfaces. Its main function is it will do the same dns request to multiple DNS servers, if any of the servers replies with an authoritative reply it chooses that, otherwise it provides the reply from the fallback dns server. This will help in case for example you have a local dns server and you connect to work over a VPN and use that DNS server at the same time.

## Diagram
![image](https://github.com/earentir/dnsplane/assets/97396839/79acfa3e-8b83-48ec-92be-ed99085b2cc5)



## Usage/Examples

### start as daemon (default)
```bash
./dnsplane
```
The daemon keeps the resolver running, exposes the UNIX control socket at `/tmp/dnsplane.socket`, and listens for remote TUI clients on TCP port `8053` by default.

### start in client mode (connects to the default unix socket unless overridden)
```bash
./dnsplane --client
# or specify a custom socket path
./dnsplane --client /tmp/dnsplane.sock
```

### connect to a remote resolver over TCP (default port 8053)
```bash
./dnsplane --client 192.168.178.40
./dnsplane --client 192.168.178.40:8053
```

### change the server socket path
```bash
./dnsplane --server-socket /tmp/custom.sock
```

### change the TCP TUI listener
```bash
./dnsplane --server-tcp 0.0.0.0:9000
```

### Recording of clearing and adding dns records
https://github.com/user-attachments/assets/f5ca52cb-3874-499c-a594-ba3bf64b3ba9


## Config Files

| File | Usage |
| --- | --- |
| dnsrecords.json | holds dns records |
| dnsservers.json | holds the dns servers used for queries |
| dnscache.json | holds queries already done if their ttl diff is still above 0 |
| dnsplane.json | the app config |

## Roadmap

- REST API enhancements
- Remote Client (Over TCP)

## Dependancies & Documentation
[![Go Mod](https://img.shields.io/github/go-mod/go-version/earentir/dnsplane?style=for-the-badge)]()

[![Go Reference](https://pkg.go.dev/badge/github.com/earentir/dnsplane.svg)](https://pkg.go.dev/github.com/earentir/dnsplane)

[![Dependancies](https://img.shields.io/librariesio/github/earentir/dnsplane?style=for-the-badge)](https://libraries.io/github/earentir/dnsplane)

[![OpenSSF Best Practices](https://bestpractices.coreinfrastructure.org/projects/8887/badge)](https://www.bestpractices.dev/projects/8887)

[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/earentir/dnsplane/badge)](https://securityscorecards.dev/viewer/?uri=github.com/earentir/dnsplane)

![Code Climate issues](https://img.shields.io/codeclimate/tech-debt/earentir/dnsplane?style=for-the-badge)

![GitHub commit activity](https://img.shields.io/github/commit-activity/m/earentir/dnsplane?style=for-the-badge)


## Contributing

Contributions are always welcome!
All contributions are required to follow the https://google.github.io/styleguide/go/


## Authors

- [@earentir](https://www.github.com/earentir)


## License

I will always follow the Linux Kernel License as primary, if you require any other OPEN license please let me know and I will try to accomodate it.

[![License](https://img.shields.io/github/license/earentir/gitearelease)](https://opensource.org/license/gpl-2-0)
