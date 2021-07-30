package main

import "time"

type DummyWatchdog struct {
	Period int
}

func (d *DummyWatchdog) Initialize() error {
	return nil
}

func (d *DummyWatchdog) PeriodD() time.Duration {
	return time.Duration(d.Period) * time.Second
}

func (d *DummyWatchdog) Ping() error {
	log.Debug("Ping!")
	return nil
}
