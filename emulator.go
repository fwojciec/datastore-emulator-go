package emulator

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"
)

var (
	timeout             = 30 * time.Second
	pollingRate         = 200 * time.Millisecond
	resetEndpoint       = "/reset"
	shutdownEndpoint    = "/shutdown"
	healthcheckEndpoint = ""
	defaultProject      = "test"
	defaultHost         = "localhost:8088"
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
		"--consistency=1.0",         // prevents random test failures
		"--no-store-on-disk",        // test in memory
		"--host-port="+defaultHost,  // use a specific port
		"--project="+defaultProject, // use a specific project name for tests
	).Start(); err != nil {
		return err
	}
	e.Host = "http://" + defaultHost
	e.ProjectID = defaultProject
	if err := e.confirmStartup(); err != nil {
		_ = e.Close()
		return err
	}
	os.Setenv("DATASTORE_EMULATOR_HOST", defaultHost)
	os.Setenv("DATASTORE_PROJECT_ID", defaultProject)
	return nil
}

// Reset resets the Datastore Emulator (but only works in testing/i.e. when
// using in-memory storage).
func (e *Emulator) Reset() error {
	return e.request(resetEndpoint, http.MethodPost)
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
	// check health of the running instance
	if err := e.request(host, http.MethodGet); err != nil {
		return false
	}
	e.Host = host
	e.ProjectID = projectID
	return true
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

func (e *Emulator) initEnv() {
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
