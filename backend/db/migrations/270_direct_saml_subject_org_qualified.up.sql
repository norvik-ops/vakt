-- R1-SEC-01: Direct-SAML-Konten aus der Zeit vor 2026-11-09 tragen die rohe
-- NameID mit oidc_provider = 'saml' — dieselbe Form wie Casdoor-SAML-Konten.
-- Ein Casdoor-Login mit gleichem Subject traf sie. Umgeschrieben auf die Form,
-- die der Code jetzt schreibt: 'saml:<org_id>:<NameID>', Provider 'saml_direct'.
-- Als Direct-SAML gilt ein 'saml'-Konto, wenn seine Org Direct-SAML eingerichtet
-- hat oder login_history einen erfolgreichen Direct-SAML-Login nennt. Trifft das
-- ein Casdoor-SAML-Konto, verknüpft der nächste Login es über die vom IdP
-- bestätigte E-Mail neu — kein Aussperren. Umkehrbar: siehe .down.sql.
UPDATE users u
SET oidc_subject  = 'saml:' || src.org_id::text || ':' || u.oidc_subject,
    oidc_provider = 'saml_direct'
FROM (
    SELECT DISTINCT ON (om.user_id) om.user_id, om.org_id
    FROM org_members om
    WHERE EXISTS (SELECT 1 FROM org_saml_configs c WHERE c.org_id = om.org_id)
       OR EXISTS (SELECT 1 FROM login_history lh
                  WHERE lh.user_id = om.user_id AND lh.org_id = om.org_id
                    AND lh.source = 'saml_direct' AND lh.result = 'ok')
    ORDER BY om.user_id, om.joined_at ASC
) src
WHERE u.id = src.user_id
  AND u.oidc_provider = 'saml'
  AND COALESCE(u.oidc_subject, '') <> ''
  AND u.oidc_subject NOT LIKE 'saml:%';
