package signal_test

import (
	"context"
	"fmt"
	"time"

	"github.com/alexfalkowski/go-signal/v2"
)

func ExampleTimer() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := signal.Timer(ctx, time.Millisecond, func(context.Context) error {
		fmt.Println("tick")
		cancel()
		return nil
	})
	fmt.Println(err)
	// Output:
	// tick
	// <nil>
}

func ExampleApp_Serve() {
	app, err := signal.NewApp(signal.Register(func(sh signal.Shutdowner) {
		sh.Shutdown(0)
	}))
	if err != nil {
		panic(err)
	}

	fmt.Println(app.Serve(context.Background()))
	// Output: <nil>
}

func ExampleNewApp() {
	app, err := signal.NewApp(
		signal.Constructor(func() string {
			return "hello"
		}),
		signal.Register(func(lc signal.Lifecycle, message string) {
			lc.Append(signal.Hook{
				OnStart: func(context.Context) error {
					fmt.Println(message)
					return nil
				},
				OnStop: func(context.Context) error {
					fmt.Println("stopped")
					return nil
				},
			})
		}),
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(app.Run(context.Background()))
	// Output:
	// hello
	// stopped
	// <nil>
}
