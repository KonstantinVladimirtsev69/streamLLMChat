package model

import "errors"

// Domain errors for database and business logic.
var (
	// ErrNotFound is returned when an entity is not found in the database.
	ErrNotFound = errors.New("entity not found")

	// ErrInsufficientBalance is returned when attempting to deduct more than the available balance.
	ErrInsufficientBalance = errors.New("insufficient balance")

	// ErrUserAlreadyExists is returned when a user with the given VK ID already exists.
	ErrUserAlreadyExists = errors.New("user already exists")

	// ErrInvalidOperation is returned when a requested operation violates domain constraints.
	ErrInvalidOperation = errors.New("invalid operation")
)
