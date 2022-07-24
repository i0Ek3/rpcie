package xclient

import (
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"
)

type LBStrategy int

const (
	RandomSelect LBStrategy = iota
	RoundRobin
)

type Discovery interface {
	Refresh() error
	Update(servers []string) error
	Get(mode LBStrategy) (string, error)
	GetAll() ([]string, error)
}

type MultiServersDiscovery struct {
	// random numbers instance
	r       *rand.Rand
	mu      sync.Mutex
	servers []string
	// index records where the Round Robin algorithm has polled
	index int
}

func NewMultiServerDiscovery(servers []string) *MultiServersDiscovery {
	d := &MultiServersDiscovery{
		servers: servers,
		r:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	d.index = d.r.Intn(math.MaxInt32 - 1)
	return d
}

var _ Discovery = (*MultiServersDiscovery)(nil)

func (d *MultiServersDiscovery) Refresh() error {
	return nil
}

func (d *MultiServersDiscovery) Update(servers []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.servers = servers
	return nil
}

func (d *MultiServersDiscovery) Get(mode LBStrategy) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := len(d.servers)
	if n == 0 {
		return "", errors.New("rpc discovery: no available servers")
	}
	switch mode {
	case RandomSelect:
		return d.servers[d.r.Intn(n)], nil
	case RoundRobin:
		s := d.servers[d.index%n]
		d.index = (d.index + 1) % n
		return s, nil
	default:
		return "", errors.New("rpc discovery: not supported strategy")
	}
}

func (d *MultiServersDiscovery) GetAll() ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	servers := make([]string, len(d.servers), len(d.servers))
	copy(servers, d.servers)
	return servers, nil
}
