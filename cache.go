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
	heap.Push(r.heap, info)
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

func (r *RecordCache) Len() int {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return len(r.cache)
}

func (r *RecordCache) loopEvict() {
	for {
		r.lock.Lock()
		if r.heap.Len() == 0 {
			r.lock.Unlock()
			time.Sleep(time.Second)
			continue
		}
		info := heap.Pop(r.heap).(*TTLInfo)
		sleepTime := info.TTLTo.Sub(time.Now())
		if sleepTime > time.Second {
			heap.Push(r.heap, info)
			r.lock.Unlock()
			time.Sleep(time.Second)
			continue
		} else if sleepTime <= 0 {
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
