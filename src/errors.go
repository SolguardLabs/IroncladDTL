package main

import "fmt"

type AppError struct {
	Code    string `json:"code"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

func (e *AppError) Error() string {
	if e.Field == "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s[%s]: %s", e.Code, e.Field, e.Message)
}

func NewValidationError(field string, message string) *AppError {
	return &AppError{Code: "validation_error", Field: field, Message: message}
}

func NewNotFoundError(field string, id string) *AppError {
	return &AppError{Code: "not_found", Field: field, Message: id}
}

func NewConflictError(field string, message string) *AppError {
	return &AppError{Code: "conflict", Field: field, Message: message}
}

func NewInsufficientFundsError(field string, requested int64, available int64) *AppError {
	return &AppError{
		Code:    "insufficient_funds",
		Field:   field,
		Message: fmt.Sprintf("requested %d available %d", requested, available),
	}
}

func ErrorCode(err error) string {
	if err == nil {
		return ""
	}
	if app, ok := err.(*AppError); ok {
		return app.Code
	}
	return "internal_error"
}

func ErrorField(err error) string {
	if err == nil {
		return ""
	}
	if app, ok := err.(*AppError); ok {
		return app.Field
	}
	return ""
}

func ErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	if app, ok := err.(*AppError); ok {
		return app.Message
	}
	return err.Error()
}
