package mapper

import (
	"fmt"
	"io"
	"sync"
)

type Mapper struct {
	data map[string][]string
	mu   sync.RWMutex
}

func New() *Mapper {
	return &Mapper{
		data: make(map[string][]string),
	}
}

func (m *Mapper) Add(key string, items ...string) {
	m.mu.Lock()
	m.data[key] = append(m.data[key], items...)
	m.mu.Unlock()
}

func (m *Mapper) List(w io.Writer) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for k, vals := range m.data {
		fmt.Fprintf(w, "Key: %v\n", k)
		for _, v := range vals {
			fmt.Fprintf(w, "\t - %v\n", v)
		}
	}
}
