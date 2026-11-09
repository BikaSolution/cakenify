package cronjob

import "errors"

var (
	ErrIntervalMustBePositive  = errors.New("cronjob: interval must be positive")
	ErrTaskFunctionCannotBeNil = errors.New("cronjob: task function cannot be nil")
)
