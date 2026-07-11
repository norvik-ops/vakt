// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package rbaccov hosts the cross-package RBAC-coverage regression test
// (S121 Epic E2 / O3). It has no production code — the test in this package
// mounts every shared platform package that exposes mutating HTTP routes and
// asserts that a Viewer token receives 403 on each write route. This closes the
// gap that let R1–R7 (Broken-Access-Control on seven shared packages) ship
// unnoticed: those packages live outside the per-module rbac_test.go files, so
// nothing exercised their authorization.
package rbaccov
