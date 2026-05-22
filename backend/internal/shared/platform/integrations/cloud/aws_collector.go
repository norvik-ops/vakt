// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

const awsSource = "aws-collector"

// AWSCollector collects compliance evidence from an AWS account.
type AWSCollector struct {
	db       *pgxpool.Pool
	evidence EvidenceWriter
}

// NewAWSCollector creates a new AWSCollector.
func NewAWSCollector(db *pgxpool.Pool, evidence EvidenceWriter) *AWSCollector {
	return &AWSCollector{
		db:       db,
		evidence: evidence,
	}
}

// Collect runs all AWS evidence collectors for the given org and config.
// Returns the number of evidence items created.
func (c *AWSCollector) Collect(ctx context.Context, orgID string, cfg AWSConfig) (int, error) {
	awsCfg, err := awscfg.LoadDefaultConfig(ctx,
		awscfg.WithRegion(cfg.Region),
		awscfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID, cfg.SecretAccessKey, "",
		)),
	)
	if err != nil {
		return 0, fmt.Errorf("load aws config: %w", err)
	}

	// Find IAM controls to attach evidence to (best-effort; nil = no control link)
	iamControls, err := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"iam", "access", "identity", "password", "mfa"})
	if err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("aws_collector: no iam controls found")
	}

	cloudtrailControls, err := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"audit", "log", "trail", "monitoring"})
	if err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("aws_collector: no cloudtrail controls found")
	}

	storageControls, err := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"encryption", "storage", "s3", "backup"})
	if err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("aws_collector: no storage controls found")
	}

	total := 0

	// IAM password policy
	if n, err := c.collectPasswordPolicy(ctx, orgID, awsCfg, iamControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("aws_collector: password policy collection failed")
	} else {
		total += n
	}

	// IAM MFA status
	if n, err := c.collectMFAStatus(ctx, orgID, awsCfg, iamControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("aws_collector: mfa status collection failed")
	} else {
		total += n
	}

	// IAM credential report
	if n, err := c.collectCredentialReport(ctx, orgID, awsCfg, iamControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("aws_collector: credential report collection failed")
	} else {
		total += n
	}

	// CloudTrail configuration
	if n, err := c.collectCloudTrail(ctx, orgID, awsCfg, cloudtrailControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("aws_collector: cloudtrail collection failed")
	} else {
		total += n
	}

	// S3 encryption + versioning
	if n, err := c.collectS3(ctx, orgID, awsCfg, storageControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("aws_collector: s3 collection failed")
	} else {
		total += n
	}

	return total, nil
}

// firstControlID returns the ID of the first control or "" if empty.
func firstControlID(controls []ControlMatch) string {
	if len(controls) > 0 {
		return controls[0].ID
	}
	return ""
}

func (c *AWSCollector) addEvidence(ctx context.Context, orgID, controlID, title string, details map[string]any) error {
	data, _ := json.Marshal(details)
	if controlID == "" {
		// No matching control — store without control link by finding any control (best-effort)
		// We use AddCollectorEvidence with an empty controlID UUID workaround: skip if no control
		log.Debug().Str("org_id", orgID).Str("title", title).Msg("aws_collector: no matching control, skipping evidence")
		return nil
	}
	return c.evidence.AddCollectorEvidence(ctx, orgID, controlID, "", awsSource, title, data)
}

