package mock

//go:generate go run go.uber.org/mock/mockgen -package mock -destination mock.go cakenify/internal/cakenify Service,Notifier
