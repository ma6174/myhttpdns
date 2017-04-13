package main

import (
	"net"
	"strings"
	"time"
)

func QueryFromDNSServer(domain string) (info *TTLInfo) {
	info = &TTLInfo{
		Domain: domain,
		TTLTo:  time.Now().Add(time.Second * 3),
	}
	if strings.HasSuffix(domain, ".") {
		domain = domain[:len(domain)-1]
	}
	info.Records, info.Err = net.LookupHost(domain)
	info.TTL = 600
	info.TTLTo = time.Now().Add(time.Duration(601) * time.Second)
	return
}
