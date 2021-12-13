package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

var simulatedInstances = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "simulated_instances",
	Help: "Number of simulated instances",
})

func main() {

	log.Info("Starting server-load-simulator")

	config := Config{}
	config.RegisterFlags(flag.CommandLine)
	flag.Parse()

	log.Info("Config:", "config", config)

	// create a ticker that ticks every second
	ticker := time.NewTicker(10 * time.Second)

	startTime := time.Now()

	// setup signal handlers
	shutdown := make(chan os.Signal, 2)
	defer close(shutdown)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(fmt.Sprintf(":%d", config.Port), nil)

mainloop:
	for {
		select {
		case <-shutdown:
			log.Info("Shutting down")
			break mainloop
		case <-ticker.C:
			elapsedTime := time.Since(startTime).Seconds()
			frequencySeconds := config.Frequency.Seconds()
			// calculate the simulated load
			simulatedLoad := float64(config.MaxValue/2) * (1 + math.Sin(2*math.Pi*(1/frequencySeconds)*elapsedTime))

			// creates a random jitter value
			jitter := rand.Float64() * float64(config.Jitter)
			flipJitter := rand.Intn(1000)
			if flipJitter < 500 {
				jitter = jitter * -1
			}
			simulatedLoad = simulatedLoad + (jitter * simulatedLoad)

			// set the simulated load
			simulatedInstances.Set(math.Ceil(simulatedLoad))
			log.Info("Simulated instances:", "value", math.Ceil(simulatedLoad))
		}
	}
}
