package vaktcomply

import "context"

// --- Access Review Campaigns ---

func (s *Service) ListAccessReviewCampaigns(ctx context.Context, orgID string) ([]AccessReviewCampaign, error) {
	return s.repo.ListAccessReviewCampaigns(ctx, orgID)
}

func (s *Service) GetAccessReviewCampaign(ctx context.Context, orgID, id string) (*AccessReviewCampaign, error) {
	return s.repo.GetAccessReviewCampaign(ctx, orgID, id)
}

func (s *Service) CreateAccessReviewCampaign(ctx context.Context, orgID string, in CreateAccessReviewCampaignInput) (*AccessReviewCampaign, error) {
	return s.repo.CreateAccessReviewCampaign(ctx, orgID, in)
}

func (s *Service) UpdateAccessReviewCampaign(ctx context.Context, orgID, id string, in UpdateAccessReviewCampaignInput) (*AccessReviewCampaign, error) {
	return s.repo.UpdateAccessReviewCampaign(ctx, orgID, id, in)
}

func (s *Service) DeleteAccessReviewCampaign(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteAccessReviewCampaign(ctx, orgID, id)
}

// --- Access Review Items ---

func (s *Service) ListAccessReviewItems(ctx context.Context, orgID, campaignID string) ([]AccessReviewItem, error) {
	return s.repo.ListAccessReviewItems(ctx, orgID, campaignID)
}

func (s *Service) CreateAccessReviewItem(ctx context.Context, orgID string, in CreateAccessReviewItemInput) (*AccessReviewItem, error) {
	return s.repo.CreateAccessReviewItem(ctx, orgID, in)
}

func (s *Service) UpdateAccessReviewItem(ctx context.Context, orgID, id string, in UpdateAccessReviewItemInput) (*AccessReviewItem, error) {
	return s.repo.UpdateAccessReviewItem(ctx, orgID, id, in)
}
