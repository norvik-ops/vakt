// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

// buildAWSConfig constructs an aws.Config from an AWSConfig struct.
// Used by the service layer for lightweight connectivity tests.
func buildAWSConfig(cfg *AWSConfig) aws.Config {
	return aws.Config{
		Region: cfg.Region,
		Credentials: aws.NewCredentialsCache(
			credentials.NewStaticCredentialsProvider(
				cfg.AccessKeyID, cfg.SecretAccessKey, "",
			),
		),
	}
}
