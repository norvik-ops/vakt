{{/*
S13-8 Install-Gate: lehnt Honeypot-Defaults ab.
Wird in jedem Template referenziert, das DB- oder Redis-Credentials nutzt.
*/}}
{{- define "vakt.validateSecrets" -}}
{{- if .Values.postgresql.enabled -}}
{{- if eq .Values.postgresql.auth.password "MUST_BE_OVERRIDDEN" -}}
{{- fail "postgresql.auth.password is the honeypot default 'MUST_BE_OVERRIDDEN'. Set it explicitly:\n\n  helm install vakt ./helm/vakt --set postgresql.auth.password=$(openssl rand -hex 32)\n\nor reference an existing secret via postgresql.auth.existingSecret." -}}
{{- end -}}
{{- if lt (len .Values.postgresql.auth.password) 16 -}}
{{- fail "postgresql.auth.password must be at least 16 characters long. Generate one with: openssl rand -hex 32" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Tag eines EIGENEN Images (ghcr.io/norvik-ops/*) -> der Tag, den es in der Registry
wirklich gibt.

R1-05-C02, die v0.42.42-Klasse zurueck in derselben Datei: values.yaml pinnte
`tag: "0.42.49"`, GHCR fuehrt 92 Tags und ALLE tragen ein `v`
(manifests/0.42.49 = 404, manifests/v0.42.49 = 200). Auf dem Default-Pfad ging
damit jede Installation in ImagePullBackOff — api, worker, frontend und der
migrate-Job. Wer `--set image.api.tag=v0.42.49` setzte, war nicht betroffen.

Warum die Normalisierung HIER sitzt und nicht in values.yaml: dieselbe v-lose
Zahl ist auch `Chart.version` und `appVersion`, und .github/workflows/release.yml
prueft vor jedem Release, dass values.yaml genau `${GITHUB_REF_NAME#v}` traegt —
also ohne v. Ein v in values.yaml wuerde dieses Release-Gate roeten. Gebaut wird
der Tag aber mit `${{ github.ref_name }}`, also MIT v. Die beiden Formen treffen
sich hier.

Nur eine nackte Semver-Zahl bekommt das v vorangestellt. `v0.42.49`, `latest`,
`main-abc1234` und `@sha256:`-Digests bleiben unveraendert — sonst waere aus
`--set image.api.tag=latest` ein `vlatest` geworden, ein neuer Bruch an der
Stelle, die dieser Helper reparieren soll.

UND NUR FUER ghcr.io/norvik-ops/. Bis zum 2026-08-02 stand diese Einschraenkung
nur in diesem Kommentar, nicht im Code: der Helper bekam allein den Tag und sah
das Repository nie. Wer das Chart gegen eine eigene Registry fuhr, bekam den
v-Tag mitgepraegt —

    --set image.api.repository=registry.intern/vakt-api --set image.api.tag=0.42.49
      ->  registry.intern/vakt-api:v0.42.49        (gemessen)

— und damit ImagePullBackOff aus demselben Grund, den dieser Helper beheben
soll, nur fuer die Gruppe, die vorher funktionierte. Einen Ausschalter gab es
nicht. Deshalb nimmt er jetzt beides und praeft die Herkunft selbst.

Argument: dict "repository" <repo> "tag" <tag>.
*/}}
{{- define "vakt.ownImageTag" -}}
{{- $repo := .repository | toString -}}
{{- $t := .tag | toString -}}
{{- if and (hasPrefix "ghcr.io/norvik-ops/" $repo) (regexMatch "^[0-9]+\\.[0-9]+\\.[0-9]+" $t) -}}v{{ $t }}{{- else -}}{{ $t }}{{- end -}}
{{- end -}}

{{/*
Expand the name of the chart.
*/}}
{{- define "vakt.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "vakt.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart label.
*/}}
{{- define "vakt.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "vakt.labels" -}}
helm.sh/chart: {{ include "vakt.chart" . }}
{{ include "vakt.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "vakt.selectorLabels" -}}
app.kubernetes.io/name: {{ include "vakt.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
PostgreSQL DSN — uses subchart service name when enabled, else value from secrets.dbUrl
*/}}
{{- define "vakt.dbUrl" -}}
{{- if .Values.postgresql.enabled -}}
{{- printf "postgres://%s:%s@%s-postgresql:5432/%s" .Values.postgresql.auth.username .Values.postgresql.auth.password .Release.Name .Values.postgresql.auth.database -}}
{{- else -}}
{{- .Values.secrets.dbUrl -}}
{{- end }}
{{- end }}

{{/*
Redis URL — uses subchart service name when enabled, else value from secrets.redisUrl
*/}}
{{- define "vakt.redisUrl" -}}
{{- if .Values.redis.enabled -}}
{{- printf "redis://%s-redis-master:6379" .Release.Name -}}
{{- else -}}
{{- .Values.secrets.redisUrl -}}
{{- end }}
{{- end }}
