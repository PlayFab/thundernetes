package main

import (
	"flag"
	"time"
)

type Config struct {
	MaxValue  int           `json:"maxValue"`
	Frequency time.Duration `json:"frequency"`
	Jitter    float64       `json:"jitter"`
	Port      int           `json:"port"`
}

func (c *Config) RegisterFlags(f *flag.FlagSet) {
	f.IntVar(&c.MaxValue, "maxValue", 100, "Maximum value of the load")
	f.DurationVar(&c.Frequency, "frequency", time.Hour, "ForecastFrequency of the load")
	f.Float64Var(&c.Jitter, "jitter", 0.15, "Jitter of the load")
	f.IntVar(&c.Port, "port", 8080, "Port to listen on")
}
