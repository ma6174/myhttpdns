package main

import (
	"io/ioutil"
	"log"
	"net"
	"strings"
)

type Hosts map[string]net.IP

func ParseHost(filename string, hosts Hosts) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Println("[WARN] open hosts fila failed", err)
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ip := net.ParseIP(fields[0])
		if ip == nil {
			continue
		}
		for _, domain := range fields[1:] {
			if _, ok := hosts[domain]; ok {
				continue
			}
			hosts[domain] = ip
		}
	}
}

func ParseHostsFiles(filenames []string) (hosts Hosts) {
	hosts = make(Hosts)
	for _, fn := range filenames {
		ParseHost(fn, hosts)
	}
	return
}
