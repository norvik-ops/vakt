import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { Shield, CheckCircle, Clock, AlertCircle, ExternalLink, Award, FileText, Users } from 'lucide-react'

interface FrameworkStatus {
  name: string
  version: string
  compliance_percent: number
  total_controls: number
  compliant_controls: number
}

interface Certificate {
  id: string
  name: string
  issuer?: string
  issued_at?: string
  expires_at?: string
}

interface PublicPolicy {
  id: string
  title: string
  body: string
}

interface TrustData {
  org_name: string
  description: string
  contact: string
  logo_url?: string
  frameworks: FrameworkStatus[]
  certificates?: Certificate[]
  public_policies?: PublicPolicy[]
  subprocessors_md?: string
  show_frameworks: boolean
  show_policies: boolean
  show_certs: boolean
  published_at: string
  powered_by: string
}

type TabId = 'frameworks' | 'certs' | 'policies' | 'subprocessors'

function ComplianceBar({ percent }: { percent: number }) {
  const color = percent >= 80 ? 'bg-green-500' : percent >= 50 ? 'bg-amber-500' : 'bg-red-500'
  return (
    <div className="w-full bg-gray-200 rounded-full h-2">
      <div className={`${color} h-2 rounded-full transition-all`} style={{ width: `${percent}%` }} />
    </div>
  )
}

function StatusIcon({ percent }: { percent: number }) {
  if (percent >= 80) return <CheckCircle className="h-5 w-5 text-green-500" />
  if (percent >= 50) return <Clock className="h-5 w-5 text-amber-500" />
  return <AlertCircle className="h-5 w-5 text-red-500" />
}

function TabButton({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <button
      onClick={onClick}
      className={`px-4 py-2 text-sm font-medium rounded-lg transition-colors ${
        active
          ? 'bg-white text-indigo-700 shadow-sm border border-gray-200'
          : 'text-gray-600 hover:text-gray-900 hover:bg-gray-100'
      }`}
    >
      {children}
    </button>
  )
}

function PolicyItem({ policy }: { policy: PublicPolicy }) {
  const [open, setOpen] = useState(false)
  return (
    <div className="bg-white rounded-xl border overflow-hidden">
      <button
        onClick={() => setOpen(!open)}
        className="w-full flex items-center justify-between px-5 py-4 text-left hover:bg-gray-50 transition-colors"
      >
        <div className="flex items-center gap-2">
          <FileText className="h-4 w-4 text-indigo-500 shrink-0" />
          <span className="font-medium text-gray-900">{policy.title}</span>
        </div>
        <span className="text-gray-400 text-xs">{open ? '▲' : '▼'}</span>
      </button>
      {open && policy.body && (
        <div className="px-5 pb-5 border-t bg-gray-50">
          <p className="text-sm text-gray-700 whitespace-pre-wrap mt-3">{policy.body}</p>
        </div>
      )}
    </div>
  )
}