// collectPasswordPolicy collects IAM account password policy evidence.
func (c *AWSCollector) collectPasswordPolicy(ctx context.Context, orgID string, awsCfg aws.Config, controls []ControlMatch) (int, error) {
	iamClient := iam.NewFromConfig(awsCfg)

	out, err := iamClient.GetAccountPasswordPolicy(ctx, &iam.GetAccountPasswordPolicyInput{})
	if err != nil {
		return 0, fmt.Errorf("GetAccountPasswordPolicy: %w", err)
	}

	p := out.PasswordPolicy
	details := map[string]any{
		"collected_at":              time.Now().UTC().Format(time.RFC3339),
		"min_password_length":       aws.ToInt32(p.MinimumPasswordLength),
		"require_uppercase":         p.RequireUppercaseCharacters,
		"require_lowercase":         p.RequireLowercaseCharacters,
		"require_numbers":           p.RequireNumbers,
		"require_symbols":           p.RequireSymbols,
		"allow_users_to_change":     p.AllowUsersToChangePassword,
		"max_password_age":          aws.ToInt32(p.MaxPasswordAge),
		"password_reuse_prevention": aws.ToInt32(p.PasswordReusePrevention),
		"hard_expiry":               aws.ToBool(p.HardExpiry),
	}

	controlID := firstControlID(controls)
	if err := c.addEvidence(ctx, orgID, controlID, "AWS IAM Passwort-Richtlinie", details); err != nil {
		return 0, err
	}
	return 1, nil
}

// collectMFAStatus lists all IAM users and checks which have MFA devices enabled.
func (c *AWSCollector) collectMFAStatus(ctx context.Context, orgID string, awsCfg aws.Config, controls []ControlMatch) (int, error) {
	iamClient := iam.NewFromConfig(awsCfg)

	usersOut, err := iamClient.ListUsers(ctx, &iam.ListUsersInput{})
	if err != nil {
		return 0, fmt.Errorf("ListUsers: %w", err)
	}

	total := len(usersOut.Users)
	withMFA := 0
	userSummaries := make([]map[string]any, 0, total)

	for _, u := range usersOut.Users {
		mfaOut, err := iamClient.ListMFADevices(ctx, &iam.ListMFADevicesInput{
			UserName: u.UserName,
		})
		hasMFA := false
		if err == nil && len(mfaOut.MFADevices) > 0 {
			hasMFA = true
			withMFA++
		}
		userSummaries = append(userSummaries, map[string]any{
			"username": aws.ToString(u.UserName),
			"mfa":      hasMFA,
		})
	}

	mfaPercent := 0.0
	if total > 0 {
		mfaPercent = float64(withMFA) / float64(total) * 100.0
	}

	details := map[string]any{
		"collected_at":         time.Now().UTC().Format(time.RFC3339),
		"total_users":          total,
		"users_with_mfa":       withMFA,
		"mfa_coverage_percent": fmt.Sprintf("%.1f%%", mfaPercent),
		"users":                userSummaries,
	}

	controlID := firstControlID(controls)
	if err := c.addEvidence(ctx, orgID, controlID, "AWS IAM MFA-Status", details); err != nil {
		return 0, err
	}
	return 1, nil
}

// collectCredentialReport generates and downloads the IAM credential report.
func (c *AWSCollector) collectCredentialReport(ctx context.Context, orgID string, awsCfg aws.Config, controls []ControlMatch) (int, error) {
	iamClient := iam.NewFromConfig(awsCfg)

	// Generate report (may take a few seconds)
	_, err := iamClient.GenerateCredentialReport(ctx, &iam.GenerateCredentialReportInput{})
	if err != nil {
		return 0, fmt.Errorf("GenerateCredentialReport: %w", err)
	}

	// Retry up to 5 times waiting for report to be ready
	var reportOut *iam.GetCredentialReportOutput
	for i := 0; i < 5; i++ {
		reportOut, err = iamClient.GetCredentialReport(ctx, &iam.GetCredentialReportInput{})
		if err == nil {
			break
		}
		// Simple back-off without sleep (context deadline will handle real timeouts)
		if i < 4 {
			time.Sleep(2 * time.Second)
		}
	}
	if err != nil {
		return 0, fmt.Errorf("GetCredentialReport: %w", err)
	}

	// Parse CSV header line count
	lines := strings.Split(string(reportOut.Content), "\n")
	userCount := len(lines) - 2 // subtract header + trailing newline
	if userCount < 0 {
		userCount = 0
	}

	details := map[string]any{
		"collected_at":         time.Now().UTC().Format(time.RFC3339),
		"generated_time":       aws.ToTime(reportOut.GeneratedTime).Format(time.RFC3339),
		"report_format":        string(reportOut.ReportFormat),
		"user_count_in_report": userCount,
	}

	controlID := firstControlID(controls)
	if err := c.addEvidence(ctx, orgID, controlID, "AWS IAM Credential Report", details); err != nil {
		return 0, err
	}
	return 1, nil
}

