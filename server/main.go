package main

import (
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"

	redis "gopkg.in/redis.v5"

	"github.com/bign8/cdn/util/health"
	"github.com/bign8/cdn/util/stats"
)

const cdnHeader = "x-bign8-cdn"

var (
	target = flag.String("target", os.Getenv("TARGET"), "target hostname")
	port   = flag.Int("port", 8081, "What port to run server on")
	cap    = flag.Int("cap", 20, "How many requests to store in cache")
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	health.Check()
	uri, err := url.Parse(*target)
	check(err)

	host, err := os.Hostname()
	check(err)

	registry := stats.New("server", host, *port)

	red := redis.NewClient(&redis.Options{Addr: "redis:6379"})
	check(red.Ping().Err())
	red.SAdd("cdn-servers", host)

	cdnHandler := &cdn{
		me:    host,
		rp:    httputil.NewSingleHostReverseProxy(uri),
		cap:   *cap,
		red:   red,
		cache: make(map[string]response),

		// stats objects
		cacheSize: registry.Gauge("cacheSize"),
		requests:  registry.Timer("requests"),
		s2scalls:  registry.Counter("s2s_calls"),
		nHit:      registry.Counter("neighbor_hit"),
		nMiss:     registry.Counter("neighbor_miss"),
	}
	cdnHandler.rp.Transport = cdnHandler
	http.Handle("/", cdnHandler)

	// Actually start the server
	log.Printf("ReverseProxy for %q serving on :%d\n", *target, *port)
	go cdnHandler.monitorNeighbors()
	check(http.ListenAndServe(":"+strconv.Itoa(*port), nil))
}
