// Package bsi fetches BSI CERT-Bund advisories and creates SecPulse findings.
package bsi

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const certBundFeedURL = "https://www.bsi.bund.de/SiteGlobals/Functions/RSSFeed/RSSNewsfeed_WarnMeldungen.xml"

// Advisory represents a single BSI CERT-Bund warning notice.
type Advisory struct {
	BSIID       string
	Title       string
	Summary     string
	Severity    string // critical / high / medium / low
	PublishedAt time.Time
	URL         string
	CVEIDs      []string
}

// rss is the top-level RSS envelope used for XML unmarshalling.
type rss struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

var cveRegex = regexp.MustCompile(`CVE-\d{4}-\d+`)

// FetchAdvisories downloads and parses the BSI CERT-Bund RSS feed.
// It returns one Advisory per RSS item.
func FetchAdvisories(ctx context.Context) ([]Advisory, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, certBundFeedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("bsi: build request: %w", err)
	}
	req.Header.Set("User-Agent", "Vakt/1.0 (BSI Advisory Sync)")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bsi: fetch feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bsi: unexpected status %d", resp.StatusCode)
	}

	var feed rss
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, fmt.Errorf("bsi: parse rss: %w", err)
	}

	advisories := make([]Advisory, 0, len(feed.Channel.Items))
	for _, item := range feed.Channel.Items {
		adv := Advisory{
			BSIID:    extractBSIID(item.GUID, item.Link),
			Title:    strings.TrimSpace(item.Title),
			Summary:  stripHTML(item.Description),
			Severity: mapSeverity(item.Title + " " + item.Description),
			URL:      strings.TrimSpace(item.Link),
			CVEIDs:   uniqueCVEs(cveRegex.FindAllString(item.Description, -1)),
		}
		if t, err := time.Parse(time.RFC1123Z, strings.TrimSpace(item.PubDate)); err == nil {
			adv.PublishedAt = t.UTC()
		} else if t, err := time.Parse(time.RFC1123, strings.TrimSpace(item.PubDate)); err == nil {
			adv.PublishedAt = t.UTC()
		} else {
			adv.PublishedAt = time.Now().UTC()
		}
		advisories = append(advisories, adv)
	}
	return advisories, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// extractBSIID tries to find the "WID-SEC-YYYY-NNNN" style ID from the GUID
// or the link URL.
func extractBSIID(guid, link string) string {
	for _, s := range []string{guid, link} {
		parts := strings.Split(s, "/")
		for i := len(parts) - 1; i >= 0; i-- {
			if strings.HasPrefix(parts[i], "WID-") || strings.HasPrefix(parts[i], "CB-") {
				return strings.TrimSpace(parts[i])
			}
		}
	}
	// Fallback: use the GUID as-is (may be a URL).
	if guid != "" {
		return guid
	}
	return link
}

// mapSeverity guesses a severity level from German severity keywords in text.
func mapSeverity(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "kritisch") || strings.Contains(lower, "critical"):
		return "critical"
	case strings.Contains(lower, "hoch") || strings.Contains(lower, "high"):
		return "high"
	case strings.Contains(lower, "mittel") || strings.Contains(lower, "medium") || strings.Contains(lower, "moderat"):
		return "medium"
	default:
		return "medium"
	}
}

// stripHTML removes HTML tags and collapses whitespace.
func stripHTML(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			b.WriteRune(' ')
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// uniqueCVEs deduplicates a slice of CVE strings preserving order.
func uniqueCVEs(cves []string) []string {
	seen := make(map[string]struct{}, len(cves))
	out := make([]string, 0, len(cves))
	for _, c := range cves {
		if _, ok := seen[c]; !ok {
			seen[c] = struct{}{}
			out = append(out, c)
		}
	}
	return out
}
