package config

import "testing"

// TestValidateAIBaseURL deckt die SSRF-Guard-Faelle aus S13-1 ab.
func TestValidateAIBaseURL(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		wantErr bool
	}{
		// Erlaubte Service-Discovery-Hostnames.
		{"ollama default port path", "http://ollama:11434/v1", false},
		{"ollama https", "https://ollama:11434/v1", false},
		{"ai-llm service", "http://ai-llm:8000/v1", false},
		{"llm-proxy service", "http://llm-proxy/v1", false},
		{"lm-studio service", "http://lm-studio:1234/v1", false},

		// Public DNS — durchgelassen.
		{"openai public", "https://api.openai.com/v1", false},
		{"mistral public", "https://api.mistral.ai/v1", false},
		{"groq public", "https://api.groq.com/openai/v1", false},

		// Blockliste: Cloud-Metadata IMDS.
		{"AWS IMDS v4", "http://169.254.169.254/latest/meta-data/", true},
		{"AWS IMDS v4 mit Port", "http://169.254.169.254:80/v1", true},

		// Blockliste: Loopback.
		{"IPv4 loopback", "http://127.0.0.1/v1", true},
		{"IPv4 loopback in 127/8", "http://127.0.0.99/v1", true},
		{"IPv6 loopback", "http://[::1]/v1", true},

		// Blockliste: Link-Local.
		{"IPv4 link-local", "http://169.254.10.10/v1", true},
		{"IPv6 link-local", "http://[fe80::1]/v1", true},

		// Blockliste: localhost als Name.
		{"localhost hostname", "http://localhost/v1", true},
		{"LOCALHOST groß", "http://LOCALHOST:8080/v1", true},

		// Schema-Fehler.
		{"file:// schema", "file:///etc/passwd", true},
		{"ftp:// schema", "ftp://ollama/v1", true},
		{"leer", "", true},
		{"kaputter URL", "://nope", true},
		{"ohne host", "http:///v1", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateAIBaseURL(c.url)
			if c.wantErr && err == nil {
				t.Errorf("expected error for %q, got nil", c.url)
			}
			if !c.wantErr && err != nil {
				t.Errorf("unexpected error for %q: %v", c.url, err)
			}
		})
	}
}
