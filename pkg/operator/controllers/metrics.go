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
	GameServerBatchCreationDuration = registry.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "thundernetes",
			Name:      "gameservers_batch_creation_duration",
			Help:      "Time it took the controller to create a requested number of GameServer objects",
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
			Help:      "Number of GameServers crashed",
		},
		[]string{"BuildName"},
	)
	GameServersUnhealthyCounter = registry.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "thundernetes",
			Name:      "gameservers_unhealthy_total",
			Help:      "Number of GameServers marked as Unhealthy",
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
			Name:      "gameservers_current_state_per_build",
			Help:      "Gameserver gauges by state per build",
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
