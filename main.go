package main

import (
	"flag"
	"log"
	"net"
	"strings"
	"time"

	"github.com/golang/groupcache/singleflight"
	"github.com/miekg/dns"
)

func main() {
	bind := flag.String("bind", "0.0.0.0:53", "server bind addr")
	flag.Parse()
	log.Println("dns server running at", *bind)
	server := &dns.Server{Addr: *bind, Net: "udp"}
	handler := NewCacheHandler()
	dns.HandleFunc(".", handler.handleRequest)
	log.Fatal(server.ListenAndServe())
}

type DnsQueryer interface {
	Query(domain string) *TTLInfo
}

func NewCacheHandler() *CachedHandler {
	return &CachedHandler{
		cache:   NewRecordCache(),
		backend: NewDnspodCli(time.Second * 3),
		group:   &singleflight.Group{},
	}
}

type CachedHandler struct {
	group   *singleflight.Group
	cache   *RecordCache
	backend DnsQueryer
}

func (s *CachedHandler) handleRequest(w dns.ResponseWriter, r *dns.Msg) {
	if len(r.Question) == 0 {
		return
	}
	var (
		info   *TTLInfo
		ttl    uint32
		domain string    = r.Question[0].Name
		start  time.Time = time.Now()
	)
	defer func() {
		from := strings.Split(w.RemoteAddr().String(), ":")[0]
		domain = domain[:len(domain)-1]
		if info.Err != nil {
			log.Printf("%v\t%d\t%v\t%v", from, s.cache.Len(), domain, info.Err)
			return
		}
		log.Printf("%v\t%d\t%vs\t%.3fms\t%v\t%v", from, s.cache.Len(),
			ttl, time.Since(start).Seconds()*1000, domain, info.Records)
	}()
	info = s.cache.Get(domain)
	if info == nil {
		i, _ := s.group.Do(domain, func() (interface{}, error) {
			info := s.backend.Query(domain)
			s.cache.Put(info)
			return info, nil
		})
		info = i.(*TTLInfo)
		if info.Err != nil {
			return
		}
	}
	m := new(dns.Msg)
	m.SetReply(r)
	for _, record := range info.Records {
		now := time.Now()
		if now.Before(info.TTLTo) {
			ttl = uint32(info.TTLTo.Sub(now).Seconds())
		}
		rr := &dns.A{
			Hdr: dns.RR_Header{
				Name:   domain,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    ttl,
			},
			A: net.ParseIP(record).To4(),
		}
		m.Answer = append(m.Answer, rr)
	}
	w.WriteMsg(m)
}
