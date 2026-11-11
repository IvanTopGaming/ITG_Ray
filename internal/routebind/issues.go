package routebind

import "sync"

type Issues struct {
	mu sync.Mutex
	m  map[string]Binding
}

func (i *Issues) Set(m map[string]Binding) {
	if i == nil {
		return
	}
	cp := make(map[string]Binding, len(m))
	for k, v := range m {
		cp[k] = v
	}
	i.mu.Lock()
	i.m = cp
	i.mu.Unlock()
}

func (i *Issues) Get() map[string]Binding {
	out := map[string]Binding{}
	if i == nil {
		return out
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	for k, v := range i.m {
		out[k] = v
	}
	return out
}
