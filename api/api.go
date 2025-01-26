package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/newtoallofthis123/loader/server"
)

type ServerPool struct {
	peers  []*server.Server
	curr   uint64
	lookup map[*url.URL]*server.Server
	reqs   map[string]*url.URL
	logger *slog.Logger
	rw     sync.RWMutex
}

type Request struct {
	remoteUrl *url.URL
	tries     uint8
	assigned  *url.URL
}

func NewServerPool(logger *slog.Logger) *ServerPool {
	return &ServerPool{
		peers:  make([]*server.Server, 0),
		curr:   0,
		lookup: make(map[*url.URL]*server.Server, 0),
		reqs:   make(map[string]*url.URL, 0),
		logger: logger,
		rw:     sync.RWMutex{},
	}
}

func NewServerPoolWithPeers(peers []*server.Server, logger *slog.Logger) *ServerPool {
	return &ServerPool{
		peers:  peers,
		curr:   ranIndex(len(peers)),
		lookup: makeLookUp(peers),
		reqs:   make(map[string]*url.URL, 0),
		logger: logger,
		rw:     sync.RWMutex{},
	}

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

func (s *ServerPool) CheckHealth() {
	s.logger.Info(fmt.Sprintf("Checking pool health"))
	s.rw.Lock()
	for _, peer := range s.peers {
		peer.UpdateStatus()
	}
	s.rw.Unlock()
}

func (s *ServerPool) NextPeer() *server.Server {
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
	}
	s.rw.Unlock()

	s.Serve(w, r)
}

func (s *ServerPool) Serve(w http.ResponseWriter, r *http.Request) {
	peer := s.NextPeer()
	if peer != nil {
		s.rw.Lock()
		s.reqs[r.RemoteAddr] = peer.Url
		s.rw.Unlock()
		peer.Proxy.ErrorHandler = s.handleErr
		peer.Proxy.ServeHTTP(w, r)
		return
	}
	http.Error(w, "Service not available", http.StatusServiceUnavailable)
}
