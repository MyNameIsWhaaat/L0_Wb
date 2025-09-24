package service

import "errors"

var ErrNotFound = errors.New("not found")

var (
	ErrDecode     = errors.New("decode")
	ErrValidation = errors.New("validation")
)
