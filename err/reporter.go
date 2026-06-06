/*
Package err provides a simple API for capturing errors.
*/
package err

import "context"

type ErrorReporter interface {
	Capture(ctx context.Context, err error)
}
