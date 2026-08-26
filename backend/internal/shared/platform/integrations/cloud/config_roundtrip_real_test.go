//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import (
	"context"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	sharedcrypto "github.com/matharnica/vakt/internal/shared/crypto"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// R1-B0-N1 — Speichern-und-wieder-lesen für JEDE Cloud-Integration.
//
// Der Defekt: f08884f5 (2026-07-11) hat den Schreibpfad von fünf Providern um
// ein BOOL erweitert ("allow_private_target"), aber vier der zugehörigen
// Lesepfade parsen das JSONB weiterhin in ein map[string]string. Ein einziger
// Nicht-String im Dokument lässt json.Unmarshal komplett fehlschlagen — die
// gesamte Konfiguration inklusive der verschlüsselten Zugangsdaten ist danach
// unlesbar, obwohl das Speichern "erfolgreich" gemeldet hat.
//
// Der Test fährt bewusst ALLE 13 Provider ab und nicht nur die vier gemeldeten:
// die Fehlerklasse ist ein Typ-Mismatch zwischen zwei Codestellen, und welche
// Paare betroffen sind, entscheidet man nicht durch Hinsehen, sondern durch
// Abfahren. Der Nenner wird am Ende ausgegeben.
//
// Ein Test, der nur prüft "Speichern gab keinen Fehler zurück", wäre zu
// schwach — genau so ist der Defekt entstanden. Deshalb vergleicht jeder Fall
// den ENTSCHLÜSSELTEN Wert nach einem echten Neu-Laden aus Postgres.

const (
	// Private Ziele: die fünf URL-Provider werden bewusst mit einem RFC1918-Ziel
	// plus AllowPrivateTarget=true angelegt. Das deckt den On-Premises-Fall ab,
	// für den das Flag überhaupt eingeführt wurde, und braucht kein DNS
	// (ValidateOutboundURL löst IP-Literale ohne Netzzugriff auf).
	testPrivateHost = "10.99.0.7"
)

type providerCase struct {
	name string
	// save legt die Konfiguration mit einem echten Geheimnis an.
	save func(ctx context.Context, s *Service, orgID string) error
	// readBack lädt sie neu und gibt das entschlüsselte Geheimnis zurück.
	readBack func(ctx context.Context, s *Service, orgID string) (secret string, err error)
	// wantSecret ist der Klartext, der nach dem Neu-Laden herauskommen MUSS.
	wantSecret string
	// isConfigured spiegelt, was die maskierte GET-Antwort dem Frontend sagt.
	isConfigured func(ctx context.Context, s *Service, orgID string) (bool, error)
	// readAllowPrivate ist nur für die fünf Provider mit dem SSRF-Opt-in gesetzt.
	// Nil heißt: dieser Provider kennt das Flag nicht.
	readAllowPrivate func(ctx context.Context, s *Service, orgID string) (bool, error)
}

