-- Zurueck auf das enge Vokabular.
--
-- Offene Einladungen, die einen Plattformnamen tragen, wuerden den alten CHECK
-- verletzen. Sie werden vorher auf ihren Cache-Wert gezogen — dieselbe
-- Abbildung, die usermgmt.assignableRoles verwendet. AuditorReadOnly und
-- InternalAuditor verlieren dabei ihre Unterscheidung und werden zu 'viewer':
-- das alte Vokabular kann sie nicht ausdruecken. Das ist der Grund, warum
-- dieser Rueckbau Information kostet, und er steht hier, damit es niemanden
-- ueberrascht.
UPDATE user_invitations SET role = 'admin'  WHERE role = 'Admin';
UPDATE user_invitations SET role = 'editor' WHERE role = 'SecurityAnalyst';
UPDATE user_invitations SET role = 'viewer' WHERE role IN ('Viewer', 'AuditorReadOnly', 'InternalAuditor');

ALTER TABLE user_invitations DROP CONSTRAINT IF EXISTS user_invitations_role_check;

ALTER TABLE user_invitations ADD CONSTRAINT user_invitations_role_check
    CHECK (role IN ('admin', 'editor', 'viewer'));
