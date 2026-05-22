package hosting

import (
	"context"
)

type Host interface {
	Name() string
	DisplayName() string
	Upload(ctx context.Context, imagePaths []string, onLog func(string)) ([]string, error)
}

type HostManager struct {
	hosts    map[string]Host
	defaultHost string
}

func NewManager() *HostManager {
	return &HostManager{
		hosts: make(map[string]Host),
	}
}

func (m *HostManager) Register(host Host) {
	m.hosts[host.Name()] = host
}

func (m *HostManager) SetDefault(name string) {
	m.defaultHost = name
}

func (m *HostManager) Get(name string) Host {
	if name == "" {
		name = m.defaultHost
	}
	return m.hosts[name]
}

func (m *HostManager) List() []Host {
	list := make([]Host, 0, len(m.hosts))
	for _, host := range m.hosts {
		list = append(list, host)
	}
	return list
}

func (m *HostManager) Names() []string {
	names := make([]string, 0, len(m.hosts))
	for name := range m.hosts {
		names = append(names, name)
	}
	return names
}