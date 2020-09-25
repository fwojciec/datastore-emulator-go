# datastore-emulator-go

## Description
This is a simple library which helps with testing GCP Datastore code written in
Go. The library wraps the
[Datastore emulator](https://cloud.google.com/datastore/docs/tools/datastore-emulator) and provides basic functionality such as starting/stopping and
reseting the emulator instance from inside the test runner.

## Example TestMain

```go
package repo_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"cloud.google.com/go/datastore"
	emulator "github.com/fwojciec/datastore-emulator-go"
)

var (
    dc *datastore.Client
    e *emulator.Emulator
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	code, err := runTests(ctx, m)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error while running tests: %v\n", err)
		os.Exit(code)
	}
	os.Exit(code)
}

func runTests(ctx context.Context, m *testing.M) (int, error) {
	emulator, err := emulator.New()
	if err != nil {
		return 1, err
	}
	defer emulator.Close()
	client, err := datastore.NewClient(ctx, emulator.ProjectID)
	if err != nil {
		return 1, err
	}
	defer client.Close()
	dc = client
    e = emulator
	return m.Run(), nil
}
```

## Caveats

You have to run the tests sequentially, which is a bummer... 😞

## Features

- Runs the emulator instance.
- Sets the necessary environment variables.
- Tears down cleanly at the end of the tests.
- Runs tests in memory by default.
- Can be used to reset Datastore state between tests.
- If a running and healthy datastore emulator instance is detected it'll be preferred to spawining a new one and it will not be shut down at the end of the test run (that way you have an option to keep an instance running manually for faster tests.)
