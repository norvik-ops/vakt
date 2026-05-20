// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

import (
	"context"
	"fmt"
	"time"
)

// --- AI System Inventory ---

func (s *Service) ListAISystems(ctx context.Context, orgID string, filters AISystemFilters) ([]AISystem, error) {
	systems, err := s.repo.ListAISystems(ctx, orgID, filters)
	if err != nil {
		return nil, err
	}
	if systems == nil {
		systems = []AISystem{}
	}
	return systems, nil
}

func (s *Service) GetAISystem(ctx context.Context, orgID, id string) (*AISystem, error) {
	return s.repo.GetAISystem(ctx, orgID, id)
}

func (s *Service) CreateAISystem(ctx context.Context, orgID string, in CreateAISystemInput) (*AISystem, error) {
	return s.repo.CreateAISystem(ctx, orgID, in)
}

func (s *Service) UpdateAISystem(ctx context.Context, orgID, id string, in UpdateAISystemInput) (*AISystem, error) {
	return s.repo.UpdateAISystem(ctx, orgID, id, in)
}

func (s *Service) DeleteAISystem(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteAISystem(ctx, orgID, id)
}

// ClassifyAISystem saves a classification from the wizard and updates the AI system record.
func (s *Service) ClassifyAISystem(ctx context.Context, orgID, systemID string, in ClassifyAISystemInput) error {
	validClasses := map[string]bool{"minimal": true, "limited": true, "high": true, "unacceptable": true}
	if !validClasses[in.RiskClass] {
		return fmt.Errorf("risk_class: must be one of minimal, limited, high, unacceptable")
	}
	if _, err := s.repo.InsertAIClassification(ctx, orgID, systemID, in); err != nil {
		return fmt.Errorf("insert ai classification: %w", err)
	}
	return s.repo.UpdateAISystemClassification(ctx, orgID, systemID, in)
}

// ListAIClassifications returns the classification history for an AI system.
func (s *Service) ListAIClassifications(ctx context.Context, orgID, systemID string) ([]AIClassification, error) {
	return s.repo.ListAIClassifications(ctx, orgID, systemID)
}

// SaveAIDocumentation creates a new documentation version for an AI system.
func (s *Service) SaveAIDocumentation(ctx context.Context, orgID, systemID string, in UpsertAIDocumentationInput) (*AIDocumentation, error) {
	return s.repo.UpsertAIDocumentation(ctx, orgID, systemID, in)
}

// GetLatestAIDocumentation returns the most recent documentation for an AI system.
func (s *Service) GetLatestAIDocumentation(ctx context.Context, orgID, systemID string) (*AIDocumentation, error) {
	doc, err := s.repo.GetLatestAIDocumentation(ctx, orgID, systemID)
	if err != nil {
		return nil, ErrNotFound
	}
	return doc, nil
}

// ListAIDocumentationVersions returns all saved versions.
func (s *Service) ListAIDocumentationVersions(ctx context.Context, orgID, systemID string) ([]AIDocumentation, error) {
	return s.repo.ListAIDocumentationVersions(ctx, orgID, systemID)
}

// ExportAIDocumentationPDF generates the PDF technical dossier for an AI system.
func (s *Service) ExportAIDocumentationPDF(ctx context.Context, orgID, systemID string) ([]byte, string, error) {
	system, err := s.repo.GetAISystem(ctx, orgID, systemID)
	if err != nil {
		return nil, "", ErrNotFound
	}
	doc, err := s.repo.GetLatestAIDocumentation(ctx, orgID, systemID)
	if err != nil {
		// Return PDF with empty documentation fields if none saved yet
		doc = &AIDocumentation{AISystemID: systemID, Version: 0}
	}
	pdfBytes, err := GenerateAIDocumentationPDF(system, doc)
	if err != nil {
		return nil, "", fmt.Errorf("generate ai documentation pdf: %w", err)
	}
	filename := fmt.Sprintf("ai-dossier-%s-v%d.pdf", system.Name, doc.Version)
	return pdfBytes, filename, nil
}

const euAIActHighRiskDeadline = "2026-08-02"

// GetEUAIActDashboard builds the EU AI Act compliance dashboard for an organisation.
func (s *Service) GetEUAIActDashboard(ctx context.Context, orgID string) (*EUAIActDashboard, error) {
	total, byRisk, byStatus, withoutDocs, err := s.repo.GetEUAIActStats(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("get eu ai act dashboard: %w", err)
	}
	deadline, _ := time.Parse("2006-01-02", euAIActHighRiskDeadline)
	daysLeft := int(time.Until(deadline).Hours() / 24)
	if daysLeft < 0 {
		daysLeft = 0
	}
	return &EUAIActDashboard{
		TotalSystems:             total,
		SystemsByRiskClass:       byRisk,
		SystemsByStatus:          byStatus,
		SystemsWithoutDocs:       withoutDocs,
		HighRiskDeadline:         euAIActHighRiskDeadline,
		HighRiskDeadlineDaysLeft: daysLeft,
		ISO27001Mappings:         euAIActISOMappings,
	}, nil
}

// ExportEUAIActReportPDF generates the full EU AI Act compliance report PDF.
func (s *Service) ExportEUAIActReportPDF(ctx context.Context, orgID string) ([]byte, error) {
	dashboard, err := s.GetEUAIActDashboard(ctx, orgID)
	if err != nil {
		return nil, err
	}
	systems, err := s.repo.ListAISystems(ctx, orgID, AISystemFilters{})
	if err != nil {
		return nil, fmt.Errorf("list ai systems for pdf: %w", err)
	}
	return GenerateEUAIActReportPDF(dashboard, systems)
}
