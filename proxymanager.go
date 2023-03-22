package main

import (
	"bufio"
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
)

// var (
// source = rand.NewSource(time.Now().UnixNano())
// randm  = rand.New(source)
// )

type ProxyManager struct {
	mu             sync.Mutex
	index          int
	proxies        []*Proxy
	leases         map[string]string
	leasesByTaskId map[string]string
	Context        context.Context
	WaitGroup      sync.WaitGroup
}

type Proxy struct {
	hash     string
	host     string
	port     string
	username string
	password string
}

func NewProxyManager() *ProxyManager {
	pm := new(ProxyManager)
	pm.Context = context.Background()
	pm.proxies = []*Proxy{}
	pm.leases = make(map[string]string)
	pm.leasesByTaskId = make(map[string]string)
	pm.index = 0
	return pm
}

func (pm *ProxyManager) Read() error {
	proxyFile, err := os.Open("proxies.txt")

	if err != nil {
		return err
	}

	proxyFileScanner := bufio.NewScanner(proxyFile)
	proxyFileScanner.Split(bufio.ScanLines)

	for proxyFileScanner.Scan() {
		line := proxyFileScanner.Text()
		pm.AddProxy(line)
	}

	return nil
}

func (pm *ProxyManager) AddProxy(proxy string) *Proxy {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	parts := strings.Split(proxy, ":")
	p := &Proxy{host: parts[0], port: parts[1]}

	if len(parts) > 2 {
		p.username = parts[2]
		p.password = parts[3]
	}

	p.hash = fmt.Sprintf("%x", md5.Sum([]byte(proxy)))

	pm.proxies = append(pm.proxies, p)

	return p
}

func (pm *ProxyManager) AddProxies(proxies ...string) {
	for _, proxy := range proxies {
		pm.AddProxy(proxy)
	}
}

func (pm *ProxyManager) unlease(taskId string) {
	pHash := pm.leasesByTaskId[taskId]

	delete(pm.leases, pHash)
	delete(pm.leasesByTaskId, taskId)
}

func (pm *ProxyManager) Lease(taskId string) (*Proxy, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.unlease(taskId)

	leased := false
	attempts := 0

	var proxy *Proxy
	var err error

	for !leased {

		i := pm.index
		attempts = attempts + 1
		pm.index = i + 1

		if pm.index == len(pm.proxies) {
			pm.index = 0
		}

		p := pm.proxies[i]

		if _, ok := pm.leases[p.hash]; !ok {
			pm.leasesByTaskId[taskId] = p.hash
			pm.leases[p.hash] = taskId

			proxy = p
			leased = true
		}

		if attempts == 5 {
			err = errors.New("unable to find an unleased proxy")
			break
		}

	}

	return proxy, err

}
