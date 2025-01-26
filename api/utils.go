package api

import (
	"math/rand"
	"net/url"

	"github.com/newtoallofthis123/loader/server"
)

func makeLookUp(servers []*server.Server) map[*url.URL]*server.Server {
	m := make(map[*url.URL]*server.Server, 0)
	for _, s := range servers {
		m[s.Url] = s
	}

	return m
}

func ranIndex(length int) uint64 {
	return uint64(rand.Int63n(int64(length)))
}
