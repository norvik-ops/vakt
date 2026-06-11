-- S74-3: Risikobewertung BSI 200-3 — Gefährdungskatalog + Risikoanalyse

CREATE TABLE ck_bsi_threats (
    id          TEXT PRIMARY KEY,
    title       TEXT NOT NULL,
    category    TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT ''
);

INSERT INTO ck_bsi_threats (id, title, category) VALUES
  ('G-0.1',  'Feuer',                                                              'Höhere Gewalt'),
  ('G-0.2',  'Ungünstige klimatische Bedingungen',                                 'Höhere Gewalt'),
  ('G-0.3',  'Wasser',                                                             'Höhere Gewalt'),
  ('G-0.4',  'Verschmutzung, Staub, Korrosion',                                    'Höhere Gewalt'),
  ('G-0.5',  'Naturkatastrophen',                                                  'Höhere Gewalt'),
  ('G-0.6',  'Katastrophen im Umfeld',                                             'Höhere Gewalt'),
  ('G-0.7',  'Großereignisse im Umfeld',                                           'Höhere Gewalt'),
  ('G-0.8',  'Ausfall oder Störung der Stromversorgung',                           'Technisches Versagen'),
  ('G-0.9',  'Ausfall oder Störung von Kommunikationsnetzen',                      'Technisches Versagen'),
  ('G-0.10', 'Ausfall oder Störung von Versorgungsnetzen',                         'Technisches Versagen'),
  ('G-0.11', 'Ausfall oder Störung von Dienstleistern',                            'Technisches Versagen'),
  ('G-0.12', 'Elektromagnetische Störstrahlung',                                   'Technisches Versagen'),
  ('G-0.13', 'Abfangen kompromittierender Abstrahlung',                            'Vorsätzliche Handlungen'),
  ('G-0.14', 'Ausspähen von Informationen',                                        'Vorsätzliche Handlungen'),
  ('G-0.15', 'Abhören',                                                            'Vorsätzliche Handlungen'),
  ('G-0.16', 'Diebstahl von Geräten, Datenträgern oder Dokumenten',               'Vorsätzliche Handlungen'),
  ('G-0.17', 'Verlust von Geräten, Datenträgern oder Dokumenten',                 'Vorsätzliche Handlungen'),
  ('G-0.18', 'Fehlplanung oder fehlende Anpassung',                               'Organisatorische Mängel'),
  ('G-0.19', 'Offenlegung schützenswerter Informationen',                          'Vorsätzliche Handlungen'),
  ('G-0.20', 'Informationen oder Produkte aus unzuverlässiger Quelle',             'Vorsätzliche Handlungen'),
  ('G-0.21', 'Manipulation von Hard- oder Software',                               'Vorsätzliche Handlungen'),
  ('G-0.22', 'Manipulation von Informationen',                                     'Vorsätzliche Handlungen'),
  ('G-0.23', 'Unbefugtes Eindringen in IT-Systeme',                               'Vorsätzliche Handlungen'),
  ('G-0.24', 'Zerstörung von Geräten oder Datenträgern',                          'Vorsätzliche Handlungen'),
  ('G-0.25', 'Ausfall von Geräten oder Systemen',                                 'Technisches Versagen'),
  ('G-0.26', 'Fehlfunktion von Geräten oder Systemen',                            'Technisches Versagen'),
  ('G-0.27', 'Ressourcenmangel',                                                   'Technisches Versagen'),
  ('G-0.28', 'Software-Schwachstellen oder -Fehler',                              'Technisches Versagen'),
  ('G-0.29', 'Verstoß gegen Gesetze oder Regelungen',                             'Organisatorische Mängel'),
  ('G-0.30', 'Unberechtigte Nutzung oder Administration von Geräten und Systemen','Vorsätzliche Handlungen'),
  ('G-0.31', 'Fehlerhafte Nutzung oder Administration von Geräten und Systemen',  'Menschliche Fehlhandlungen'),
  ('G-0.32', 'Missbrauch von Berechtigungen',                                      'Vorsätzliche Handlungen'),
  ('G-0.33', 'Personalausfall',                                                    'Organisatorische Mängel'),
  ('G-0.34', 'Anschlag',                                                           'Vorsätzliche Handlungen'),
  ('G-0.35', 'Nötigung, Erpressung oder Korruption',                              'Vorsätzliche Handlungen'),
  ('G-0.36', 'Identitätsdiebstahl',                                               'Vorsätzliche Handlungen'),
  ('G-0.37', 'Abstreiten von Handlungen',                                         'Vorsätzliche Handlungen'),
  ('G-0.38', 'Missbrauch personenbezogener Daten',                                'Vorsätzliche Handlungen'),
  ('G-0.39', 'Schadprogramme',                                                    'Vorsätzliche Handlungen'),
  ('G-0.40', 'Verhinderung von Diensten (DoS)',                                   'Vorsätzliche Handlungen'),
  ('G-0.41', 'Sabotage',                                                          'Vorsätzliche Handlungen'),
  ('G-0.42', 'Social Engineering',                                                'Vorsätzliche Handlungen'),
  ('G-0.43', 'Einspielen von Nachrichten',                                        'Vorsätzliche Handlungen'),
  ('G-0.44', 'Unbefugtes Eindringen in Räumlichkeiten',                           'Vorsätzliche Handlungen'),
  ('G-0.45', 'Datenverlust',                                                      'Technisches Versagen'),
  ('G-0.46', 'Integritätsverlust schützenswerter Informationen',                  'Technisches Versagen'),
  ('G-0.47', 'Schädliche Seitenkanal-Angriffe',                                  'Vorsätzliche Handlungen');

