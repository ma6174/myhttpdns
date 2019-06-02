package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func NewCloudflareCli(timeout time.Duration) *DnspodCli {
	return &DnspodCli{
		cli: &http.Client{Timeout: timeout},
	}
}

type cloudflareCli struct {
	cli *http.Client
}

type CloudflareAnswer struct {
	TTL  uint32 `json:"TTL"`
	Data string `json:"data"`
	Name string `json:"name"`
	Type int    `json:"type"`
}

type CloudflareQuestion struct {
	Name string `json:"name"`
	Type int    `json:"type"`
}

// https://developers.cloudflare.com/1.1.1.1/dns-over-https/json-format/
type CloudflareDnsRet struct {
	AD       bool                 `json:"AD"`
	Answer   []CloudflareAnswer   `json:"Answer"`
	CD       bool                 `json:"CD"`
	Question []CloudflareQuestion `json:"Question"`
	RA       bool                 `json:"RA"`
	RD       bool                 `json:"RD"`
	Status   int                  `json:"Status"`
	TC       bool                 `json:"TC"`
}

func (d *cloudflareCli) Query(domain string) (info *TTLInfo) {
	info = &TTLInfo{
		Domain: domain,
		TTLTo:  time.Now().Add(time.Second * 3),
	}
	if strings.HasSuffix(domain, ".") {
		domain = domain[:len(domain)-1]
	}
	query := url.Values{}
	query.Set("name", domain)
	query.Set("type", "A")
	u := "https://1.1.1.1/dns-query?" + query.Encode()
	resp, err := d.cli.Get(u)
	if err != nil {
		log.Println(err)
		info.Err = err
		return
	}
	defer resp.Body.Close()
	var ret CloudflareDnsRet
	err = json.NewDecoder(resp.Body).Decode(&ret)
	if err != nil {
		log.Println(err)
		info.Err = err
		return
	}
	if ret.Status != 0 || len(ret.Answer) == 0 {
		err = errors.New("query failed")
		log.Println(err)
		info.Err = err
		return
	}
	for _, ans := range ret.Answer {
		if ans.TTL > 3600 {
			ans.TTL = 3600
		}
		if ans.TTL <= 0 {
			ans.TTL = 3
		}
		info.TTL = ans.TTL
		info.Domain = ans.Name
		info.TTLTo = time.Now().Add(time.Second * time.Duration(ans.TTL))
		info.Records = append(info.Records, ans.Data)
	}
	return
}
