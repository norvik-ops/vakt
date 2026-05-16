// Package apidocs provides Swagger UI and OpenAPI spec endpoints.
package apidocs

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// ServeSwaggerUI delivers a Swagger UI HTML page that loads the OpenAPI spec.
func ServeSwaggerUI(c echo.Context) error {
	html := `<!DOCTYPE html>
<html>
<head>
  <title>SecHealth API Docs</title>
  <meta charset="utf-8"/>
  <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script>
SwaggerUIBundle({
  url: "/api/v1/openapi.yaml",
  dom_id: '#swagger-ui',
  presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.presets.standalone],
  layout: "BaseLayout"
})
</script>
</body>
</html>`
	return c.HTML(http.StatusOK, html)
}

// ServeOpenAPISpec delivers a minimal OpenAPI 3.0 YAML specification.
func ServeOpenAPISpec(c echo.Context) error {
	spec := generateSpec()
	c.Response().Header().Set("Content-Type", "application/yaml")
	return c.String(http.StatusOK, spec)
}

// generateSpec returns a hardcoded OpenAPI 3.0.3 YAML document covering
// the primary SecHealth API endpoints.
func generateSpec() string {
	return `openapi: 3.0.3
info:
  title: SecHealth API
  version: 1.0.0
  description: |
    Self-hosted Security & Compliance Documentation Platform.
    All endpoints require a Paseto Bearer token unless noted otherwise.

servers:
  - url: /api/v1
    description: Current API version

security:
  - BearerAuth: []

components:
  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer
      bearerFormat: Paseto

  schemas:
    Error:
      type: object
      properties:
        error:
          type: string
        code:
          type: string
        details:
          type: object

    Asset:
      type: object
      properties:
        id:
          type: string
          format: uuid
        org_id:
          type: string
          format: uuid
        name:
          type: string
        type:
          type: string
          enum: [server, container, webapp, repository]
        criticality:
          type: string
          enum: [low, medium, high, critical]
        tags:
          type: array
          items:
            type: string
        external_url:
          type: string
        created_at:
          type: string
          format: date-time
        updated_at:
          type: string
          format: date-time

    Finding:
      type: object
      properties:
        id:
          type: string
          format: uuid
        org_id:
          type: string
          format: uuid
        asset_id:
          type: string
          format: uuid
        cve_id:
          type: string
          nullable: true
        title:
          type: string
        description:
          type: string
        severity:
          type: string
          enum: [info, low, medium, high, critical]
        cvss_score:
          type: number
          format: float
          nullable: true
        status:
          type: string
          enum: [open, in_progress, resolved, accepted_risk, false_positive]
        scanner:
          type: string
        sla_due_at:
          type: string
          format: date-time
          nullable: true
        created_at:
          type: string
          format: date-time

    Framework:
      type: object
      properties:
        id:
          type: string
          format: uuid
        name:
          type: string
        type:
          type: string
        version:
          type: string
        description:
          type: string
        control_count:
          type: integer
        implemented_count:
          type: integer

    DSRRecord:
      type: object
      properties:
        id:
          type: string
          format: uuid
        type:
          type: string
        subject_name:
          type: string
        status:
          type: string
        due_date:
          type: string
          format: date-time
        created_at:
          type: string
          format: date-time

    DashboardScore:
      type: object
      properties:
        overall_score:
          type: number
        nis2_score:
          type: number
        iso27001_score:
          type: number
        open_findings:
          type: integer
        critical_findings:
          type: integer

    SearchResult:
      type: object
      properties:
        type:
          type: string
        id:
          type: string
        title:
          type: string
        url:
          type: string

paths:
  /auth/login:
    post:
      summary: Authenticate and receive a Paseto token
      security: []
      tags: [Auth]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [email, password]
              properties:
                email:
                  type: string
                  format: email
                password:
                  type: string
                  format: password
      responses:
        '200':
          description: Successful login
          content:
            application/json:
              schema:
                type: object
                properties:
                  token:
                    type: string
                  expires_at:
                    type: string
                    format: date-time
        '401':
          description: Invalid credentials
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'

  /auth/register:
    post:
      summary: Register a new organisation and admin account
      security: []
      tags: [Auth]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [email, password, org_name]
              properties:
                email:
                  type: string
                  format: email
                password:
                  type: string
                  format: password
                org_name:
                  type: string
      responses:
        '201':
          description: Organisation and user created
          content:
            application/json:
              schema:
                type: object
                properties:
                  token:
                    type: string
        '409':
          description: Email already registered
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'

  /secpulse/assets:
    get:
      summary: List assets
      tags: [SecPulse]
      parameters:
        - name: page
          in: query
          schema:
            type: integer
            default: 1
        - name: limit
          in: query
          schema:
            type: integer
            default: 25
        - name: tag
          in: query
          schema:
            type: string
      responses:
        '200':
          description: Paginated asset list
          content:
            application/json:
              schema:
                type: object
                properties:
                  data:
                    type: array
                    items:
                      $ref: '#/components/schemas/Asset'
                  total:
                    type: integer
                  page:
                    type: integer
                  limit:
                    type: integer
    post:
      summary: Create an asset
      tags: [SecPulse]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [name, type, criticality]
              properties:
                name:
                  type: string
                type:
                  type: string
                  enum: [server, container, webapp, repository]
                criticality:
                  type: string
                  enum: [low, medium, high, critical]
                tags:
                  type: array
                  items:
                    type: string
                external_url:
                  type: string
      responses:
        '201':
          description: Asset created
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Asset'
        '422':
          description: Validation error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'

  /secpulse/findings:
    get:
      summary: List vulnerability findings
      tags: [SecPulse]
      parameters:
        - name: severity
          in: query
          schema:
            type: string
            enum: [info, low, medium, high, critical]
        - name: status
          in: query
          schema:
            type: string
            enum: [open, in_progress, resolved, accepted_risk, false_positive]
        - name: asset_id
          in: query
          schema:
            type: string
            format: uuid
        - name: page
          in: query
          schema:
            type: integer
            default: 1
        - name: limit
          in: query
          schema:
            type: integer
            default: 25
      responses:
        '200':
          description: List of findings
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/Finding'

  /secpulse/findings/import:
    post:
      summary: Import findings from SARIF, CycloneDX or CSV
      tags: [SecPulse]
      parameters:
        - name: asset_id
          in: query
          required: true
          schema:
            type: string
            format: uuid
        - name: format
          in: query
          required: true
          schema:
            type: string
            enum: [sarif, cyclonedx, csv]
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                file:
                  type: string
                  format: binary
      responses:
        '200':
          description: Import result
          content:
            application/json:
              schema:
                type: object
                properties:
                  imported:
                    type: integer
                  format:
                    type: string
                  asset_id:
                    type: string
        '400':
          description: Bad request or parse error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'

  /secvitals/frameworks:
    get:
      summary: List compliance frameworks (NIS2, ISO 27001, BSI)
      tags: [SecVitals]
      responses:
        '200':
          description: List of frameworks
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/Framework'

  /dashboard/score:
    get:
      summary: Get overall compliance and risk score for the dashboard
      tags: [Dashboard]
      responses:
        '200':
          description: Dashboard score
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/DashboardScore'

  /secprivacy/dsr:
    get:
      summary: List DSGVO data subject requests (Art. 15-22)
      tags: [SecPrivacy]
      parameters:
        - name: status
          in: query
          schema:
            type: string
        - name: page
          in: query
          schema:
            type: integer
            default: 1
        - name: limit
          in: query
          schema:
            type: integer
            default: 25
      responses:
        '200':
          description: List of DSRs
          content:
            application/json:
              schema:
                type: object
                properties:
                  data:
                    type: array
                    items:
                      $ref: '#/components/schemas/DSRRecord'
                  total:
                    type: integer

  /search:
    get:
      summary: Global full-text search across all modules
      tags: [Search]
      parameters:
        - name: q
          in: query
          required: true
          schema:
            type: string
          description: Search query string
        - name: limit
          in: query
          schema:
            type: integer
            default: 10
      responses:
        '200':
          description: Search results
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/SearchResult'
`
}
