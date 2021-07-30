package main

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"os/exec"
	"syscall"
)

type UrlWatcher struct {
	CommonWatcher
	Url     string
	Execute bool
	Xtrace  bool
}

func (u *UrlWatcher) Initialize() error {
	return nil
}

func (u *UrlWatcher) Name() string {
	return "url"
}

func (u *UrlWatcher) fetchScript(ctx context.Context) ([]byte, error) {
	log.Debug("Fetching script")
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.Url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	log.Debugf("Returned: %s", string(body))
	return body, err
}

func (u *UrlWatcher) executeScript(ctx context.Context, body []byte) error {
	log.Debugf("Executing script")
	var args []string
	if u.Xtrace {
		args = append(args, "-x")
	}
	cmd := exec.CommandContext(ctx, "/bin/bash", args...)
	// Run in a new process group so we can kill children too.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	go func() {
		<-ctx.Done()
		if ctx.Err() == context.DeadlineExceeded {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()
	cmd.Stdin = bytes.NewReader(body)
	buf, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		log.Errorf("Timeout executing script: %s\n%s", err, string(buf))
		return ctx.Err()
	} else if err != nil {
		log.Errorf("Error executing script: %s\n%s", err, string(buf))
		return err
	}
	log.Debugf("Executed script success:\n%s", string(buf))
	return nil
}

func (u *UrlWatcher) Check(ctx context.Context) error {
	body, err := u.fetchScript(ctx)
	if err != nil {
		return err
	}
	if u.Execute {
		u.executeScript(ctx, body)
	}
	return nil
}
