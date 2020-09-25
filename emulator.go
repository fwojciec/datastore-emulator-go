package emulator

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

var (
	timeout             = 5 * time.Second
	pollingRate         = 200 * time.Millisecond
	resetEndpoint       = "/reset"
	shutdownEndpoint    = "/shutdown"
	healthcheckEndpoint = ""
)

// Emulator manages the GCP Datastore Emulator process.
type Emulator struct {
	Host        string
	ProjectID   string
	stopOnClose bool
}

// New returns a new instance of Emulator.
func New() (*Emulator, error) {
	e := &Emulator{}
	if err := e.Start(); err != nil {
		return nil, err
	}
	return e, nil
}

// Start starts the emulator which involves initializing the environment,
// starting the emulator and blocking until correct startup is confirmed.
// If an instance of the emaulator is already running it will be used instead
// of starting a new instance.
func (e *Emulator) Start() error {
	if e.instanceIsPresent() {
		return nil
	}
	e.stopOnClose = true
	if err := e.command(
		"start",
		"--consistency=1.0",  // prevents random test failures
		"--no-store-on-disk", // test in memory
	).Start(); err != nil {
		return err
	}
	if err := e.initEnv(); err != nil {
		_ = e.Close()
		return err
	}
	if err := e.confirmStartup(); err != nil {
		_ = e.Close()
		return err
	}
	return nil
}

func (e *Emulator) instanceIsPresent() bool {
	host := os.Getenv("DATASTORE_HOST")
	if host == "" {
		return false
	}
	projectID := os.Getenv("DATASTORE_PROJECT_ID")
	if projectID == "" {
		return false
	}
	if !e.isHealthy() {
		return false
	}
	e.Host = host
	e.ProjectID = projectID
	return true
}

// Reset resets the Datastore Emulator (but only works in testing/i.e. when
// using in-memory storage).
func (e *Emulator) Reset() error {
	return e.request(resetEndpoint, http.MethodPost)
}

// Close terminates the emulator process and cleans up the environemental
// variables (only if an instance was started and not recycled).
func (e *Emulator) Close() error {
	if !e.stopOnClose {
		return nil
	}
	os.Unsetenv("DATASTORE_EMULATOR_HOST")
	os.Unsetenv("DATASTORE_PROJECT_ID")
	if e.isHealthy() {
		return e.request(shutdownEndpoint, http.MethodPost)
	}
	return nil
}

func (e *Emulator) initEnv() error {
	var buf bytes.Buffer
	cmd := e.command("env-init")
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		return err
	}
	env := make(map[string]string, 5)
	scanner := bufio.NewScanner(&buf)
	for scanner.Scan() {
		line := scanner.Text()
		e := strings.Split(strings.Split(line, " ")[1], "=")
		env[e[0]] = e[1]
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	e.Host = env["DATASTORE_HOST"]
	e.ProjectID = env["DATASTORE_PROJECT_ID"]
	os.Setenv("DATASTORE_EMULATOR_HOST", env["DATASTORE_EMULATOR_HOST"])
	os.Setenv("DATASTORE_PROJECT_ID", env["DATASTORE_PROJECT_ID"])
	return nil
}

func (e *Emulator) isHealthy() bool {
	if err := e.request(healthcheckEndpoint, http.MethodGet); err != nil {
		return false
	}
	return true
}

func (e *Emulator) confirmStartup() error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	t := time.NewTicker(pollingRate)
	for {
		select {
		case <-t.C:
			if e.isHealthy() {
				t.Stop()
				return nil
			}
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		}
	}
}

func (e *Emulator) command(extraArgs ...string) *exec.Cmd {
	args := []string{"beta", "emulators", "datastore"}
	args = append(args, extraArgs...)
	return exec.Command("gcloud", args...)
}

func (e *Emulator) request(path, method string) error {
	ctx, cancel := context.WithTimeout(context.Background(), pollingRate)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, e.Host+path, nil)
	if err != nil {
		return err
	}
	c := http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("status code error: %d", resp.StatusCode)
	}
	return nil
}
