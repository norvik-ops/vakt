package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// siemExportEntry is the read model for a single audit log row used in SIEM exports.
type siemExportEntry struct {
	ID           string
	OrgID        string
	UserID       string
	Action       string
	ResourceType string
	ResourceID   string
	ResourceName string
	IPAddress    string
	CreatedAt    time.Time
}

// SIEMHandler handles SIEM export endpoints for the audit log.
type SIEMHandler struct {
	db *pgxpool.Pool
}

// NewSIEMHandler constructs a SIEMHandler.
func NewSIEMHandler(db *pgxpool.Pool) *SIEMHandler {
	return &SIEMHandler{db: db}
}

// fetchExportEntries queries audit_log entries for the given org within the last
// `days` days.  Maximum 365 days; default 30.
func (h *SIEMHandler) fetchExportEntries(c echo.Context, orgID string) ([]siemExportEntry, int, error) {
	days := 30
	if raw := c.QueryParam("days"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			days = n
		}
	}
	if days > 365 {
		days = 365
	}

	from := time.Now().UTC().AddDate(0, 0, -days)

	rows, err := h.db.Query(c.Request().Context(), `
		SELECT
			id::text,
			org_id::text,
			COALESCE(user_id::text, ''),
			action,
			resource_type,
			COALESCE(resource_id, ''),
			COALESCE(resource_name, ''),
			COALESCE(ip_address, ''),
			created_at
		FROM audit_log
		WHERE org_id = $1::uuid
		  AND created_at >= $2
		  AND deleted_at IS NULL
		ORDER BY created_at ASC`,
		orgID, from,
	)
	if err != nil {
		return nil, days, fmt.Errorf("query audit log for SIEM export: %w", err)
	}
	defer rows.Close()

	var entries []siemExportEntry
	for rows.Next() {
		var e siemExportEntry
		if err := rows.Scan(
			&e.ID, &e.OrgID, &e.UserID, &e.Action, &e.ResourceType,
			&e.ResourceID, &e.ResourceName, &e.IPAddress, &e.CreatedAt,
		); err != nil {
			return nil, days, fmt.Errorf("scan audit log row: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, days, fmt.Errorf("iterate audit log rows: %w", err)
	}
	return entries, days, nil
}

// cefEscape replaces pipe (|) and backslash (\) characters in CEF header fields.
// CEF spec requires these characters to be escaped with a preceding backslash.
func cefEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "|", `\|`)
	return s
}

// cefExtEscape escapes values in the CEF extension portion (key=value pairs).
// The CEF spec requires = and \ to be escaped in extension values.
func cefExtEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "=", `\=`)
	// Newlines in extension values are not permitted; replace with space.
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

// toCEFLine converts one siemExportEntry to a single CEF 0 line.
//
// Format:
//
//	CEF:0|Vakt|Vakt Comply|1.0|<action>|<resource_type> <action>|5|rt=<ts> suser=<user_id> ...
func toCEFLine(e siemExportEntry) string {
	// RFC 5424 / ArcSight CEF timestamp format: milliseconds since epoch
	rt := e.CreatedAt.UTC().UnixMilli()

	return fmt.Sprintf(
		"CEF:0|Vakt|Vakt Comply|1.0|%s|%s %s|5|rt=%d suser=%s cs1=%s cs1Label=OrgID cs2=%s cs2Label=ResourceID cs3=%s cs3Label=ResourceName src=%s",
		cefEscape(e.Action),
		cefEscape(e.ResourceType),
		cefEscape(e.Action),
		rt,
		cefExtEscape(e.UserID),
		cefExtEscape(e.OrgID),
		cefExtEscape(e.ResourceID),
		cefExtEscape(e.ResourceName),
		cefExtEscape(e.IPAddress),
	)
}

// ExportCEF handles GET /api/v1/admin/audit-log/export.cef
//
// Query params:
//
//	days  int  — number of days to look back (default 30, max 365)
func (h *SIEMHandler) ExportCEF(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
			"code":  "AUTH_MISSING_ORG",
		})
	}

	entries, _, err := h.fetchExportEntries(c, orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("siem cef export failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to export audit log",
			"code":  "ADMIN_SIEM_EXPORT_ERROR",
		})
	}

	date := time.Now().UTC().Format("2006-01-02")
	filename := fmt.Sprintf("vakt-audit-%s.cef", date)

	c.Response().Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Response().WriteHeader(http.StatusOK)

	w := c.Response().Writer
	for _, e := range entries {
		fmt.Fprintln(w, toCEFLine(e))
	}

	return nil
}

// toSyslogLine converts one siemExportEntry to a syslog-compatible RFC 5424 line.
//
// Format:
//
//	<134>1 <timestamp> vakt Vakt - - [vakt@12345 action="..." resource="..." user="..." org="..."] <resource_name>
func toSyslogLine(e siemExportEntry) string {
	// Priority 134 = Facility 16 (local0) * 8 + Severity 6 (informational)
	ts := e.CreatedAt.UTC().Format(time.RFC3339)

	// Structured data: escape " and \ in SD-PARAM values per RFC 5424.
	sdEscape := func(s string) string {
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, `"`, `\"`)
		s = strings.ReplaceAll(s, "]", `\]`)
		return s
	}

	sd := fmt.Sprintf(
		`[vakt@12345 action="%s" resource="%s" user="%s" org="%s"]`,
		sdEscape(e.Action),
		sdEscape(e.ResourceType),
		sdEscape(e.UserID),
		sdEscape(e.OrgID),
	)

	// MSG: resource_name (or "-" if empty)
	msg := e.ResourceName
	if msg == "" {
		msg = "-"
	}

	return fmt.Sprintf("<134>1 %s vakt Vakt - - %s %s", ts, sd, msg)
}

// ExportSyslog handles GET /api/v1/admin/audit-log/export.syslog
//
// Query params:
//
//	days  int  — number of days to look back (default 30, max 365)
func (h *SIEMHandler) ExportSyslog(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
			"code":  "AUTH_MISSING_ORG",
		})
	}

	entries, _, err := h.fetchExportEntries(c, orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("siem syslog export failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to export audit log",
			"code":  "ADMIN_SIEM_EXPORT_ERROR",
		})
	}

	date := time.Now().UTC().Format("2006-01-02")
	filename := fmt.Sprintf("vakt-audit-%s.syslog", date)

	c.Response().Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Response().WriteHeader(http.StatusOK)

	w := c.Response().Writer
	for _, e := range entries {
		fmt.Fprintln(w, toSyslogLine(e))
	}

	return nil
}