func cloudProviderCases() []providerCase {
	return []providerCase{
		{
			name: "aws",
			save: func(ctx context.Context, s *Service, org string) error {
				return s.SaveAWSConfig(ctx, org, SaveAWSConfigInput{
					AccessKeyID: "AKIAEXAMPLE", SecretAccessKey: "aws-secret-1", Region: "eu-central-1",
				})
			},
			readBack: func(ctx context.Context, s *Service, org string) (string, error) {
				c, err := s.getDecryptedAWSConfig(ctx, org)
				return secretOf(err, c, func() string { return c.SecretAccessKey })
			},
			wantSecret: "aws-secret-1",
			isConfigured: func(ctx context.Context, s *Service, org string) (bool, error) {
				r, err := s.GetAWSConfig(ctx, org)
				return configuredOf(err, r, func() bool { return r.IsConfigured })
			},
		},
		{
			name: "azure",
			save: func(ctx context.Context, s *Service, org string) error {
				return s.SaveAzureConfig(ctx, org, SaveAzureConfigInput{
					TenantID: "t", ClientID: "c", ClientSecret: "azure-secret-2", SubscriptionID: "s",
				})
			},
			readBack: func(ctx context.Context, s *Service, org string) (string, error) {
				c, err := s.getDecryptedAzureConfig(ctx, org)
				return secretOf(err, c, func() string { return c.ClientSecret })
			},
			wantSecret: "azure-secret-2",
			isConfigured: func(ctx context.Context, s *Service, org string) (bool, error) {
				r, err := s.GetAzureConfig(ctx, org)
				return configuredOf(err, r, func() bool { return r.IsConfigured })
			},
		},
		{
			name: "hetzner",
			save: func(ctx context.Context, s *Service, org string) error {
				return s.SaveHetznerConfig(ctx, org, SaveHetznerConfigInput{APIToken: "hetzner-token-3", Location: "fsn1"})
			},
			readBack: func(ctx context.Context, s *Service, org string) (string, error) {
				c, err := s.getDecryptedHetznerConfig(ctx, org)
				return secretOf(err, c, func() string { return c.APIToken })
			},
			wantSecret: "hetzner-token-3",
			isConfigured: func(ctx context.Context, s *Service, org string) (bool, error) {
				r, err := s.GetHetznerConfig(ctx, org)
				return configuredOf(err, r, func() bool { return r.IsConfigured })
			},
		},
		{
			name: "ionos",
			save: func(ctx context.Context, s *Service, org string) error {
				return s.SaveIONOSConfig(ctx, org, SaveIONOSConfigInput{Username: "u", Password: "ionos-pw-4", Token: "ionos-token-4"})
			},
			readBack: func(ctx context.Context, s *Service, org string) (string, error) {
				c, err := s.getDecryptedIONOSConfig(ctx, org)
				return secretOf(err, c, func() string { return c.Password })
			},
			wantSecret: "ionos-pw-4",
			isConfigured: func(ctx context.Context, s *Service, org string) (bool, error) {
				r, err := s.GetIONOSConfig(ctx, org)
				return configuredOf(err, r, func() bool { return r.IsConfigured })
			},
		},
		{
			name: "wazuh",
			save: func(ctx context.Context, s *Service, org string) error {
				return s.SaveWazuhConfig(ctx, org, SaveWazuhConfigInput{
					BaseURL: "https://" + testPrivateHost + ":55000", Username: "wu",
					Password: "wazuh-pw-5", VerifyTLS: true, AllowPrivateTarget: true,
				})
			},
			readBack: func(ctx context.Context, s *Service, org string) (string, error) {
				c, err := s.getDecryptedWazuhConfig(ctx, org)
				return secretOf(err, c, func() string { return c.Password })
			},
			wantSecret: "wazuh-pw-5",
			isConfigured: func(ctx context.Context, s *Service, org string) (bool, error) {
				r, err := s.GetWazuhConfig(ctx, org)
				return configuredOf(err, r, func() bool { return r.IsConfigured })
			},
			readAllowPrivate: func(ctx context.Context, s *Service, org string) (bool, error) {
				c, err := s.getDecryptedWazuhConfig(ctx, org)
				if err != nil || c == nil {
					return false, err
				}
				return c.AllowPrivateTarget, nil
			},
		},
		{
			name: "prometheus",
			save: func(ctx context.Context, s *Service, org string) error {
				return s.SavePrometheusConfig(ctx, org, SavePrometheusConfigInput{
					PrometheusURL: "http://" + testPrivateHost + ":9090", Token: "prom-token-6", AllowPrivateTarget: true,
				})
			},
			readBack: func(ctx context.Context, s *Service, org string) (string, error) {
				c, err := s.getDecryptedPrometheusConfig(ctx, org)
				return secretOf(err, c, func() string { return c.Token })
			},
			wantSecret: "prom-token-6",
			isConfigured: func(ctx context.Context, s *Service, org string) (bool, error) {
				r, err := s.GetPrometheusConfig(ctx, org)
				return configuredOf(err, r, func() bool { return r.IsConfigured })
			},
			readAllowPrivate: func(ctx context.Context, s *Service, org string) (bool, error) {
				c, err := s.getDecryptedPrometheusConfig(ctx, org)
				if err != nil || c == nil {
					return false, err
				}
				return c.AllowPrivateTarget, nil
			},
		},
		{
			name: "entra_id",
			save: func(ctx context.Context, s *Service, org string) error {
				return s.SaveEntraIDConfig(ctx, org, SaveEntraIDConfigInput{TenantID: "t", ClientID: "c", ClientSecret: "entra-secret-7"})
			},
			readBack: func(ctx context.Context, s *Service, org string) (string, error) {
				c, err := s.getDecryptedEntraIDConfig(ctx, org)
				return secretOf(err, c, func() string { return c.ClientSecret })
			},
			wantSecret: "entra-secret-7",
			isConfigured: func(ctx context.Context, s *Service, org string) (bool, error) {
				r, err := s.GetEntraIDConfig(ctx, org)
				return configuredOf(err, r, func() bool { return r.IsConfigured })
			},
		},
		{
			name: "intune",
			save: func(ctx context.Context, s *Service, org string) error {
				return s.SaveIntuneConfig(ctx, org, SaveIntuneConfigInput{TenantID: "t", ClientID: "c", ClientSecret: "intune-secret-8"})
			},
			readBack: func(ctx context.Context, s *Service, org string) (string, error) {
				c, err := s.getDecryptedIntuneConfig(ctx, org)
				return secretOf(err, c, func() string { return c.ClientSecret })
			},
			wantSecret: "intune-secret-8",
			isConfigured: func(ctx context.Context, s *Service, org string) (bool, error) {
				r, err := s.GetIntuneConfig(ctx, org)
				return configuredOf(err, r, func() bool { return r.IsConfigured })
			},
		},
		{
			name: "keycloak",
			save: func(ctx context.Context, s *Service, org string) error {
				return s.SaveKeycloakConfig(ctx, org, SaveKeycloakConfigInput{
					KeycloakURL: "https://" + testPrivateHost + ":8443", Realm: "master",
					ClientID: "vakt", ClientSecret: "keycloak-secret-9", AllowPrivateTarget: true,
				})
			},
			readBack: func(ctx context.Context, s *Service, org string) (string, error) {
				c, err := s.getDecryptedKeycloakConfig(ctx, org)
				return secretOf(err, c, func() string { return c.ClientSecret })
			},
			wantSecret: "keycloak-secret-9",
			isConfigured: func(ctx context.Context, s *Service, org string) (bool, error) {
				r, err := s.GetKeycloakConfig(ctx, org)
				return configuredOf(err, r, func() bool { return r.IsConfigured })
			},
			readAllowPrivate: func(ctx context.Context, s *Service, org string) (bool, error) {
				c, err := s.getDecryptedKeycloakConfig(ctx, org)
				if err != nil || c == nil {
					return false, err
				}
				return c.AllowPrivateTarget, nil
			},
		},
		{
			name: "ldap",
			save: func(ctx context.Context, s *Service, org string) error {
				return s.SaveLDAPConfig(ctx, org, SaveLDAPConfigInput{
					Host: "ldap.internal", Port: 636, BindDN: "cn=svc", BindPassword: "ldap-pw-10",
					BaseDN: "dc=example,dc=org", UseTLS: true, IsActiveDirectory: true,
				})
			},
			readBack: func(ctx context.Context, s *Service, org string) (string, error) {
				c, err := s.getDecryptedLDAPConfig(ctx, org)
				return secretOf(err, c, func() string { return c.BindPassword })
			},
			wantSecret: "ldap-pw-10",
			isConfigured: func(ctx context.Context, s *Service, org string) (bool, error) {
				r, err := s.GetLDAPConfig(ctx, org)
				return configuredOf(err, r, func() bool { return r.IsConfigured })
			},
		},
		{
			name: "gitlab",
			save: func(ctx context.Context, s *Service, org string) error {
				return s.SaveGitLabConfig(ctx, org, SaveGitLabConfigInput{
					GitLabURL: "https://" + testPrivateHost, AccessToken: "gitlab-token-11",
					GroupID: "42", AllowPrivateTarget: true,
				})
			},
			readBack: func(ctx context.Context, s *Service, org string) (string, error) {
				c, err := s.getDecryptedGitLabConfig(ctx, org)
				return secretOf(err, c, func() string { return c.AccessToken })
			},
			wantSecret: "gitlab-token-11",
			isConfigured: func(ctx context.Context, s *Service, org string) (bool, error) {
				r, err := s.GetGitLabConfig(ctx, org)
				return configuredOf(err, r, func() bool { return r.IsConfigured })
			},
			readAllowPrivate: func(ctx context.Context, s *Service, org string) (bool, error) {
				c, err := s.getDecryptedGitLabConfig(ctx, org)
				if err != nil || c == nil {
					return false, err
				}
				return c.AllowPrivateTarget, nil
			},
		},
		{
			name: "sonarqube",
			save: func(ctx context.Context, s *Service, org string) error {
				return s.SaveSonarQubeConfig(ctx, org, SaveSonarQubeConfigInput{
					BaseURL: "https://" + testPrivateHost + ":9000", Token: "sonar-token-12", AllowPrivateTarget: true,
				})
			},
			readBack: func(ctx context.Context, s *Service, org string) (string, error) {
				c, err := s.getDecryptedSonarQubeConfig(ctx, org)
				return secretOf(err, c, func() string { return c.Token })
			},
			wantSecret: "sonar-token-12",
			isConfigured: func(ctx context.Context, s *Service, org string) (bool, error) {
				r, err := s.GetSonarQubeConfig(ctx, org)
				return configuredOf(err, r, func() bool { return r.IsConfigured })
			},
			readAllowPrivate: func(ctx context.Context, s *Service, org string) (bool, error) {
				c, err := s.getDecryptedSonarQubeConfig(ctx, org)
				if err != nil || c == nil {
					return false, err
				}
				return c.AllowPrivateTarget, nil
			},
		},
		{
			name: "personio",
			save: func(ctx context.Context, s *Service, org string) error {
				return s.SavePersonioConfig(ctx, org, SavePersonioConfigInput{WebhookSecret: "personio-secret-13"})
			},
			readBack: func(ctx context.Context, s *Service, org string) (string, error) {
				return s.GetDecryptedPersonioSecret(ctx, org)
			},
			wantSecret: "personio-secret-13",
			isConfigured: func(ctx context.Context, s *Service, org string) (bool, error) {
				r, err := s.GetPersonioConfig(ctx, org)
				return configuredOf(err, r, func() bool { return r.IsConfigured })
			},
		},
	}
}

