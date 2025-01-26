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
	Alive        bool
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
	duration := time.Since(start)
	defer conn.Close()
	return duration, true
}

func (s *Server) UpdateStatus() {
	res, isAlive := CheckResponseTime(s.Url)
	s.ResponseTime = res
	s.Alive = isAlive
}

func (s *Server) IsAlive() bool {
	alive := s.Alive

	return alive
}

func (s *Server) Debug() string {
	return fmt.Sprintf("Peer: %s, Alive: %t, ResponseTime: %f", s.Url, s.Alive, s.ResponseTime.Seconds())
}
