package config

type Validator interface {
	Validate() error
}

// validatorPtr combines the [Validator] interface with a pointer constraint.
type validatorPtr[T any] interface {
	Validator
	*T
}