// TestCloudConfig_SaveThenReadBack ist die Abnahme: für jeden Provider muss ein
// frisch angelegter Datensatz nach einem echten Neu-Laden aus Postgres wieder
// entschlüsselbar sein, und die maskierte GET-Antwort muss "konfiguriert" melden.
func TestCloudConfig_SaveThenReadBack(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pool := cloudTestPool(ctx, t)
	svc := NewService(pool, cloudTestMasterKey(t), nil)
	orgID := seedCloudOrg(ctx, t, pool, "roundtrip")

	cases := cloudProviderCases()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, tc.save(ctx, svc, orgID), "speichern")

			got, err := tc.readBack(ctx, svc, orgID)
			require.NoError(t, err,
				"neu laden nach dem Speichern — genau hier bricht der Typ-Mismatch (%s)", tc.name)
			assert.Equal(t, tc.wantSecret, got, "entschluesseltes Geheimnis nach dem Neu-Laden")

			configured, err := tc.isConfigured(ctx, svc, orgID)
			require.NoError(t, err, "maskierte GET-Antwort")
			assert.True(t, configured,
				"GET meldet 'nicht konfiguriert', obwohl gerade gespeichert wurde — die Integration sieht fuer den Kunden leer aus")

			if tc.readAllowPrivate != nil {
				allow, err := tc.readAllowPrivate(ctx, svc, orgID)
				require.NoError(t, err, "allow_private_target neu laden")
				assert.True(t, allow,
					"das persistierte SSRF-Opt-in kommt beim Lesen nicht an — der On-Premises-Collector wird beim Verbinden abgewiesen")
			}
		})
	}

	// Nenner ausweisen: ein Test, der nur eine Teilmenge abfährt, meldet Erfolg
	// für Arbeit, die er nicht getan hat.
	t.Logf("geprueft: %d von %d Provider-Speicher-/Lese-Paaren im Paket", len(cases), countUpsertProviders(t))
	require.Equal(t, countUpsertProviders(t), len(cases),
		"jeder Provider mit einem UpsertConfig-Aufruf braucht hier einen Fall")
}

