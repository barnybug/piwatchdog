package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

type TemperatureWatcher struct {
	CommonWatcher
	Path        string
	FailureOver int
}

func (t *TemperatureWatcher) Initialize() error {
	if _, err := os.Stat(t.Path); os.IsNotExist(err) {
		return fmt.Errorf("Path does not exist: %s", t.Path)
	}
	return nil
}

func (t *TemperatureWatcher) Name() string {
	return "temperature"
}

func (t *TemperatureWatcher) Check(ctx context.Context) error {
	b, err := ioutil.ReadFile(t.Path)
	if err != nil {
		return err
	}
	trimmed := strings.TrimSpace(string(b))
	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return err
	}
	log.Debugf("Read temperature: %d", value)
	if value > t.FailureOver {
		return fmt.Errorf("value %.1f > threshold %.1f", float32(value)/1000, float32(t.FailureOver)/1000)
	}
	return nil
}
