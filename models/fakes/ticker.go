package fakes

import "time"

type Ticker struct {
	C chan time.Time
}

func NewTicker() *Ticker {
	return &Ticker{C: make(chan time.Time, 1)}
}
