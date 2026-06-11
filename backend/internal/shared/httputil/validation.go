// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package httputil provides shared HTTP helpers for Vakt handlers.
package httputil

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// HumanValidationError converts a go-playground/validator error into a
// user-facing message that does not leak internal Go struct field names.
//
// Raw validator strings (e.g. "Key: 'CreateWebhookInput.URL' Error:Field
// validation for 'URL' failed on the 'required' tag") must never reach the
// client — this function maps the most common tags to natural language.
func HumanValidationError(err error) string {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return "Validation failed — check your input."
	}
	for _, fe := range ve {
		field := strings.ToLower(fe.Field())
		switch fe.Tag() {
		case "required":
			return fmt.Sprintf("Required field missing: %s.", fe.Field())
		case "email":
			return "Not a valid email address."
		case "url", "http_url":
			return "Not a valid URL."
		case "min":
			switch field {
			case "password":
				return fmt.Sprintf("Password must be at least %s characters.", fe.Param())
			case "name", "display_name":
				return fmt.Sprintf("%s must be at least %s characters.", fe.Field(), fe.Param())
			default:
				return fmt.Sprintf("%s is too short (minimum %s).", fe.Field(), fe.Param())
			}
		case "max":
			return fmt.Sprintf("%s is too long (maximum %s).", fe.Field(), fe.Param())
		case "oneof":
			return fmt.Sprintf("Invalid value for %s.", fe.Field())
		}
	}
	return "Validation failed — check your input."
}
