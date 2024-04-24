package main

import (
	"GTMServer/etcd"
	"GTMServer/quicClient"
	"GTMServer/tracer"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"net/http"
	"time"
)

var (
	q *quicClient.QClient
)

const chBuffSize int = 10

var flushTime time.Duration = 5 * time.Second

const serviceName = "GTMServerTom"

func main() {
	// We start a server echoing data on the first stream the client opens,
	// then connect with a client, send the message, and wait for its receipt.
	//go func() { log.Fatal(quicClient.EchoServer()) }()
	//err := quicClient.ClientMain()
	//if err != nil {
	//	panic(err)
	//}
	eConf := etcd.EtcdConfig{
		Host:    []string{"127.0.0.1:2379"},
		TimeOut: 5 * time.Second,
	}
	etcdClient := etcd.InitialEtcd(eConf)
	q = quicClient.Initialize("127.0.0.1:2234")
	tracer.InitTracer(&tracer.ConfigTracer{ChBuffSize: chBuffSize, FlushTime: flushTime, ServiceName: serviceName}, q, etcdClient)
	r := chi.NewRouter()

	// A good base middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(tracer.TracerTopology)
	// Set a timeout value on the request context (ctx), that will signal
	// through ctx.Done() that the request has timed out and further
	// processing should be stopped.
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hi"))
	})

	http.ListenAndServe("127.0.0.1:3333", r)
}
