package signal

import "context"

// Handler used for hook.
type Handler func(context.Context) error
