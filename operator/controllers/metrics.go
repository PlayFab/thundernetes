package controllers

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	GameServersCreatedCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gameservers_created_total",
			Help: "Number of GameServers created",
		},
		[]string{"BuildName"},
	)
	GameServersSessionEndedCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gameservers_sessionended_total",
			Help: "Number of GameServer sessions ended",
		},
		[]string{"BuildName"},
	)
	GameServersCrashedCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gameservers_crashed_total",
			Help: "Number of GameServers sessions crashed",
		},
		[]string{"BuildName"},
	)
	GameServersDeletedCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gameservers_deleted_total",
			Help: "Number of GameServers deleted",
		},
		[]string{"BuildName"},
	)
	InitializingGameServersGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gameservers_initializing_total",
			Help: "Number of initializing GameServers",
		},
		[]string{"BuildName"},
	)
	StandingByGameServersGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gameservers_standingby_total",
			Help: "Number of standing by GameServers",
		},
		[]string{"BuildName"},
	)
	ActiveGameServersGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gameservers_active_total",
			Help: "Number of active GameServers",
		},
		[]string{"BuildName"},
	)
	AllocationsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "allocations_total",
			Help: "Number of GameServers allocations",
		},
		[]string{"BuildName"},
	)
)

func addMetricsToRegistry() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(GameServersCreatedCounter,
		GameServersCrashedCounter,
		GameServersDeletedCounter,
		GameServersSessionEndedCounter,
		InitializingGameServersGauge,
		StandingByGameServersGauge,
		ActiveGameServersGauge,
		AllocationsCounter)
}
