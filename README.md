# piwatchdog

piwatchdog is a small, configurable watchdog/monitor written in Go. It runs a set of "watchers" (temperature, HTTP URL, etc.) and pings a configured watchdog (hardware via I2C or a dummy implementation) while the system is healthy. If watchers fail or become past-due, the program stops pinging the watchdog so a hardware watchdog can reset the device.

This project is intended for Raspberry Pi and [Piwatcher](https://www.omzlo.com/articles/the-piwatcher) hardware watchdog device.

Features
- Pluggable watchdog implementations:
  - `Piwatcher` — interacts with a Pi watchdog device over I2C.
  - `DummyWatchdog` — no-op watchdog for development / testing.
- Built-in watchers:
  - `TemperatureWatcher` — reads sysfs temperature input and fails above a threshold.
  - `UrlWatcher` — fetches a URL and optionally executes the fetched script with `bash`.
- Configurable with TOML.
- Simple, concurrency-friendly design: each watcher runs in its own goroutine and reports liveness to the main loop.

Repository layout
- `main.go` — application entrypoint, CLI, configuration loading, watcher/watchdog orchestration.
- `piwatcher.go` — I2C-based hardware watchdog implementation.
- `dummy.go` — dummy watchdog implementation for testing.
- `temperature.go` — temperature watcher implementation.
- `url.go` — URL watcher implementation (can fetch and optionally execute scripts).
- `config.toml` — example configuration.

Requirements
- Go 1.21 (see `go.mod`)
- For `Piwatcher`: I2C enabled and available to the process (and the `go-i2c` dependency).
- If using `UrlWatcher` with `execute = true`, be aware that the program executes remote code — only use trusted URLs.

Configuration

The app reads a TOML configuration file (default `config.toml`). Example keys:

- `logging.level` — `DEBUG` / `INFO` / `WARN` / `ERROR` / `FATAL` / `NOTICE`
- `watchdog.dummy` or `watchdog.piwatcher` — configure one of the watchdogs
- `watcher.temperature`
  - `period` — poll period in seconds
  - `timeout` — per-check timeout in seconds (optional)
  - `path` — path to sysfs temperature input (e.g. `/sys/.../temp1_input`)
  - `failureOver` — threshold in thousandths of degrees C (e.g. `37000` = 37.0°C)
- `watcher.url`
  - `period`, `timeout` — seconds
  - `url` — URL to fetch
  - `execute` — if true, execute fetched body under `/bin/bash`
  - `xtrace` — if true, pass `-x` to `bash`

See the provided `config.toml` for an example configuration.

How it works (high level)
1. The program loads configuration and sets logging level.
2. It initializes the configured watchdog (hardware or dummy).
3. It initializes all configured watchers and starts each watcher in its own goroutine.
4. Each watcher periodically performs a check:
   - On success it reports "alive" to the main loop.
   - On failure it enters a backoff loop and does not report "alive".
5. The main loop maintains watcher states. When all watchers are healthy, the program pings the watchdog at the watchdog's period. If any watcher fails or becomes overdue, the program stops pinging the watchdog so the hardware watchdog may take action (e.g., reset the system).

Building and running
- Build:
  - `go build` will produce the `piwatchdog` binary in the current directory.
- Run:
  - `./piwatchdog -c config.toml`
- CLI:
  - `-c`, `--config` — path to TOML config file (default `config.toml`)

Safety and security notes
- The `UrlWatcher` can execute arbitrary shell code when `execute = true`. Only enable this with trusted, auditable sources, and consider network and privilege isolation.
- If you use the hardware `Piwatcher`, ensure the process has access to the I2C bus and necessary permissions. Running as root or via a systemd service with appropriate capabilities is common.
- Consider running piwatchdog inside a minimal container or under a dedicated user with limited permissions for additional safety.

Extending piwatchdog
- Add a new watcher:
  - Implement the `Watcher` interface (methods: `Initialize() error`, `Name() string`, `Check(ctx context.Context) error`, `Config() *CommonWatcher`).
  - Register the watcher in `configureWatchers` in `main.go`.
- Add a new watchdog:
  - Implement the `Watchdog` interface (methods: `Initialize() error`, `PeriodD() time.Duration`, `Ping() error`).
  - Wire it in `configureWatchdog` in `main.go`.

Systemd unit (suggested)
- To run piwatchdog as a service, create a systemd unit that runs the binary and supplies the config path and necessary privileges. Make sure to control access to I2C and any scripts that may be executed.

Troubleshooting
- Check logs — logging is controlled via `logging.level` in the config.
- If using `Piwatcher`, ensure the I2C address and bus are correct and that the I2C kernel drivers are enabled.
- If a watcher is not behaving as expected, increase logging to `DEBUG` and verify the configured `period` and `timeout` values.

Contributing
- Contributions are welcome. Please open issues and pull requests.
- If you add features or watchers, include unit tests where applicable and keep the design consistent with the current watcher/watchdog interfaces.
