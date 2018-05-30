package urlfilter

import (
	"net/url"
	"sync"
)

type URLFilter struct {
	hosts map[string]struct{}
	mu    sync.RWMutex
}

func New() *URLFilter {
	return &URLFilter{
		hosts: make(map[string]struct{}),
	}
}

func (f *URLFilter) AddHost(host string) {
	f.mu.Lock()
	f.hosts[host] = struct{}{}
	f.mu.Unlock()
}

func (f *URLFilter) Match(u *url.URL) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	_, ok := f.hosts[u.Host]
	return ok
}
