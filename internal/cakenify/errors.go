package cakenify

import "errors"

var (
	ErrInternal           = errors.New("cakenify: internal error")
	ErrInvalidParameters  = errors.New("cakenify: invalid request parameters")
	ErrNotFound           = errors.New("cakenify: not found")
	ErrInvalidCredentials = errors.New("cakenify: invalid username or password")
	ErrUnauthorized       = errors.New("cakenify: unauthorized")
	ErrForbidden          = errors.New("cakenify: forbidden")
	ErrUsernameTaken      = errors.New("cakenify: username already taken")
	ErrUnknownStrategy    = errors.New("cakenify: unknown notification strategy")
)
