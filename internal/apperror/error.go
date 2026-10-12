// Package apperror defines errors that can safely cross the HTTP API boundary.
package apperror

import "fmt"

// Error is an application error safe to expose through the HTTP API.
type Error struct {
	Status  int    // HTTP status returned to the client.
	Code    string // Stable machine-readable code used by the frontend.
	Message string // Human-readable message that is safe to expose.
	Cause   error  // Internal cause retained for logs, but never sent to the client.
	Details any    // Optional structured validation details that are safe to expose.
}

// New creates an API-safe error without an underlying cause.
func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

// Wrap creates an API-safe error while preserving the internal cause for logging.
func Wrap(status int, code, message string, cause error) *Error {
	return &Error{Status: status, Code: code, Message: message, Cause: cause}
}

// WrapWithDetails สร้าง API error ที่มีรายละเอียดแบบมีโครงสร้าง เช่น
// รายการแถวที่ผิดจากไฟล์ import โดยยังคง code และ message รูปแบบกลางไว้.
func WrapWithDetails(status int, code, message string, details any, cause error) *Error {
	return &Error{Status: status, Code: code, Message: message, Cause: cause, Details: details}
}

// Error implements the standard error interface.
func (e *Error) Error() string {
	if e.Cause == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Cause)
}

// Unwrap allows errors.Is and errors.As to inspect the underlying cause.
func (e *Error) Unwrap() error {
	return e.Cause
}
