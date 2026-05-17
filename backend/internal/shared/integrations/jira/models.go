// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package jira

import "time"

// Config holds the Jira integration configuration for an organisation.
// The APIToken field is always masked ("****") when returned to the client.
type Config struct {
	JiraURL       string `json:"jira_url"`
	ProjectKey    string `json:"project_key"`
	UserEmail     string `json:"user_email"`
	APITokenMask  string `json:"api_token"`     // always "****" in API responses
	IsConfigured  bool   `json:"is_configured"` // true when all fields are set
}

// SaveConfigInput is the validated request body for saving Jira config.
type SaveConfigInput struct {
	JiraURL    string `json:"jira_url"     validate:"required,url"`
	ProjectKey string `json:"project_key"  validate:"required"`
	UserEmail  string `json:"user_email"   validate:"required,email"`
	APIToken   string `json:"api_token"    validate:"required"`
}

// TestResult is returned by the test-connection endpoint.
type TestResult struct {
	Success     bool   `json:"success"`
	DisplayName string `json:"display_name,omitempty"` // from Jira /myself
	Error       string `json:"error,omitempty"`
}

// CreateIssueResult is returned when a Jira issue is successfully created.
type CreateIssueResult struct {
	IssueKey string `json:"issue_key"`
	IssueURL string `json:"issue_url"`
}

// JiraIssue is a row from the jira_issues tracking table.
type JiraIssue struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	FindingID   string    `json:"finding_id"`
	IssueKey    string    `json:"issue_key"`
	IssueURL    string    `json:"issue_url"`
	CreatedAt   time.Time `json:"created_at"`
}
