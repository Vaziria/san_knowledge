package hub

import "time"

// SetGrace shortens the time added to a command's timeout, so a test of a
// browser that never answers does not wait 5s.
func (h *Hub) SetGrace(d time.Duration) { h.grace = d }
