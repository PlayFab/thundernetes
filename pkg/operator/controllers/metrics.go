package controllers

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	ActiveServerStatus       = "active"
	StandingByServerStatus   = "standingby"
	InitializingServerStatus = "initializing"
	PendingServerStatus      = "pending"
)

var (
	registry = promauto.With(metrics.Registry)

	GameServersCreatedCounter = registry.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "thundernetes",
			Name:      "gameservers_created_total",
			Help:      "Number of GameServers created",
		},
		[]string{"BuildName"},
	)
	GameServersSessionEndedCounter = registry.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "thundernetes",
			Name:      "gameservers_sessionended_total",
			Help:      "Number of GameServer sessions ended",
		},
		[]string{"BuildName"},
	)
	GameServersCrashedCounter = registry.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "thundernetes",
			Name:      "gameservers_crashed_total",
			Help:      "Number of GameServers sessions crashed",
		},
		[]string{"BuildName"},
	)
	GameServersDeletedCounter = registry.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "thundernetes",
			Name:      "gameservers_deleted_total",
			Help:      "Number of GameServers deleted",
		},
		[]string{"BuildName"},
	)
	CurrentGameServerGauge = registry.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "thundernetes",
			Name:      "gameservers_current_state",
			Help:      "Gameserver gauges by state",
		},
		[]string{"BuildName", "state"},
	)
	AllocationsCounter = registry.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "thundernetes",
			Name:      "allocations_total",
			Help:      "Number of GameServers allocations",
		},
		[]string{"BuildName"},
	)
)
