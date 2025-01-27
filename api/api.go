package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/newtoallofthis123/loader/server"
)

type ServerPool struct {
	peers       []*server.Server
	curr        uint64
	lookup      map[*url.URL]*server.Server
	reqs        map[string]*url.URL
	logger      *slog.Logger
	rw          sync.RWMutex
	lastChecked time.Time
	wg          sync.WaitGroup
}

type Request struct {
	remoteUrl *url.URL
	tries     uint8
	assigned  *url.URL
}

func NewServerPool(logger *slog.Logger) *ServerPool {
	return &ServerPool{
		peers:       make([]*server.Server, 0),
		curr:        0,
		lookup:      make(map[*url.URL]*server.Server, 0),
		reqs:        make(map[string]*url.URL, 0),
		logger:      logger,
		rw:          sync.RWMutex{},
		wg:          sync.WaitGroup{},
		lastChecked: time.Now(),
	}
}

func NewServerPoolWithPeers(peers []*server.Server, logger *slog.Logger) *ServerPool {
	server := &ServerPool{
		peers:       peers,
		curr:        ranIndex(len(peers)),
		lookup:      makeLookUp(peers),
		reqs:        make(map[string]*url.URL, 0),
		logger:      logger,
		rw:          sync.RWMutex{},
		wg:          sync.WaitGroup{},
		lastChecked: time.Now(),
	}

	server.reorder()
	return server
}

func (s *ServerPool) DebugPrint() {
	for _, peer := range s.peers {
		s.logger.Info(peer.Debug())
	}
}

func (s *ServerPool) reorder() {
	sort.Slice(s.peers, func(i, j int) bool {
		return s.peers[i].ResponseTime < s.peers[j].ResponseTime
	})
}

func (s *ServerPool) NextIndex() int {
	s.rw.RLock()
	defer s.rw.RUnlock()
	return int(atomic.AddUint64(&s.curr, uint64(1)) % uint64(len(s.peers)))
}

func (s *ServerPool) AddServer(server *server.Server) {
	s.rw.Lock()
	s.peers = append(s.peers, server)
	s.lookup[server.Url] = server
	s.logger.Info(fmt.Sprintf("Added server %s and reordered pool", server.Url))
	s.reorder()
	s.rw.Unlock()
}

func (sp *ServerPool) StartHealthChecks() {
	ticker := time.NewTicker(1 * time.Minute)
	sp.wg.Add(1)

	go func() {
		for range ticker.C {
			sp.CheckHealth()
		}
	}()

	sp.wg.Wait()
}
func (s *ServerPool) CheckHealth() {
	s.logger.Info(fmt.Sprintf("Checking pool health"))
	var wg sync.WaitGroup
	for _, peer := range s.peers {
		wg.Add(1)

		go func(p *server.Server) {
			defer wg.Done()
			p.UpdateStatus()
			if !p.IsAlive() {
				s.logger.Info(fmt.Sprintf("%s is down", p.Url))
			}
		}(peer)
	}

	wg.Wait()
	s.logger.Info("Pool check completed")
}

func (s *ServerPool) shouldReOrder() bool {
	s.rw.RLock()
	should := time.Since(s.lastChecked) >= time.Minute
	s.rw.RUnlock()
	return should
}

func (s *ServerPool) NextPeer() *server.Server {
	if s.shouldReOrder() {
		s.rw.Lock()
		s.reorder()
		s.logger.Info(fmt.Sprintf("Reordered"))
		s.lastChecked = time.Now()
		s.rw.Unlock()
	}
	next := s.NextIndex()
	circle := len(s.peers) + next
	for i := next; i < circle; i++ {
		index := i % len(s.peers)
		if s.peers[index].IsAlive() {
			atomic.StoreUint64(&s.curr, uint64(index))
			s.logger.Info(fmt.Sprintf("Served peer: %s", s.peers[index].Url))
			return s.peers[index]
		}
	}

	return nil
}

func (s *ServerPool) handleErr(w http.ResponseWriter, r *http.Request, e error) {
	s.logger.Error(fmt.Sprintf("Request failed with err %s", e.Error()))
	s.rw.Lock()
	server := s.lookup[s.reqs[r.RemoteAddr]]
	server.UpdateStatus()
	if !server.IsAlive() {
		s.logger.Error(fmt.Sprintf("Server %s is not alive", server.Url))
		s.CheckHealth()
	}
	delete(s.reqs, r.RemoteAddr)
	s.rw.Unlock()

	s.Serve(w, r)
}

func (s *ServerPool) handleRes(res *http.Response) error {
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("Server res failed with err and status code %d", res.StatusCode)
	}
	s.rw.Lock()
	delete(s.reqs, res.Request.RemoteAddr)
	s.rw.Unlock()
	return nil
}

func (s *ServerPool) Serve(w http.ResponseWriter, r *http.Request) {
	peer := s.NextPeer()
	if peer != nil {
		s.rw.Lock()
		s.reqs[r.RemoteAddr] = peer.Url
		s.rw.Unlock()
		peer.Proxy.ErrorHandler = s.handleErr
		peer.Proxy.ModifyResponse = s.handleRes
		peer.Proxy.ServeHTTP(w, r)
		return
	}
	http.Error(w, "Service not available", http.StatusServiceUnavailable)
}
