package server

import (
	"fmt"
	"net"
	"net/http/httputil"
	"net/url"
	"time"
)

type Server struct {
	Url          *url.URL
	alive        bool
	Proxy        *httputil.ReverseProxy
	ResponseTime time.Duration
}

func NewServer(serverUrl string) (*Server, error) {
	parsedUrl, err := url.Parse(serverUrl)
	if err != nil {
		return nil, err
	}

	server := Server{
		Url:   parsedUrl,
		Proxy: httputil.NewSingleHostReverseProxy(parsedUrl),
	}
	server.UpdateStatus()

	return &server, nil
}

func CheckResponseTime(url *url.URL) (time.Duration, bool) {
	timeout := 2 * time.Second
	start := time.Now()
	conn, err := net.DialTimeout("tcp", url.Host, timeout)
	if err != nil {
		return 0, false
	}
	defer conn.Close()
	duration := time.Since(start)
	return duration, true
}

func (s *Server) UpdateStatus() {
	s.ResponseTime, s.alive = CheckResponseTime(s.Url)
}

func (s *Server) IsAlive() bool {
	return s.alive
}

func (s *Server) Debug() string {
	return fmt.Sprintf("Peer: %s, Alive: %t, ResponseTime: %f", s.Url, s.alive, s.ResponseTime.Seconds())
}
