// Code generated by protoc-gen-validate. DO NOT EDIT.
// source: dynabuf.proto

package dynabuf

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/types/known/anypb"
)

// ensure the imports are used
var (
	_ = bytes.MinRead
	_ = errors.New("")
	_ = fmt.Print
	_ = utf8.UTFMax
	_ = (*regexp.Regexp)(nil)
	_ = (*strings.Reader)(nil)
	_ = net.IPv4len
	_ = time.Duration(0)
	_ = (*url.URL)(nil)
	_ = (*mail.Address)(nil)
	_ = anypb.Any{}
	_ = sort.Sort
)

// Validate checks the field values on Field with the rules defined in the
// proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *Field) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on Field with the rules defined in the
// proto definition for this message. If any rules are violated, the result is
// a list of violation errors wrapped in FieldMultiError, or nil if none found.
func (m *Field) ValidateAll() error {
	return m.validate(true)
}

func (m *Field) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if m.PartitionKey != nil {
		// no validation rules for PartitionKey
	}

	if m.SortKey != nil {
		// no validation rules for SortKey
	}

	if len(errors) > 0 {
		return FieldMultiError(errors)
	}

	return nil
}

// FieldMultiError is an error wrapping multiple validation errors returned by
// Field.ValidateAll() if the designated constraints aren't met.
type FieldMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m FieldMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m FieldMultiError) AllErrors() []error { return m }

// FieldValidationError is the validation error returned by Field.Validate if
// the designated constraints aren't met.
type FieldValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e FieldValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e FieldValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e FieldValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e FieldValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e FieldValidationError) ErrorName() string { return "FieldValidationError" }

// Error satisfies the builtin error interface
func (e FieldValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sField.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = FieldValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = FieldValidationError{}

// Validate checks the field values on Table with the rules defined in the
// proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *Table) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on Table with the rules defined in the
// proto definition for this message. If any rules are violated, the result is
// a list of violation errors wrapped in TableMultiError, or nil if none found.
func (m *Table) ValidateAll() error {
	return m.validate(true)
}

func (m *Table) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for Name

	if m.BillingMode != nil {
		// no validation rules for BillingMode
	}

	if len(errors) > 0 {
		return TableMultiError(errors)
	}

	return nil
}

// TableMultiError is an error wrapping multiple validation errors returned by
// Table.ValidateAll() if the designated constraints aren't met.
type TableMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m TableMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m TableMultiError) AllErrors() []error { return m }

// TableValidationError is the validation error returned by Table.Validate if
// the designated constraints aren't met.
type TableValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e TableValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e TableValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e TableValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e TableValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e TableValidationError) ErrorName() string { return "TableValidationError" }

// Error satisfies the builtin error interface
func (e TableValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sTable.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = TableValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = TableValidationError{}
