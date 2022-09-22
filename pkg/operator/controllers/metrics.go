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
	AllocationsTimeTakenDuration = registry.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "thundernetes",
			Name:      "allocations_time_taken_duration",
			Help:      "Average time it took to allocate newest set of GameServers",
		},
		[]string{"BuildName"},
	)
	AllocationsRetriesCounter = registry.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "thundernetes",
			Name:      "allocations_retried",
			Help:      "The number of times allocation had to be retried",
		},
		[]string{"BuildName"},
	)
	Allocations429ErrorsCounter = registry.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "thundernetes",
			Name:      "allocations_429",
			Help:      "The number of 429 (too many requests) errors during allocation",
		},
		[]string{"BuildName"},
	)
	Allocations404ErrorsCounter = registry.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "thundernetes",
			Name:      "allocations_404",
			Help:      "The number of 404 (not found) errors during allocation",
		},
		[]string{"BuildName"},
	)
	Allocations500ErrorsCounter = registry.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "thundernetes",
			Name:      "allocations_500",
			Help:      "The number of 500 (internal) errors during allocation",
		},
		[]string{"BuildName"},
	)
)