// TestCloudConfig_LegacyRowsRemainReadable belegt, dass der Fix bestehende
// Datensätze nicht entwertet: eine Zeile im alten Format (vor f08884f5, also
// ausschließlich String-Werte und ohne allow_private_target) muss weiterhin
// lesbar sein, mit AllowPrivateTarget=false als ehrlichem Default.
func TestCloudConfig_LegacyRowsRemainReadable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pool := cloudTestPool(ctx, t)
	key := cloudTestMasterKey(t)
	svc := NewService(pool, key, nil)
	orgID := seedCloudOrg(ctx, t, pool, "legacy")

	encrypt := func(plain string) string {
		ct, err := sharedcrypto.Encrypt(key, []byte(plain))
		require.NoError(t, err)
		return hex.EncodeToString(ct)
	}

	// Exakt die Form, die vor f08884f5 geschrieben wurde: nur Strings.
	legacy := map[string]struct {
		provider string
		config   map[string]any
		read     func() (string, error)
		want     string
	}{
		"prometheus": {
			provider: ProviderPrometheus,
			config:   map[string]any{"prometheus_url": "http://prom.old:9090", "alertmanager_url": "", "token": encrypt("legacy-prom")},
			read: func() (string, error) {
				c, err := svc.getDecryptedPrometheusConfig(ctx, orgID)
				return secretOf(err, c, func() string { return c.Token })
			},
			want: "legacy-prom",
		},
		"keycloak": {
			provider: ProviderKeycloak,
			config:   map[string]any{"keycloak_url": "https://kc.old", "realm": "master", "client_id": "vakt", "client_secret": encrypt("legacy-kc")},
			read: func() (string, error) {
				c, err := svc.getDecryptedKeycloakConfig(ctx, orgID)
				return secretOf(err, c, func() string { return c.ClientSecret })
			},
			want: "legacy-kc",
		},
		"gitlab": {
			provider: ProviderGitLab,
			config:   map[string]any{"gitlab_url": "https://gl.old", "access_token": encrypt("legacy-gl"), "group_id": "7"},
			read: func() (string, error) {
				c, err := svc.getDecryptedGitLabConfig(ctx, orgID)
				return secretOf(err, c, func() string { return c.AccessToken })
			},
			want: "legacy-gl",
		},
		"sonarqube": {
			provider: ProviderSonarQube,
			config:   map[string]any{"base_url": "https://sq.old", "token": encrypt("legacy-sq")},
			read: func() (string, error) {
				c, err := svc.getDecryptedSonarQubeConfig(ctx, orgID)
				return secretOf(err, c, func() string { return c.Token })
			},
			want: "legacy-sq",
		},
	}

	repo := NewRepository(pool)
	for name, lc := range legacy {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, repo.UpsertConfig(ctx, orgID, lc.provider, lc.config))
			got, err := lc.read()
			require.NoError(t, err, "Altdatensatz muss ohne Migration lesbar bleiben")
			assert.Equal(t, lc.want, got)
		})
	}
}

