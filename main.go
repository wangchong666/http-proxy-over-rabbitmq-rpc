package main

import (
	"flag"
	log "github.com/sirupsen/logrus"
	"hpor/rpc"
	"os"
)

var t = flag.String("t", "RP", "RPC or PROXY  or RP")
var amqp = flag.String("url", "amqp://guest:guest@127.0.0.1:5672/", "amqp address")
var port = flag.Int("p", 8000, "HTTP proxy port")
var count = flag.Int("c", 1, "The number of consumers，work with -t RPC")
var debug = flag.Bool("d", false, "log debug info")
var queueName = flag.String("q", "rpc_queue", "The queue name for rpc")

func main() {

	flag.Parse()

	log.SetOutput(os.Stdout)
	if *debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	conf := &rpc.Config{
		Amqp:      *amqp,
		Port:      *port,
		QueueName: *queueName,
	}

	log.Info("start app as ", *t)

	if *t == "RPC" {
		rpc.Accept(conf, *count)
	} else if *t == "RP" {
		rpc.ReverseProxy(conf)
	} else {
		rpc.Proxy(conf)
	}
}
