// SPDX-License-Identifier: AGPL-3.0-or-later
//
// RiraLicense — license server & API
// Copyright (C) 2026 RiraLab — Rizaru Desu. All rights reserved.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// For full license text and ownership/attribution terms, see LICENSE and NOTICE.

package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"

	"riralicense/internal/auth"
)

const maxJSONBodyBytes = 1 << 20 // 1 MiB

func newValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" || name == "" {
			return fld.Name
		}
		return name
	})
	return v
}

func decodeAndValidate(r *http.Request, v *validator.Validate, dst any) (map[string]string, error) {
	defer r.Body.Close()
	limited := io.LimitReader(r.Body, maxJSONBodyBytes)
	dec := json.NewDecoder(limited)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return nil, err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return nil, errors.New("request body must contain a single JSON object")
	}

	if err := v.Struct(dst); err != nil {
		var valErrs validator.ValidationErrors
		if errors.As(err, &valErrs) {
			fields := make(map[string]string, len(valErrs))
			for _, fe := range valErrs {
				fields[fe.Field()] = formatValidationError(fe)
			}
			return fields, err
		}
		return nil, err
	}
	return nil, nil
}

func formatValidationError(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return fe.Field() + " is required"
	case "email":
		return fe.Field() + " must be a valid email address"
	case "min":
		return fe.Field() + " must be at least " + fe.Param() + " characters"
	case "max":
		return fe.Field() + " must be at most " + fe.Param() + " characters"
	default:
		return fe.Field() + " is invalid"
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeErrorCode(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{
		"code":  code,
		"error": message,
	})
}

func writeValidationError(w http.ResponseWriter, fields map[string]string) {
	writeJSON(w, http.StatusBadRequest, map[string]any{
		"error":  "validation failed",
		"fields": fields,
	})
}

func writeTooManyRequests(w http.ResponseWriter, retryAfter time.Duration) {
	secs := int(retryAfter.Seconds())
	if secs < 1 {
		secs = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(secs))
	writeError(w, http.StatusTooManyRequests, "too many requests, please try again later")
}

func requestMeta(r *http.Request) auth.RequestMeta {
	return auth.RequestMeta{
		IP:        extractClientIP(r),
		UserAgent: r.UserAgent(),
	}
}

func extractClientIPString(r *http.Request) string {
	if ip := extractClientIP(r); ip != nil {
		return ip.String()
	}
	return r.RemoteAddr
}

func extractClientIP(r *http.Request) *netip.Addr {
	raw := strings.TrimSpace(r.RemoteAddr)
	if raw == "" {
		return nil
	}
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	}
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return nil
	}
	unmapped := addr.Unmap()
	return &unmapped
}