// TestCloudConfig_MaskedSecretWithoutExistingRowIsRefused hält fest, dass der
// Maskierungs-Platzhalter "****" niemals als Geheimnis verschlüsselt in der
// Datenbank landet. Ohne diesen Guard schreibt ein Speichern, bei dem die
// bestehende Konfiguration nicht geladen werden konnte, den Platzhalter als
// echten Wert — die Integration meldet dann "konfiguriert" und authentifiziert
// sich mit dem String "****".
func TestCloudConfig_MaskedSecretWithoutExistingRowIsRefused(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pool := cloudTestPool(ctx, t)
	svc := NewService(pool, cloudTestMasterKey(t), nil)
	orgID := seedCloudOrg(ctx, t, pool, "masked")

	err := svc.SavePrometheusConfig(ctx, orgID, SavePrometheusConfigInput{
		PrometheusURL: "http://" + testPrivateHost + ":9090", Token: "****", AllowPrivateTarget: true,
	})
	require.Error(t, err, "der Platzhalter darf ohne bestehendes Geheimnis nicht akzeptiert werden")
	assert.Contains(t, err.Error(), "erneut", "die Meldung muss dem Admin sagen, was zu tun ist")

	// Und es darf keine halbfertige Zeile zurückbleiben.
	resp, getErr := svc.GetPrometheusConfig(ctx, orgID)
	require.NoError(t, getErr)
	assert.False(t, resp.IsConfigured, "abgelehntes Speichern darf keine Konfiguration hinterlassen")
}

