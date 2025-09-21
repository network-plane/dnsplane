CGO_ENABLED=0 go build && sudo setcap 'cap_net_bind_service,cap_net_raw=+ep' ./dnsresolver
getcap ./dnsresolver
