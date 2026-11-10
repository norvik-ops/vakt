-- Stellt die alte Form her: rohe NameID, Provider 'saml'. 'saml:' (5) + UUID (36)
-- + ':' = 42 Zeichen Präfix. Gilt auch für Konten, die der neue Code angelegt
-- hat — der alte Code kennt nur die rohe Form.
UPDATE users
SET oidc_subject  = substr(oidc_subject, 43),
    oidc_provider = 'saml'
WHERE oidc_provider = 'saml_direct'
  AND oidc_subject ~ '^saml:[0-9a-f-]{36}:';
