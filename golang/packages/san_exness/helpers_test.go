package san_exness

import (
	"context"
	"time"
)

var errDeadline = context.DeadlineExceeded

func contextWithTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}
