# dnsresolver

A DNS Server with multiple management interfaces. 

## Usage/Examples

### start with interactive CLI
```bash
./dnsresolver
```

### start as daemon
```bash
./dnsresolver --daemon
```

### start in client mode (it will try to connect to the default unix socket)
```bash
./dnsresolver --client-mode
```


## Config Files

| File | Usage |
| --- | --- |
| records.json | holds dns records |
| servers.json | |
| cache.json | |
| dnsresolver.json | |

## Roadmap

- mDNS Server
- DHCP Server
- REST API
- Remote Client (Over TCP)

## Dependancies & Documentation
[![Go Mod](https://img.shields.io/github/go-mod/go-version/earentir/dnsresolver)]()

[![Go Reference](https://pkg.go.dev/badge/github.com/earentir/dnsresolver.svg)](https://pkg.go.dev/github.com/earentir/dnsresolver)

[![Dependancies](https://img.shields.io/librariesio/github/earentir/dnsresolver)](https://libraries.io/github/earentir/dnsresolver)

[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/8581/badge)](https://www.bestpractices.dev/projects/8581)

[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/earentir/dnsresolver/badge)](https://securityscorecards.dev/viewer/?uri=github.com/earentir/dnsresolver)

![Code Climate issues](https://img.shields.io/codeclimate/tech-debt/earentir/dnsresolver)



## Contributing

Contributions are always welcome!
All contributions are required to follow the https://google.github.io/styleguide/go/

## Vulnerability Reporting

Please report any security vulnerabilities to the project using issues or directly to the owner.

## Code of Conduct
 This project follows the go project code of conduct, please refer to https://go.dev/conduct for more details

## Authors

- [@earentir](https://www.github.com/earentir)


## License

I will always follow the Linux Kernel License as primary, if you require any other OPEN license please let me know and I will try to accomodate it.

[![License](https://img.shields.io/github/license/earentir/gitearelease)](https://opensource.org/license/gpl-2-0)
