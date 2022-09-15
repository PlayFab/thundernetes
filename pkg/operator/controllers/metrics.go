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
	GameServersCreatedDuration = registry.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "thundernetes",
			Name:      "gameservers_create_duration",
			Help:      "Average time it took to create a the newest set of GameServers",
		},
		[]string{"BuildName"},
	)
	GameServersStandByReconcileDuration = registry.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "thundernetes",
			Name:      "gameservers_reconcile_standby_duration",
			Help:      "Time it took to begin initialization for all new GameServers",
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
	GameServersEndedDuration = registry.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "thundernetes",
			Name:      "gameservers_end_duration",
			Help:      "Time it took to delete a set of non-active GameServers",
		},
		[]string{"BuildName"},
	)
	GameServersCleanUpDuration = registry.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "thundernetes",
			Name:      "gameservers_clean_up_duration",
			Help:      "Average time it took to clean up all completed or unhealthy GameServers",
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
