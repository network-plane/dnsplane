package dnsserver

// GetDNSArray returns an array of DNS servers in the format "Address:Port".
func GetDNSArray(dnsServers []DNSServer) []string {
	var dnsArray []string
	for _, dnsServer := range dnsServers {
		dnsArray = append(dnsArray, dnsServer.Address+":"+dnsServer.Port)
	}
	return dnsArray
}

func Add() {

}
