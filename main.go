package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/newtoallofthis123/loader/api"
	"github.com/newtoallofthis123/loader/server"
)

func main() {
	var serverList string
	var port int
	flag.StringVar(&serverList, "urls", "", "Comma seperated urls")
	flag.IntVar(&port, "port", 6969, "Port to serve")
	flag.Parse()
	urls := strings.Split(serverList, ",")

	peers := make([]*server.Server, 0)
	for _, url := range urls {
		server, err := server.NewServer(url)
		if err != nil {
			log.Fatalf("Error creating server: %v", err)
		}
		peers = append(peers, server)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	pool := api.NewServerPoolWithPeers(peers, logger)
	go pool.StartHealthChecks()

	server := http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: http.HandlerFunc(pool.Serve),
	}

	logger.Info(fmt.Sprintf("Started Listening on :%d from pool: ", port))
	pool.DebugPrint()

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
