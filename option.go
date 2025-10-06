package signal

import "time"

// Option for Lifecycle.
type Option interface {
	apply(opts *options)
}

type options struct {
	timeout time.Duration
}

type optionFunc func(*options)

func (f optionFunc) apply(o *options) {
	f(o)
}

// WithTimeout for hook.
func WithTimeout(timeout time.Duration) Option {
	return optionFunc(func(o *options) {
		o.timeout = timeout
	})
}

func applyOptions(os ...Option) *options {
	opts := &options{}
	for _, o := range os {
		o.apply(opts)
	}
	if opts.timeout == 0 {
		opts.timeout = 30 * time.Second
	}
	return opts
}
