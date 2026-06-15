package signal_test

import (
	"context"
	"fmt"
	"time"

	"github.com/alexfalkowski/go-signal"
)

func Example() {
	lifecycle := signal.NewLifeCycle(time.Second)
	events := make([]string, 0, 3)

	lifecycle.Register(signal.Hook{
		OnStart: func(context.Context) error {
			events = append(events, "start")
			return nil
		},
		OnStop: func(context.Context) error {
			events = append(events, "stop")
			return nil
		},
	})

	err := lifecycle.Run(context.Background(), func(context.Context) error {
		events = append(events, "run")
		return nil
	})

	fmt.Println(err)
	fmt.Println(events)

	// Output:
	// <nil>
	// [start run stop]
}
