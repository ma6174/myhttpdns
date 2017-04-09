package main

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func NewDnspodCli(timeout time.Duration) *DnspodCli {
	return &DnspodCli{
		cli: &http.Client{Timeout: timeout},
	}
}

type DnspodCli struct {
	cli *http.Client
}

func (d *DnspodCli) Query(domain string) (info *TTLInfo) {
	info = &TTLInfo{
		Domain: domain,
		TTLTo:  time.Now().Add(time.Second * 3),
	}
	if strings.HasSuffix(domain, ".") {
		domain = domain[:len(domain)-1]
	}
	query := url.Values{}
	query.Set("dn", domain)
	query.Set("ttl", "1")
	u := "http://119.29.29.29/d?" + query.Encode()
	resp, err := d.cli.Get(u)
	if err != nil {
		info.Err = err
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		info.Err = err
		return
	}
	sp := strings.Split(string(body), ",")
	info.Records = strings.Split(sp[0], ";")
	if len(sp) < 2 {
		return
	}
	if len(info.Records) > 20 {
		info.Records = info.Records[:20]
	}
	ttl, err := strconv.Atoi(sp[1])
	if err != nil {
		info.Err = err
		return
	}
	if ttl > 3600 {
		ttl = 3600
	}
	if ttl <= 0 || len(info.Records) == 0 {
		ttl = 3
	}
	info.TTLTo = time.Now().Add(time.Duration(ttl) * time.Second)
	return
}
