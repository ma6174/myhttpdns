package main

import (
	"container/heap"
	"sync"
	"time"
)

func NewRecordCache() (cache *RecordCache) {
	var h *TTLHeap = &TTLHeap{}
	heap.Init(h)
	cache = &RecordCache{
		cache: make(map[string]*TTLInfo),
		heap:  h,
	}
	go cache.loopEvict()
	return
}

type TTLHeap []*TTLInfo

func (h TTLHeap) Len() int           { return len(h) }
func (h TTLHeap) Less(i, j int) bool { return h[i].TTLTo.Sub(h[j].TTLTo) < 0 }
func (h TTLHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *TTLHeap) Push(x interface{}) {
	*h = append(*h, x.(*TTLInfo))
}

func (h *TTLHeap) Pop() interface{} {
	old := *h
	n := len(old)
	if n == 0 {
		return nil
	}
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type TTLInfo struct {
	Domain  string
	TTLTo   time.Time
	Records []string
	Err     error
}

type RecordCache struct {
	lock  sync.RWMutex
	cache map[string]*TTLInfo
	heap  heap.Interface
}

func (r *RecordCache) Put(info *TTLInfo) {
	r.lock.Lock()
	r.cache[info.Domain] = info
	r.heap.Push(info)
	r.lock.Unlock()
}

func (r *RecordCache) Get(domain string) *TTLInfo {
	r.lock.RLock()
	defer r.lock.RUnlock()
	if info, ok := r.cache[domain]; ok {
		return info
	}
	return nil
}

func (r *RecordCache) loopEvict() {
	for {
		r.lock.Lock()
		i := r.heap.Pop()
		if i == nil {
			r.lock.Unlock()
			time.Sleep(time.Millisecond * 500)
			continue
		}
		info := i.(*TTLInfo)
		sleepTime := info.TTLTo.Sub(time.Now())
		if sleepTime > time.Second {
			r.heap.Push(info)
			r.lock.Unlock()
			time.Sleep(time.Millisecond * 500)
			continue
		} else if sleepTime < 0 {
			delete(r.cache, info.Domain)
			r.lock.Unlock()
			continue
		}
		r.lock.Unlock()
		time.Sleep(sleepTime)
		r.lock.Lock()
		delete(r.cache, info.Domain)
		r.lock.Unlock()
	}
}
