package cakenifydb

import "errors"

var (
	ErrInternal      = errors.New("cakenifydb: internal error")
	ErrInvalidParams = errors.New("cakenifydb: invalid parameters")
	ErrNotFound      = errors.New("cakenifydb: not found")
	ErrAlreadyExists = errors.New("cakenifydb: already exists")
)
