package main

import (
	"errors"
	"strings"
	"time"

	"github.com/d2r2/go-i2c"
	"github.com/d2r2/go-logger"
)

const i2c_address = 0x62

const (
	reg_status = 0
	reg_watch  = 1
	reg_wake   = 2
	reg_time   = 4
)

const (
	status_button_boot    = 1 << 5
	status_timer_boot     = 1 << 6
	status_button_pressed = 1 << 7
)

type Piwatcher struct {
	i2c   *i2c.I2C
	Wake  int
	Watch byte
}

type PiwatcherStatus byte

func (s PiwatcherStatus) String() string {
	ps := []string{"OK"}
	if s&status_button_boot != 0 {
		ps = append(ps, "BUTTON_BOOT")
	}
	if s&status_timer_boot != 0 {
		ps = append(ps, "TIMER_BOOT")
	}
	if s&status_button_pressed != 0 {
		ps = append(ps, "BUTTON_PRESSED")
	}
	return strings.Join(ps, " ")
}

func (p *Piwatcher) Initialize() error {
	logger.ChangePackageLogLevel("i2c", logger.InfoLevel)
	i2c, err := i2c.NewI2C(i2c_address, 1)
	if err != nil {
		return err
	}
	p.i2c = i2c
	log.Infof("Setting wake to %d", p.Wake)
	err = p.SetWake(p.Wake)
	if err != nil {
		return err
	}

	log.Infof("Setting watch to %d", p.Watch)
	err = p.SetWatch(p.Watch)
	if err != nil {
		return err
	}
	return nil
}

func (p *Piwatcher) PeriodD() time.Duration {
	return time.Duration(p.Watch) * time.Second
}

func (p *Piwatcher) Status() (PiwatcherStatus, error) {
	status, err := p.i2c.ReadRegU8(reg_status)
	return PiwatcherStatus(status), err
}

func (p *Piwatcher) Ping() error {
	_, err := p.Status()
	return err
}

func (p *Piwatcher) SetWatch(watch byte) error {
	return p.i2c.WriteRegU8(reg_watch, watch)
}

func (p *Piwatcher) GetWatch() (byte, error) {
	return p.i2c.ReadRegU8(reg_watch)
}

func (p *Piwatcher) SetWake(wake int) error {
	if wake&1 != 0 {
		return errors.New("wake must be a multiple to 2")
	}
	if wake > 131070 {
		return errors.New("wake must be not be greater than 131070")
	}
	wake = wake >> 1
	buf := []byte{reg_wake, byte(wake & 0xff), byte(wake >> 8)}
	_, err := p.i2c.WriteBytes(buf)
	return err
}

func (p *Piwatcher) GetWake() (int, error) {
	wake, err := p.i2c.ReadRegU16LE(reg_wake)
	return int(wake) * 2, err
}

func (p *Piwatcher) Reset() error {
	return p.i2c.WriteRegU8(reg_status, 0xff)
}

func (p *Piwatcher) Close() error {
	return p.i2c.Close()
}
