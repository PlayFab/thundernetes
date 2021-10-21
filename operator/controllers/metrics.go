package controllers

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	ActiveServerStatus       = "active"
	StandingByServerStatus   = "standingby"
	InitializingServerStatus = "initializing"
)

var (
	GameServersCreatedCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gameservers_created_total",
			Help: "Number of GameServers created",
		},
		[]string{"BuildName"},
	)
	GameServersSessionEndedCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gameservers_sessionended_total",
			Help: "Number of GameServer sessions ended",
		},
		[]string{"BuildName"},
	)
	GameServersCrashedCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gameservers_crashed_total",
			Help: "Number of GameServers sessions crashed",
		},
		[]string{"BuildName"},
	)
	GameServersDeletedCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gameservers_deleted_total",
			Help: "Number of GameServers deleted",
		},
		[]string{"BuildName"},
	)
	CurrentGameServerGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gameservers_current",
			Help: "Current number of GameServers",
		},
		[]string{"BuildName", "status"},
	)
	AllocationsCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "allocations_total",
			Help: "Number of GameServers allocations",
		},
		[]string{"BuildName"},
	)
)
