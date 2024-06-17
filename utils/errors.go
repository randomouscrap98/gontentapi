package utils

import ()

type BadRequest struct {
	Message string
}

func (e *BadRequest) Error() string {
	return e.Message
}

type NotFound struct {
	Message string
}

func (e *NotFound) Error() string {
	return e.Message
}
