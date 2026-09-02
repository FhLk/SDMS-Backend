package domain

import "errors"

var (
	ErrUserNotFound = errors.New(
		"user not found",
	)

	ErrInvalidUserID = errors.New(
		"invalid user id",
	)

	ErrUsernameRequired = errors.New(
		"username is required",
	)

	ErrEmployeeCodeRequired = errors.New(
		"employee code is required",
	)

	ErrPrefixRequired = errors.New(
		"prefix is required",
	)

	ErrFirstNameRequired = errors.New(
		"first name is required",
	)

	ErrLastNameRequired = errors.New(
		"last name is required",
	)

	ErrInvalidRole = errors.New(
		"invalid role",
	)

	ErrInvalidStatus = errors.New(
		"invalid status",
	)

	ErrUsernameAlreadyExists = errors.New(
		"username already exists",
	)

	ErrEmployeeCodeAlreadyExists = errors.New(
		"employee code already exists",
	)
)