export default function TrustPage() {
  const { slug } = useParams<{ slug: string }>()
  const [data, setData] = useState<TrustData | null>(null)
  const [loading, setLoading] = useState(true)
  const [notFound, setNotFound] = useState(false)
  const [activeTab, setActiveTab] = useState<TabId>('frameworks')

  useEffect(() => {
    fetch(`/api/v1/trust/${slug}`)
      .then(res => {
        if (!res.ok) { setNotFound(true); return null }
        return res.json()
      })
      .then((d: TrustData | null) => {
        if (d) {
          setData(d)
          // Set default tab to first available
          if (d.show_frameworks) setActiveTab('frameworks')
          else if (d.show_certs) setActiveTab('certs')
          else if (d.show_policies) setActiveTab('policies')
          else if (d.subprocessors_md) setActiveTab('subprocessors')
        }
      })
      .finally(() => setLoading(false))
  }, [slug])

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600" />
      </div>
    )
  }

  if (notFound || !data) {
    return (
      <div className="min-h-screen flex flex-col items-center justify-center bg-gray-50 gap-4">
        <Shield className="h-12 w-12 text-gray-300" />
        <h1 className="text-xl font-semibold text-gray-700">Trust Center nicht gefunden</h1>
        <p className="text-gray-500">Diese Organisation hat kein öffentliches Trust Center aktiviert.</p>
      </div>
    )
  }

  const frameworks = data.frameworks ?? []
  const certificates = data.certificates ?? []
  const publicPolicies = data.public_policies ?? []

  const overallPercent = frameworks.length > 0
    ? Math.round(frameworks.reduce((s, f) => s + f.compliance_percent, 0) / frameworks.length)
    : 0

  const availableTabs: { id: TabId; label: string; icon: React.ElementType }[] = []
  if (data.show_frameworks && frameworks.length > 0) {
    availableTabs.push({ id: 'frameworks', label: 'Frameworks', icon: Shield })
  }
  if (data.show_certs && certificates.length > 0) {
    availableTabs.push({ id: 'certs', label: 'Zertifikate', icon: Award })
  }
  if (data.show_policies && publicPolicies.length > 0) {
    availableTabs.push({ id: 'policies', label: 'Policies', icon: FileText })
  }
  if (data.subprocessors_md) {
    availableTabs.push({ id: 'subprocessors', label: 'Unterauftragnehmer', icon: Users })
  }

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <div className="bg-white border-b">
        <div className="max-w-3xl mx-auto px-6 py-8">
          <div className="flex items-start gap-4">
            {data.logo_url ? (
              <img src={data.logo_url} alt={data.org_name} className="h-14 w-auto object-contain" />
            ) : (
              <div className="p-3 bg-indigo-100 rounded-xl">
                <Shield className="h-8 w-8 text-indigo-600" />
              </div>
            )}
            <div>
              <h1 className="text-2xl font-bold text-gray-900">{data.org_name}</h1>
              <p className="text-gray-500 mt-1">Security & Compliance Trust Center</p>
              {data.description && (
                <p className="text-gray-700 mt-3 max-w-xl">{data.description}</p>
              )}
            </div>
          </div>

          {/* Overall score — only when frameworks are shown */}
          {data.show_frameworks && frameworks.length > 0 && (
            <div className="mt-6 flex items-center gap-6">
              <div className="text-center">
                <div className="text-4xl font-bold text-indigo-600">{overallPercent}%</div>
                <div className="text-xs text-gray-500 mt-1">Gesamtcompliance</div>
              </div>
              <div className="flex-1">
                <ComplianceBar percent={overallPercent} />
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Tab navigation */}
      {availableTabs.length > 1 && (
        <div className="max-w-3xl mx-auto px-6 pt-6">
          <div className="flex gap-2 bg-gray-100 rounded-xl p-1 w-fit">
            {availableTabs.map(tab => (
              <TabButton
                key={tab.id}
                active={activeTab === tab.id}
                onClick={() => setActiveTab(tab.id)}
              >
                {tab.label}
              </TabButton>
            ))}
          </div>
        </div>
      )}

      {/* Tab content */}
      <div className="max-w-3xl mx-auto px-6 py-6">

        {/* Frameworks tab */}
        {activeTab === 'frameworks' && data.show_frameworks && (
          <div className="grid gap-4">
            {frameworks.map(fw => (
              <div key={fw.name} className="bg-white rounded-xl border p-5">
                <div className="flex items-center justify-between mb-3">
                  <div className="flex items-center gap-2">
                    <StatusIcon percent={fw.compliance_percent} />
                    <div>
                      <span className="font-semibold text-gray-900">{fw.name}</span>
                      {fw.version && (
                        <span className="ml-2 text-xs text-gray-400">{fw.version}</span>
                      )}
                    </div>
                  </div>
                  <span className="text-lg font-bold text-gray-900">{fw.compliance_percent}%</span>
                </div>
                <ComplianceBar percent={fw.compliance_percent} />
                <div className="mt-2 text-xs text-gray-500">
                  {fw.compliant_controls} von {fw.total_controls} Kontrollen erfüllt
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Certificates tab */}
        {activeTab === 'certs' && data.show_certs && (
          <div className="grid gap-4 sm:grid-cols-2">
            {certificates.map(cert => (
              <div key={cert.id} className="bg-white rounded-xl border p-5 flex flex-col gap-2">
                <div className="flex items-center gap-2">
                  <Award className="h-5 w-5 text-indigo-500 shrink-0" />
                  <span className="font-semibold text-gray-900">{cert.name}</span>
                </div>
                {cert.issuer && (
                  <div className="text-xs text-gray-500">Aussteller: {cert.issuer}</div>
                )}
                <div className="flex gap-4 text-xs text-gray-400 mt-1">
                  {cert.issued_at && <span>Ausgestellt: {cert.issued_at}</span>}
                  {cert.expires_at && <span>Gültig bis: {cert.expires_at}</span>}
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Policies tab */}
        {activeTab === 'policies' && data.show_policies && (
          <div className="grid gap-3">
            {publicPolicies.map(policy => (
              <PolicyItem key={policy.id} policy={policy} />
            ))}
          </div>
        )}

        {/* Subprocessors tab */}
        {activeTab === 'subprocessors' && data.subprocessors_md && (
          <div className="bg-white rounded-xl border p-6">
            <div className="flex items-center gap-2 mb-4">
              <Users className="h-5 w-5 text-indigo-500" />
              <h2 className="font-semibold text-gray-900">Unterauftragnehmer</h2>
            </div>
            <pre className="text-sm text-gray-700 whitespace-pre-wrap font-sans leading-relaxed">
              {data.subprocessors_md}
            </pre>
          </div>
        )}

        {/* Contact */}
        {data.contact && (
          <div className="mt-6 bg-white rounded-xl border p-5">
            <h3 className="font-semibold text-gray-900 mb-2">Kontakt für Auditoranfragen</h3>
            <a href={`mailto:${data.contact}`} className="text-indigo-600 hover:underline flex items-center gap-1">
              {data.contact} <ExternalLink className="h-3 w-3" />
            </a>
          </div>
        )}

        {/* Footer */}
        <div className="mt-8 text-center text-xs text-gray-400">
          <p>Zuletzt aktualisiert: {new Date(data.published_at).toLocaleDateString('de-DE')}</p>
          <p className="mt-1">
            Powered by{' '}
            <a href="https://github.com/sechealth-app/sechealth" className="text-indigo-500 hover:underline" target="_blank" rel="noreferrer">
              Vakt
            </a>
          </p>
        </div>
      </div>
    </div>
  )
}
