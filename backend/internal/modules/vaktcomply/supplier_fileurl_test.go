// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import "testing"

// TestValidateSupplierAnswerURLs is the R1-SA22-03 regression: file_url arrives
// on the UNAUTHENTICATED supplier portal and is later rendered as an <a href> in
// the internal reviewer's UI. Only the app-relative upload path that
// PortalUploadFile itself produces is allowed; anything that could point a
// reviewer at an external target must be rejected.
func TestValidateSupplierAnswerURLs(t *testing.T) {
	mk := func(url string) SaveAnswersInput {
		return SaveAnswersInput{Answers: []AnswerInput{{QuestionID: "q", FileURL: url}}}
	}

	// Rejected — the security cases.
	rejected := []struct {
		name string
		url  string
	}{
		{"absolute https phishing", "https://evil.example/steal"},
		{"absolute http phishing", "http://evil.example/"},
		{"javascript scheme", "javascript:alert(1)"},
		{"protocol-relative", "//evil.example/uploads/supplier-assessments/x"},
		{"path traversal", "/uploads/supplier-assessments/../../etc/passwd"},
		{"wrong prefix", "/uploads/other/x.pdf"},
		{"data uri", "data:text/html,<script>alert(1)</script>"},
		{"bare host", "evil.example/uploads/supplier-assessments/x"},
	}
	for _, tc := range rejected {
		if validateSupplierAnswerURLs(mk(tc.url)) {
			t.Errorf("%s: expected file_url %q to be rejected, but it was accepted", tc.name, tc.url)
		}
	}

	// Accepted — non-vacuity: legitimate portal uploads and empty values pass.
	accepted := []struct {
		name string
		in   SaveAnswersInput
	}{
		{"legit upload path", mk("/uploads/supplier-assessments/1111-2222/abc.pdf")},
		{"empty file_url", mk("")},
		{"no answers", SaveAnswersInput{Answers: nil}},
		{"mixed empty and valid", SaveAnswersInput{Answers: []AnswerInput{
			{QuestionID: "a", FileURL: ""},
			{QuestionID: "b", FileURL: "/uploads/supplier-assessments/x/y.xlsx"},
		}}},
	}
	for _, tc := range accepted {
		if !validateSupplierAnswerURLs(tc.in) {
			t.Errorf("%s: expected input to be accepted, but it was rejected", tc.name)
		}
	}

	// A valid answer alongside a malicious one must still fail the whole set.
	mixed := SaveAnswersInput{Answers: []AnswerInput{
		{QuestionID: "a", FileURL: "/uploads/supplier-assessments/x/y.pdf"},
		{QuestionID: "b", FileURL: "https://evil.example/"},
	}}
	if validateSupplierAnswerURLs(mixed) {
		t.Error("a malicious file_url must fail the set even when another answer is valid")
	}
}
