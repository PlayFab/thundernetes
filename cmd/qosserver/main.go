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
	Name:      "qos_successful_server_requests",
	Help:      "Number of requests made to the QoS server",
}, []string{"remote_ip"})

func main() {
	go metricsServer(3075)
	qosServer(3075)
}

// metricsServer starts an http server, on a given port,
// with a prometheus metrics endpoint
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

// qosServer starts a Quality of Service UDP server, on a given port,
// that expects requests starting with two 0xFF bytes, and responds
// the same value but with those first bytes flipped to 0x00
func qosServer(port int) {
	log.Infof("Starting UDP server on port :%d", port)
	conn, err := createServer(port)
	if err != nil {
		log.Fatal(err)
	}
	log.Info("UDP server is listening")
	for {
		remote, err := serverLoop(conn)
		if err != nil {
			log.Error(err)
		} else {
			QoSServerRequests.WithLabelValues(remote.IP.String()).Inc()
		}
	}
}

// createServer starts and returns a UDP socket listening to a given port
func createServer(port int) (*net.UDPConn, error) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// serverLoop contains the logic of a single loop of the QoS server,
// it reads from the socket, validates the request, and sends a
// response if needed
func serverLoop(conn *net.UDPConn) (*net.UDPAddr, error) {
	buffer := make([]byte, 32)
	count, remote, err := conn.ReadFromUDP(buffer)
	if err != nil {
		return nil, err
	}
	log.Infof("Read %d bytes, from remote address %v, with value %q", count, remote, buffer[:count])
	response := getResponse(buffer, count)
	if response != nil {
		count, err := conn.WriteTo(buffer[:count], remote)
		if err != nil {
			return nil, err
		}
		log.Infof("Valid input, sent %d bytes response to remote address %v", count, remote)
	}
	return remote, nil
}

// getResponse validates the bytes received, expecting the first two
// values to be 0xFF, and if thats the case returns the same input
// with those values flipped to 0x00, if not return nil
func getResponse(buffer []byte, count int) ([]byte) {
	if count > 2 && buffer[0] == 0xFF && buffer[1] == 0xFF {
		buffer[0] = 0x00
		buffer[1] = 0x00
		return buffer
	}
	return nil
}
