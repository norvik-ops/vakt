# SAML Metadata Templates

Example SP (Service Provider) metadata XMLs and IdP-side configuration snippets
for common identity providers. Use these as a starting point when configuring
Vakt's SAML 2.0 integration.

## How to use

1. In Vakt: **Settings → SAML** → Copy your ACS URL and Entity ID.
2. In your IdP: Create a new SAML application, paste the ACS URL + Entity ID.
3. Download your IdP's metadata XML and upload it in Vakt's SAML settings.
4. Click **Test Connection** to verify the roundtrip.

## Templates

| IdP | File |
|-----|------|
| Microsoft Azure Entra ID (formerly AzureAD) | `azure-entra-id.xml` |
| Okta | `okta.xml` |
| OneLogin | `onelogin.xml` |
| Google Workspace | `google-workspace.xml` |

## ACS URL Pattern

Your Vakt instance's ACS URL follows this pattern:

```
https://<your-vakt-domain>/api/v1/auth/saml/acs
```

The Entity ID (SP EntityID) is:

```
https://<your-vakt-domain>/api/v1/auth/saml/metadata
```