CREATE TABLE ck_bsi_risk_assessments (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id               UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    target_object_id     UUID NOT NULL REFERENCES ck_bsi_target_objects(id) ON DELETE CASCADE,
    threat_id            TEXT NOT NULL REFERENCES ck_bsi_threats(id),
    eintrittshaeufigkeit TEXT NOT NULL DEFAULT 'selten'
                             CHECK (eintrittshaeufigkeit IN ('selten', 'mittel', 'haeufig', 'sehr_haeufig')),
    schadensauswirkung   TEXT NOT NULL DEFAULT 'begrenzt'
                             CHECK (schadensauswirkung IN ('vernachlaessigbar', 'begrenzt', 'betraechtlich', 'existenzbedrohend')),
    risikokategorie      TEXT GENERATED ALWAYS AS (
                             CASE
                                 WHEN eintrittshaeufigkeit = 'selten'       AND schadensauswirkung = 'vernachlaessigbar' THEN 'gering'
                                 WHEN eintrittshaeufigkeit = 'selten'       AND schadensauswirkung = 'begrenzt'          THEN 'gering'
                                 WHEN eintrittshaeufigkeit = 'selten'       AND schadensauswirkung = 'betraechtlich'     THEN 'mittel'
                                 WHEN eintrittshaeufigkeit = 'selten'       AND schadensauswirkung = 'existenzbedrohend' THEN 'hoch'
                                 WHEN eintrittshaeufigkeit = 'mittel'       AND schadensauswirkung = 'vernachlaessigbar' THEN 'gering'
                                 WHEN eintrittshaeufigkeit = 'mittel'       AND schadensauswirkung = 'begrenzt'          THEN 'mittel'
                                 WHEN eintrittshaeufigkeit = 'mittel'       AND schadensauswirkung = 'betraechtlich'     THEN 'hoch'
                                 WHEN eintrittshaeufigkeit = 'mittel'       AND schadensauswirkung = 'existenzbedrohend' THEN 'sehr_hoch'
                                 WHEN eintrittshaeufigkeit = 'haeufig'      AND schadensauswirkung = 'vernachlaessigbar' THEN 'mittel'
                                 WHEN eintrittshaeufigkeit = 'haeufig'      AND schadensauswirkung = 'begrenzt'          THEN 'hoch'
                                 WHEN eintrittshaeufigkeit = 'haeufig'      AND schadensauswirkung = 'betraechtlich'     THEN 'sehr_hoch'
                                 WHEN eintrittshaeufigkeit = 'haeufig'      AND schadensauswirkung = 'existenzbedrohend' THEN 'sehr_hoch'
                                 WHEN eintrittshaeufigkeit = 'sehr_haeufig' THEN 'sehr_hoch'
                                 ELSE 'mittel'
                             END
                         ) STORED,
    behandlungsoption    TEXT CHECK (behandlungsoption IN ('reduzieren', 'akzeptieren', 'vermeiden', 'transferieren')),
    massnahme            TEXT NOT NULL DEFAULT '',
    verantwortlicher     TEXT NOT NULL DEFAULT '',
    zieldatum            DATE,
    restrisiko           TEXT CHECK (restrisiko IN ('gering', 'mittel', 'hoch', 'sehr_hoch')),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, target_object_id, threat_id)
);

CREATE INDEX idx_bsi_risk_org    ON ck_bsi_risk_assessments(org_id);
CREATE INDEX idx_bsi_risk_target ON ck_bsi_risk_assessments(target_object_id);
