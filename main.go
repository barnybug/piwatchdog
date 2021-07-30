package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/d2r2/go-logger"
	"github.com/pelletier/go-toml/v2"
	"github.com/urfave/cli/v2"
)

type WatchdogConfig struct {
	Piwatcher *Piwatcher
	Dummy     *DummyWatchdog
}

type CommonWatcher struct {
	Period  int
	Timeout int
}

func (c *CommonWatcher) Config() *CommonWatcher {
	return c
}

func (c *CommonWatcher) PeriodD() time.Duration {
	period := time.Second * time.Duration(c.Period)
	if period == 0 {
		period = time.Second * 30
	}
	return period
}

func (c *CommonWatcher) TimeoutD() time.Duration {
	timeout := time.Second * time.Duration(c.Timeout)
	if timeout == 0 {
		timeout = time.Second * 30
	}
	return timeout
}

type Watchers struct {
	Temperature *TemperatureWatcher
	Url         *UrlWatcher
}

type LoggingConfig struct {
	Level string
}

type Config struct {
	Logging  LoggingConfig
	Watchdog WatchdogConfig
	Watcher  Watchers
}

type Watchdog interface {
	Initialize() error
	PeriodD() time.Duration
	Ping() error
}

func loadConfig(configPath string) *Config {
	file, err := os.Open(configPath)
	if err != nil {
		log.Fatal(err)
	}
	var config Config
	dec := toml.NewDecoder(file)
	err = dec.Decode(&config)
	if err != nil {
		log.Fatal(err)
	}
	return &config
}

var log = logger.NewPackageLogger("main",
	logger.DebugLevel,
)

func configureLogging(l LoggingConfig) {
	var level logger.LogLevel
	switch l.Level {
	case "INFO":
		level = logger.InfoLevel
	case "ERROR":
		level = logger.ErrorLevel
	case "WARN", "WARNING":
		level = logger.WarnLevel
	case "NOTICE":
		level = logger.NotifyLevel
	case "DEBUG":
		level = logger.DebugLevel
	case "FATAL":
		level = logger.FatalLevel
	default:
		log.Fatalf("Log level not understood: %s", l.Level)
	}
	logger.ChangePackageLogLevel("main", level)
}

func configureWatchdog(config *Config) Watchdog {
	log.Info("Creating watchdog...")
	var watchdog Watchdog
	if config.Watchdog.Dummy != nil {
		watchdog = config.Watchdog.Dummy
	} else if config.Watchdog.Piwatcher != nil {
		watchdog = config.Watchdog.Piwatcher
	} else {
		log.Fatal("No watchdog configured")
	}
	err := watchdog.Initialize()
	if err != nil {
		log.Fatal(err)
	}
	return watchdog
}

type Watcher interface {
	Initialize() error
	Name() string
	Check(ctx context.Context) error
	Config() *CommonWatcher
}

func configureWatchers(config *Config) []Watcher {
	var ret []Watcher
	// defaults
	if config.Watcher.Temperature != nil {
		ret = append(ret, config.Watcher.Temperature)
	}
	if config.Watcher.Url != nil {
		ret = append(ret, config.Watcher.Url)
	}
	return ret
}

func runWatcher(watcher Watcher, alive chan Watcher) {
	backoff := time.Second
	period := watcher.Config().PeriodD()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), watcher.Config().TimeoutD())
		defer cancel()

		err := watcher.Check(ctx)
		if err == nil {
			alive <- watcher
			log.Infof("%s: good", watcher.Name())
			backoff = time.Second
			time.Sleep(period)
		} else {
			log.Errorf("%s: error: %s", watcher.Name(), err)
			log.Errorf("%s: backoff for %s", watcher.Name(), backoff)
			time.Sleep(backoff)
			backoff = backoff * 2
			if backoff > period {
				backoff = period
			}
		}
	}
}

type WatcherState struct {
	due  time.Time
	good bool
}

func ping(watchdog Watchdog) {
	log.Debug("Pinging watchdog")
	err := watchdog.Ping()
	if err != nil {
		log.Fatalf("Error pinging watchdog: %s", err)
	}
}

func main() {
	app := &cli.App{
		Name: "piwatchdog",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Value:   "config.toml",
				Usage:   "set config file",
			},
		},
		Action: func(c *cli.Context) error {
			run(c.String("config"))
			return nil
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run(configPath string) {
	config := loadConfig(configPath)
	configureLogging(config.Logging)
	log.Notify("Starting up...")
	log.Debugf("config loaded: %+v", *config)

	watchdog := configureWatchdog(config)
	watchers := configureWatchers(config)
	log.Debug("Starting watchers...")
	watcherStates := map[Watcher]*WatcherState{}
	alive := make(chan Watcher, 10)

	for _, watcher := range watchers {
		watcherStates[watcher] = &WatcherState{due: time.Now(), good: false}
		err := watcher.Initialize()
		if err != nil {
			log.Fatalf("Error initializing watchdog %s: %s", watcher, err)
		}
		go runWatcher(watcher, alive)
	}

	failing := len(watchers)
	ticker := time.NewTicker(watchdog.PeriodD() / 2)

	for {
		select {
		case watcher := <-alive:
			// set next due
			state := watcherStates[watcher]
			state.due = time.Now().Add(watcher.Config().PeriodD() * 2) // allow grace
			if !state.good {
				// BAD->GOOD
				log.Notifyf("%s: BAD->GOOD", watcher.Name())
				state.good = true
				failing -= 1
				if failing == 0 {
					// ping immediately as we're living on borrowed time
					log.Notify("ALL: BAD->GOOD")
					ping(watchdog)
				}
			}
		case now := <-ticker.C:
			if failing == 0 {
				// check for any past due
				for watcher, state := range watcherStates {
					if state.good && state.due.Before(now) {
						log.Notifyf("%s: GOOD->BAD", watcher.Name())
						state.good = false
						failing += 1
						if failing == 1 {
							log.Notify("ALL: GOOD->BAD")
						}
					}
				}
				if failing == 0 {
					ping(watchdog)
				}
			}
		}
	}
}
