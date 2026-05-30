package db

import "time"

// DefaultQueryTimeout is the maximum duration for a database query before
// the context is cancelled and a timeout error is returned.
// This value is enforced by the DBTimeoutMiddleware in the handlers package.
const DefaultQueryTimeout = 5 * time.Second
