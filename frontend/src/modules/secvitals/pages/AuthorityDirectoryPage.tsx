import { Building2, ExternalLink, Phone, Send } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import { useAuthorities, useOrgSector } from '../hooks/useOrgSector'
import { SECTOR_LABELS } from '../types'

export default function AuthorityDirectoryPage() {
  const { data: authorities, isLoading } = useAuthorities()
  const { data: sectorSettings } = useOrgSector()

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Behörden-Verzeichnis"
        description="NIS2-Meldebehörden mit Kontaktinformationen und Einreichungskanälen."
      />

      <div className="flex-1 p-6 space-y-4">
        {sectorSettings && (
          <div className="text-sm text-muted-foreground bg-muted/30 rounded-lg px-4 py-2">
            Konfigurierter Sektor Ihrer Organisation:{' '}
            <span className="font-medium text-foreground" data-testid="sector-display">
              {SECTOR_LABELS[sectorSettings.sector] ?? sectorSettings.sector}
            </span>
          </div>
        )}

        {isLoading && (
          <div className="flex items-center justify-center h-32">
            <div className="w-5 h-5 border-2 border-primary border-t-transparent rounded-full animate-spin" />
          </div>
        )}

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4" data-testid="authority-list">
          {authorities?.map((auth) => (
            <Card key={auth.name}>
              <CardHeader>
                <CardTitle className="text-sm flex items-center gap-2">
                  <Building2 className="w-4 h-4" />
                  {auth.name}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-3 text-sm">
                <div className="flex items-center gap-2 text-muted-foreground">
                  <ExternalLink className="w-3.5 h-3.5 shrink-0" />
                  <a
                    href={auth.portal}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-primary hover:underline truncate"
                    data-testid={`authority-portal-${auth.name}`}
                  >
                    {auth.portal}
                  </a>
                </div>
                <div className="flex items-center gap-2 text-muted-foreground">
                  <Phone className="w-3.5 h-3.5 shrink-0" />
                  <span>{auth.phone}</span>
                </div>
                <div className="flex items-start gap-2">
                  <Send className="w-3.5 h-3.5 shrink-0 mt-0.5 text-muted-foreground" />
                  <p className="text-xs text-muted-foreground">{auth.submit_note}</p>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>

        {/* GDPR authority note */}
        <Card className="border-dashed">
          <CardContent className="pt-4">
            <p className="text-xs text-muted-foreground">
              <Badge variant="outline" className="mr-2 text-xs">DSGVO Art. 33</Badge>
              Bei Datenpannen mit personenbezogenen Daten muss zusätzlich die zuständige
              Landesdatenschutzbehörde innerhalb von 72h informiert werden.
              {sectorSettings?.federal_state
                ? ` Konfiguriertes Bundesland: ${sectorSettings.federal_state}.`
                : ' Bundesland kann in den Organisationseinstellungen konfiguriert werden.'}
            </p>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