// --- Hilfsmittel ---

func secretOf[T any](err error, cfg *T, get func() string) (string, error) {
	if err != nil {
		return "", err
	}
	if cfg == nil {
		return "", nil
	}
	return get(), nil
}

func configuredOf[T any](err error, resp *T, get func() bool) (bool, error) {
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, nil
	}
	return get(), nil
}

// countUpsertProviders zählt die UpsertConfig-Aufrufe in service.go. Damit
// bringt der Test seinen eigenen Nenner mit: kommt ein Provider dazu, ohne dass
// jemand hier einen Fall ergänzt, wird der Test rot statt still unvollständig.
func countUpsertProviders(t *testing.T) int {
	t.Helper()
	src, err := os.ReadFile("service.go")
	require.NoError(t, err)
	return strings.Count(string(src), "s.repo.UpsertConfig(ctx,")
}

func cloudTestMasterKey(t *testing.T) []byte {
	t.Helper()
	key, err := hex.DecodeString("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	require.NoError(t, err)
	return key
}

func seedCloudOrg(ctx context.Context, t *testing.T, pool *pgxpool.Pool, slug string) string {
	t.Helper()
	var orgID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ($1, $2)
		RETURNING id::text`, "CloudRT "+slug, "cloud-rt-"+slug+"-"+time.Now().Format("150405.000000")).Scan(&orgID))
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM cloud_integrations WHERE org_id = $1::uuid`, orgID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1::uuid`, orgID)
	})
	return orgID
}

// cloudTestPool nutzt eine bereitgestellte Wegwerf-Datenbank (VAKT_TEST_DB_DSN)
// und fährt sonst einen eigenen Postgres-Container hoch — wie die übrigen
// *_real_test.go. Kein stiller Skip: ohne Datenbank scheitert der Test.
func cloudTestPool(ctx context.Context, t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("VAKT_TEST_DB_DSN")
	if dsn == "" {
		pgC, err := postgres.Run(ctx,
			"postgres:16.14-alpine@sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777",
			postgres.WithDatabase("vakt_test"),
			postgres.WithUsername("vakt"),
			postgres.WithPassword("vakt"),
			postgres.WithSQLDriver("pgx"),
			postgres.BasicWaitStrategies(),
		)
		require.NoError(t, err, "postgres container")
		t.Cleanup(func() { _ = pgC.Terminate(context.Background()) })

		dsn, err = pgC.ConnectionString(ctx, "sslmode=disable")
		require.NoError(t, err)
		require.NoError(t, shareddb.RunMigrations(dsn, cloudMigrationsDir(t)))
	}

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx), "Testdatenbank nicht erreichbar: %s", dsn)
	t.Cleanup(pool.Close)
	return pool
}

func cloudMigrationsDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "..", "db", "migrations"))
	require.NoError(t, err)
	_, statErr := os.Stat(dir)
	require.NoError(t, statErr, "Migrationsverzeichnis nicht gefunden: %s", dir)
	return dir
}
