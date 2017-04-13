package main

import (
	"flag"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/golang/groupcache/singleflight"
	"github.com/miekg/dns"
)

func main() {
	bind := flag.String("bind", "0.0.0.0:53", "server bind addr")
	hostfiless := flag.String("hosts", "/etc/hosts", "hosts file, file1,file2,file3...")
	flag.Parse()
	log.Println("dns server running at", *bind)
	server := &dns.Server{Addr: *bind, Net: "udp"}
	handler := NewCacheHandler(strings.Split(*hostfiless, ","))
	dns.HandleFunc(".", handler.handleRequest)
	log.Fatal(server.ListenAndServe())
}

type DnsQueryer interface {
	Query(domain string) *TTLInfo
}

func NewCacheHandler(hostfiles []string) *CachedHandler {
	hosts := ParseHostsFiles(hostfiles)
	log.Printf("load %d hosts from hosts file", len(hosts))
	ch := &CachedHandler{
		cache:   NewRecordCache(),
		backend: NewDnspodCli(time.Second * 3),
		group:   &singleflight.Group{},
		hosts:   hosts,
	}
	return ch
}

type CachedHandler struct {
	lock       sync.RWMutex
	group      *singleflight.Group
	cache      *RecordCache
	backend    DnsQueryer
	hosts      Hosts
	hostfiless []string
}

func (s *CachedHandler) loopUpdateHosts() {
	for {
		time.Sleep(time.Second * 5)
		hosts := ParseHostsFiles(s.hostfiless)
		s.lock.Lock()
		s.hosts = hosts
		s.lock.Unlock()
	}
}
func (s *CachedHandler) GetFromHostFile(domain string) *TTLInfo {
	s.lock.RLock()
	defer s.lock.RUnlock()
	d := domain[:len(domain)-1]
	if ip, ok := s.hosts[d]; ok {
		return &TTLInfo{
			Records: []string{ip.String()},
			TTL:     600,
			TTLTo:   time.Now().Add(time.Second * 601),
			Domain:  domain,
		}
	}
	return nil
}

func (s *CachedHandler) handleRequest(w dns.ResponseWriter, r *dns.Msg) {
	if len(r.Question) == 0 {
		return
	}
	var (
		info   *TTLInfo
		start  time.Time = time.Now()
		domain string    = r.Question[0].Name
	)
	defer func() {
		m := s.genReply(info)
		m.SetReply(r)
		err := w.WriteMsg(m)
		from := strings.Split(w.RemoteAddr().String(), ":")[0]
		domain = domain[:len(domain)-1]
		if info.Err != nil {
			log.Printf("%v\t%d\t%v\t%v", from, s.cache.Len(), domain, info.Err)
			return
		}
		if err != nil {
			log.Printf("%v\t%d\t%v\t%v", from, s.cache.Len(), domain, err)
			return
		}
		log.Printf("%v\t%d\t%vs\t%.3fms\t%v\t%v", from, s.cache.Len(),
			info.TTL, time.Since(start).Seconds()*1000, domain, info.Records)
	}()
	info = s.GetFromHostFile(domain)
	if info != nil {
		return
	}
	info = s.cache.Get(domain)
	if info != nil {
		return
	}
	i, _ := s.group.Do(domain, func() (interface{}, error) {
		info := s.backend.Query(domain)
		s.cache.Put(info)
		return info, nil
	})
	info = i.(*TTLInfo)
	if info != nil {
		return
	}
	i, _ = s.group.Do(domain, func() (interface{}, error) {
		info := QueryFromDNSServer(domain)
		s.cache.Put(info)
		return info, nil
	})
	info = i.(*TTLInfo)
}

func (s *CachedHandler) genReply(info *TTLInfo) *dns.Msg {
	m := new(dns.Msg)
	for _, record := range info.Records {
		now := time.Now()
		if now.Before(info.TTLTo) {
			info.TTL = uint32(info.TTLTo.Sub(now).Seconds())
		}
		rr := &dns.A{
			Hdr: dns.RR_Header{
				Name:   info.Domain,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    info.TTL,
			},
			A: net.ParseIP(record).To4(),
		}
		m.Answer = append(m.Answer, rr)
	}
	return m
}
