package vaktscan

import (
	"net/http"
	"os"
	"os/exec"

	"github.com/labstack/echo/v4"
)

type scannerStatusResponse struct {
	Trivy   bool `json:"trivy"`
	Nuclei  bool `json:"nuclei"`
	OpenVAS bool `json:"openvas"`
}

// GetScannerStatus reports which scanners are available in the current environment.
// Trivy and Nuclei are bundled in the Docker image; OpenVAS is external (opt-in via env).
func (h *Handler) GetScannerStatus(c echo.Context) error {
	_, trivyErr := exec.LookPath("trivy")
	_, nucleiErr := exec.LookPath("nuclei")
	openvasURL := os.Getenv("VAKT_OPENVAS_URL")

	return c.JSON(http.StatusOK, scannerStatusResponse{
		Trivy:   trivyErr == nil,
		Nuclei:  nucleiErr == nil,
		OpenVAS: openvasURL != "",
	})
}
