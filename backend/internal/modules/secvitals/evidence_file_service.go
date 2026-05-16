package secvitals

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// allowedEvidenceMIME lists permitted MIME types for evidence file uploads.
var allowedEvidenceMIME = map[string]bool{
	"application/pdf":                                                               true,
	"image/png":                                                                     true,
	"image/jpeg":                                                                    true,
	"text/plain":                                                                    true,
	"text/csv":                                                                      true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":             true, // xlsx
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":       true, // docx
	"application/zip":                                                               true,
	"application/x-zip-compressed":                                                  true,
}

// allowedEvidenceExt lists permitted file extensions (lower-case, with dot).
var allowedEvidenceExt = map[string]bool{
	".pdf":  true,
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".txt":  true,
	".csv":  true,
	".xlsx": true,
	".docx": true,
	".zip":  true,
}

const maxEvidenceFileSizeBytes = 50 * 1024 * 1024 // 50 MB

// EvidenceFileService handles storage and retrieval of evidence file attachments.
type EvidenceFileService struct {
	repo      *Repository
	uploadDir string
}

// NewEvidenceFileService creates a new EvidenceFileService.
func NewEvidenceFileService(repo *Repository, uploadDir string) *EvidenceFileService {
	if uploadDir == "" {
		uploadDir = "./data/uploads"
	}
	return &EvidenceFileService{repo: repo, uploadDir: uploadDir}
}

// Upload validates, stores, and records a new evidence file.
// evidenceID may be empty when a file is attached directly to a control without a parent evidence record.
func (s *EvidenceFileService) Upload(
	ctx context.Context,
	orgID, controlID, evidenceID, uploaderID string,
	file multipart.File,
	header *multipart.FileHeader,
) (EvidenceFile, error) {
	// Size check
	if header.Size > maxEvidenceFileSizeBytes {
		return EvidenceFile{}, fmt.Errorf("file too large: max 50 MB")
	}

	// Extension check
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedEvidenceExt[ext] {
		return EvidenceFile{}, fmt.Errorf("file type not allowed: %s", ext)
	}

	// MIME check (use header value; browsers set this based on OS type detection)
	mime := header.Header.Get("Content-Type")
	// Strip parameters (e.g. "text/plain; charset=utf-8" → "text/plain")
	if idx := strings.Index(mime, ";"); idx != -1 {
		mime = strings.TrimSpace(mime[:idx])
	}
	if mime != "" && !allowedEvidenceMIME[mime] {
		return EvidenceFile{}, fmt.Errorf("MIME type not allowed: %s", mime)
	}

	// Build destination path
	storedName := uuid.New().String() + ext
	dir := filepath.Join(s.uploadDir, "evidence", orgID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return EvidenceFile{}, fmt.Errorf("create upload dir: %w", err)
	}
	destPath := filepath.Join(dir, storedName)

	// Write file to disk
	dst, err := os.Create(destPath)
	if err != nil {
		return EvidenceFile{}, fmt.Errorf("create file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		_ = os.Remove(destPath)
		return EvidenceFile{}, fmt.Errorf("write file: %w", err)
	}

	// Insert DB record
	rec := EvidenceFile{
		OrgID:        orgID,
		EvidenceID:   evidenceID,
		ControlID:    controlID,
		OriginalName: header.Filename,
		StoredName:   storedName,
		MimeType:     mime,
		SizeBytes:    header.Size,
		UploadedBy:   uploaderID,
	}
	out, err := s.repo.CreateEvidenceFile(ctx, rec)
	if err != nil {
		_ = os.Remove(destPath)
		return EvidenceFile{}, fmt.Errorf("record evidence file: %w", err)
	}

	out.DownloadURL = "/api/v1/secvitals/evidence-files/" + out.ID + "/download"
	return out, nil
}

// Download returns the file metadata and full disk path for streaming.
func (s *EvidenceFileService) Download(ctx context.Context, orgID, fileID string) (EvidenceFile, string, error) {
	f, err := s.repo.GetEvidenceFile(ctx, orgID, fileID)
	if err != nil {
		return EvidenceFile{}, "", fmt.Errorf("get evidence file: %w", err)
	}
	diskPath := filepath.Join(s.uploadDir, "evidence", orgID, f.StoredName)
	f.DownloadURL = "/api/v1/secvitals/evidence-files/" + f.ID + "/download"
	return f, diskPath, nil
}

// Delete removes the DB record and the associated file from disk.
func (s *EvidenceFileService) Delete(ctx context.Context, orgID, fileID string) error {
	f, err := s.repo.DeleteEvidenceFile(ctx, orgID, fileID)
	if err != nil {
		return fmt.Errorf("delete evidence file record: %w", err)
	}
	diskPath := filepath.Join(s.uploadDir, "evidence", orgID, f.StoredName)
	if err := os.Remove(diskPath); err != nil && !os.IsNotExist(err) {
		// Log but don't fail — the DB record is already gone.
		_ = err
	}
	return nil
}

// ListForEvidence returns all files attached to a specific evidence record.
func (s *EvidenceFileService) ListForEvidence(ctx context.Context, orgID, evidenceID string) ([]EvidenceFile, error) {
	items, err := s.repo.ListEvidenceFiles(ctx, orgID, evidenceID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].DownloadURL = "/api/v1/secvitals/evidence-files/" + items[i].ID + "/download"
	}
	return items, nil
}

// ListForControl returns all files attached to any evidence under a given control.
func (s *EvidenceFileService) ListForControl(ctx context.Context, orgID, controlID string) ([]EvidenceFile, error) {
	items, err := s.repo.ListEvidenceFilesByControl(ctx, orgID, controlID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].DownloadURL = "/api/v1/secvitals/evidence-files/" + items[i].ID + "/download"
	}
	return items, nil
}
