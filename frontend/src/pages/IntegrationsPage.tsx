import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Plug, GitBranch, Cloud, ShieldAlert } from 'lucide-react'
import { GitHubTab } from '../modules/vaktcomply/integrations/GitHubTab'
import {
  AWSTab,
  AzureTab,
  HetznerTab,
  IONOSTab,
  WazuhTab,
  PrometheusTab,
} from '../modules/vaktcomply/integrations/CloudProviderTabs'
import {
  EntraIDTab,
  IntuneTab,
  KeycloakTab,
  LDAPTab,
} from '../modules/vaktcomply/integrations/IdentityTabs'
import {
  GitLabTab,
  SonarQubeTab,
} from '../modules/vaktcomply/integrations/DevOpsTabs'
import { PersonioTab } from '../modules/vaktcomply/integrations/PersonioTab'

// --- No third-party integrations info box ---

function NoThirdPartyInfoBox() {
  const { t } = useTranslation()
  return (
    <div className="flex items-start gap-4 p-5 rounded-xl border border-border bg-surface max-w-lg">
      <ShieldAlert className="w-6 h-6 text-amber-500 shrink-0 mt-0.5" />
      <div>
        <p className="text-sm font-semibold text-primary mb-1">{t('integrations.page.noThirdPartyTitle')}</p>
        <p className="text-xs text-secondary leading-relaxed">
          {t('integrations.page.noThirdPartyDesc')}
        </p>
      </div>
    </div>
  )
}

// --- Main page ---

type Tab = 'github' | 'aws' | 'azure' | 'hetzner' | 'ionos' | 'wazuh' | 'prometheus' | 'entra-id' | 'intune' | 'keycloak' | 'ldap' | 'gitlab' | 'sonarqube' | 'personio'

export default function IntegrationsPage() {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState<Tab>('github')

  const tabs: { id: Tab; label: string; icon: React.ReactNode }[] = [
    { id: 'github', label: 'GitHub', icon: <GitBranch className="w-4 h-4" /> },
    { id: 'aws', label: 'AWS', icon: <Cloud className="w-4 h-4" /> },
    { id: 'azure', label: 'Azure', icon: <Cloud className="w-4 h-4" /> },
    { id: 'hetzner', label: 'Hetzner', icon: <Cloud className="w-4 h-4" /> },
    { id: 'ionos', label: 'IONOS', icon: <Cloud className="w-4 h-4" /> },
    { id: 'wazuh', label: 'Wazuh', icon: <Cloud className="w-4 h-4" /> },
    { id: 'prometheus', label: 'Prometheus', icon: <Cloud className="w-4 h-4" /> },
    { id: 'entra-id', label: 'Entra ID', icon: <ShieldAlert className="w-4 h-4" /> },
    { id: 'intune', label: 'Intune', icon: <ShieldAlert className="w-4 h-4" /> },
    { id: 'keycloak', label: 'Keycloak', icon: <ShieldAlert className="w-4 h-4" /> },
    { id: 'ldap', label: 'LDAP/AD', icon: <ShieldAlert className="w-4 h-4" /> },
    { id: 'gitlab', label: 'GitLab', icon: <GitBranch className="w-4 h-4" /> },
    { id: 'sonarqube', label: 'SonarQube', icon: <ShieldAlert className="w-4 h-4" /> },
    { id: 'personio', label: 'Personio', icon: <ShieldAlert className="w-4 h-4" /> },
  ]

  return (
    <div className="p-6 max-w-4xl mx-auto">
      {/* Page header */}
      <div className="flex items-center gap-2.5 mb-6">
        <Plug className="w-5 h-5 text-brand" />
        <div>
          <h1 className="text-lg font-semibold text-primary">{t('integrations.page.title')}</h1>
          <p className="text-xs text-secondary">{t('integrations.page.description')}</p>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex flex-wrap gap-1 border-b border-border mb-6">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => { setActiveTab(tab.id); }}
            className={`flex items-center gap-1.5 px-4 py-2 text-sm font-medium border-b-2 transition-colors -mb-px ${
              activeTab === tab.id
                ? 'border-brand text-brand'
                : 'border-transparent text-secondary hover:text-primary'
            }`}
          >
            {tab.icon}
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      {activeTab === 'github' && <GitHubTab />}
      {activeTab === 'aws' && <AWSTab />}
      {activeTab === 'azure' && <AzureTab />}
      {activeTab === 'hetzner' && <HetznerTab />}
      {activeTab === 'ionos' && <IONOSTab />}
      {activeTab === 'wazuh' && <WazuhTab />}
      {activeTab === 'prometheus' && <PrometheusTab />}
      {activeTab === 'entra-id' && <EntraIDTab />}
      {activeTab === 'intune' && <IntuneTab />}
      {activeTab === 'keycloak' && <KeycloakTab />}
      {activeTab === 'ldap' && <LDAPTab />}
      {activeTab === 'gitlab' && <GitLabTab />}
      {activeTab === 'sonarqube' && <SonarQubeTab />}
      {activeTab === 'personio' && <PersonioTab />}

      {/* No third-party integrations notice */}
      <div className="mt-6">
        <NoThirdPartyInfoBox />
      </div>
    </div>
  )
}