// collectCloudTrail collects CloudTrail trail configuration evidence.
func (c *AWSCollector) collectCloudTrail(ctx context.Context, orgID string, awsCfg aws.Config, controls []ControlMatch) (int, error) {
	ctClient := cloudtrail.NewFromConfig(awsCfg)

	out, err := ctClient.DescribeTrails(ctx, &cloudtrail.DescribeTrailsInput{
		IncludeShadowTrails: aws.Bool(false),
	})
	if err != nil {
		return 0, fmt.Errorf("DescribeTrails: %w", err)
	}

	trails := make([]map[string]any, 0, len(out.TrailList))
	for _, t := range out.TrailList {
		trails = append(trails, map[string]any{
			"name":                          aws.ToString(t.Name),
			"s3_bucket":                     aws.ToString(t.S3BucketName),
			"is_multi_region":               aws.ToBool(t.IsMultiRegionTrail),
			"log_file_validation":           aws.ToBool(t.LogFileValidationEnabled),
			"include_global_service_events": aws.ToBool(t.IncludeGlobalServiceEvents),
			"home_region":                   aws.ToString(t.HomeRegion),
		})
	}

	details := map[string]any{
		"collected_at": time.Now().UTC().Format(time.RFC3339),
		"trail_count":  len(out.TrailList),
		"trails":       trails,
	}

	controlID := firstControlID(controls)
	if err := c.addEvidence(ctx, orgID, controlID, "AWS CloudTrail Konfiguration", details); err != nil {
		return 0, err
	}
	return 1, nil
}

// collectS3 checks encryption and versioning for each S3 bucket.
func (c *AWSCollector) collectS3(ctx context.Context, orgID string, awsCfg aws.Config, controls []ControlMatch) (int, error) {
	s3Client := s3.NewFromConfig(awsCfg)

	bucketsOut, err := s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return 0, fmt.Errorf("ListBuckets: %w", err)
	}

	bucketSummaries := make([]map[string]any, 0, len(bucketsOut.Buckets))
	for _, b := range bucketsOut.Buckets {
		name := aws.ToString(b.Name)
		summary := map[string]any{
			"name":       name,
			"created_at": aws.ToTime(b.CreationDate).Format(time.RFC3339),
		}

		// Check encryption
		encOut, encErr := s3Client.GetBucketEncryption(ctx, &s3.GetBucketEncryptionInput{
			Bucket: aws.String(name),
		})
		if encErr == nil && encOut.ServerSideEncryptionConfiguration != nil {
			rules := encOut.ServerSideEncryptionConfiguration.Rules
			if len(rules) > 0 {
				summary["encryption"] = string(rules[0].ApplyServerSideEncryptionByDefault.SSEAlgorithm)
			}
		} else {
			summary["encryption"] = "none"
		}

		// Check versioning
		verOut, verErr := s3Client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{
			Bucket: aws.String(name),
		})
		if verErr == nil {
			summary["versioning"] = string(verOut.Status)
		} else {
			summary["versioning"] = "unknown"
		}

		bucketSummaries = append(bucketSummaries, summary)
	}

	details := map[string]any{
		"collected_at": time.Now().UTC().Format(time.RFC3339),
		"bucket_count": len(bucketsOut.Buckets),
		"buckets":      bucketSummaries,
	}

	controlID := firstControlID(controls)
	if err := c.addEvidence(ctx, orgID, controlID, "AWS S3 Verschlüsselung & Versionierung", details); err != nil {
		return 0, err
	}
	return 1, nil
}
