package main

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

var QoSServerRequests = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "thundernetes",
	Name:      "qos_server_requests",
	Help:      "Number of requests made to the QoS server",
}, []string{"remote_ip"})

func main() {
	go metricsServer(3075)
	qosServer(3075)
}

func metricsServer(port int) {
	http.Handle("/metrics", promhttp.Handler())
	srv := &http.Server{
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		MaxHeaderBytes: http.DefaultMaxHeaderBytes,
		Addr:           fmt.Sprintf(":%d", port),
	}
	log.Infof("Starting metrics server on port %s", srv.Addr)
	err := srv.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}

func qosServer(port int) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("Starting UDP server on port :%d", addr.Port)
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatal(err)
	}
	buffer := make([]byte, 32)
	log.Info("UDP server is listening")
	for {
		count, remote, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Error(err)
			continue
		}
		log.Infof("Read %d bytes, from remote address %v, with value %q", count, remote, buffer[:count])
		if count > 2 && buffer[0] == 0xFF && buffer[1] == 0xFF {
			buffer[0] = 0x00
			buffer[1] = 0x00
			count, err := conn.WriteTo(buffer[:count], remote)
			if err != nil {
				log.Error(err)
				continue
			}
			log.Infof("Valid input, sent %d bytes response to remote address %v", count, remote)
			QoSServerRequests.WithLabelValues(remote.IP.String()).Inc()
		}
	}
}
