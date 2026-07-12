// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package admin

import (
	"embed"
	"fmt"
	"html/template"
	"io"

	"github.com/labstack/echo/v4"
)

//go:embed templates/*.html
var files embed.FS

// Renderer renders one page at a time.
//
// Each page gets its OWN template set, because layout.html calls {{template
// "content" .}} and both pages define "content" — parsed into a single set, the
// second definition would silently win and every page would render the same body.
type Renderer struct {
	pages map[string]*template.Template
}

func NewRenderer() (*Renderer, error) {
	r := &Renderer{pages: map[string]*template.Template{}}
	for _, page := range []string{"dashboard.html", "subscriptions.html", "invoices.html", "licences.html", "lexware.html", "subscription.html", "new.html"} {
		t, err := template.New(page).ParseFS(files, "templates/layout.html", "templates/"+page)
		if err != nil {
			return nil, fmt.Errorf("billing admin: parse %s: %w", page, err)
		}
		r.pages[page] = t
	}
	return r, nil
}

func (r *Renderer) Render(w io.Writer, name string, data any, _ echo.Context) error {
	t, ok := r.pages[name]
	if !ok {
		return fmt.Errorf("billing admin: no such page %q", name)
	}
	return t.ExecuteTemplate(w, "layout", data)
}
