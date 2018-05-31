package mapper

import (
	"fmt"
	"io"
	"sync"
)

type Mapper struct {
	data  map[string][]string
	order []string
	mu    sync.RWMutex
}

func New() *Mapper {
	return &Mapper{
		data: make(map[string][]string),
	}
}

func (m *Mapper) Add(key string, items ...string) {
	m.mu.Lock()

	d, ok := m.data[key]
	m.data[key] = append(d, items...)
	if !ok {
		m.order = append(m.order, key)
	}

	m.mu.Unlock()
}

func (m *Mapper) List(w io.Writer) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, k := range m.order {
		fmt.Fprintf(w, "Key: %v\n", k)
		for _, v := range m.data[k] {
			fmt.Fprintf(w, "\t - %v\n", v)
		}
	}
}
