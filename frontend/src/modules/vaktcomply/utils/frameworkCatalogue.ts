export interface FrameworkCatalogueEntry {
  key: string
  name: string
  fullName: string
  description: string
  category: string
  controls: string
  color: string
  /** Der Standard selbst ist noch ein Entwurf (Publikation steht aus). */
  draft?: boolean
  /**
   * Der Standard ist fertig, aber Vakt verkauft ihn nicht — kein ausgestellter
   * Lizenzschlüssel trägt das zugehörige Feature (features.UnsoldFeatures).
   *
   * Eigenes Kennzeichen statt `draft`, weil `draft` „Entwurf — erwartet Ende
   * 2026" beschriftet. Für TISAX (VDA ISA 6.0, veröffentlicht), DORA (seit
   * Januar 2025 in Kraft) und ISO/IEC 42001:2023 wäre das schlicht unwahr — die
   * Karte hätte eine falsche Tatsache behauptet, um einen Knopf abzuschalten.
   */
  notOffered?: boolean
  /**
   * Lizenz-Merkmal, das die Aktivierung gatet (frameworkFeatureGate in
   * backend/internal/modules/vaktcomply/handler.go). Liegt es in ProTier, trägt die
   * Karte die Pro-Plakette. FrameworksPage.tiers.test.tsx hält beides zusammen.
   */
  feature?: string
}

// Vordefinierte Compliance-Frameworks, die sich mit einem Klick aktivieren lassen.
// `key` ist der Name, unter dem das Backend das Framework führt (policy/plugins.go),
// `name` der Anzeigename für die Oberfläche.
export const FRAMEWORK_CATALOGUE: FrameworkCatalogueEntry[] = [
  {
    key: 'NIS2',
    name: 'NIS2',
    fullName: 'NIS-2-Richtlinie (EU) 2022/2555',
    description: 'EU-weite Richtlinie zur Netz- und Informationssicherheit. Verbindlich für wesentliche und wichtige Einrichtungen ab Dezember 2026 (NIS2UmsuCG).',
    category: 'EU-Recht',
    controls: '~90 Maßnahmen',
    color: 'text-blue-500',
  },
  {
    key: 'ISO27001',
    name: 'ISO 27001',
    fullName: 'ISO/IEC 27001:2022',
    description: 'Internationaler Standard für Informationssicherheits-Management. Grundlage für Zertifizierungen und die Nachweisbarkeit gegenüber Kunden.',
    category: 'International',
    controls: '93 Controls (Annex A)',
    color: 'text-green-500',
  },
  {
    key: 'BSI',
    feature: 'bsi_grundschutz',
    name: 'BSI IT-Grundschutz',
    fullName: 'BSI IT-Grundschutz-Kompendium 2023',
    description: 'Bewährter deutscher Standard des Bundesamts für Sicherheit in der Informationstechnik. Enthält detaillierte Bausteine und Umsetzungshinweise.',
    category: 'Deutschland',
    controls: '111 Bausteine',
    color: 'text-yellow-500',
  },
  {
    key: 'DORA',
    feature: 'dora',
    name: 'DORA',
    fullName: 'Digital Operational Resilience Act (EU) 2022/2554',
    description: 'Verbindlich seit Januar 2025 für Banken, Versicherungen, Zahlungsdienstleister und deren kritische IKT-Drittanbieter. Regelt digitale operationale Resilienz und Vorfallmeldepflichten.',
    category: 'EU-Recht / Finanz',
    controls: '15 Controls (Kap. II–V)',
    color: 'text-orange-500',
    // Aus dem Angebot genommen (v0.42.20). Seit 2026-08-08 trägt zusätzlich
    // kein ausgestellter Schlüssel mehr das Feature (features.UnsoldFeatures).
    notOffered: true,
  },
  {
    key: 'EUAIACT',
    feature: 'eu_ai_act',
    name: 'EU AI Act',
    fullName: 'EU AI Act — Verordnung (EU) 2024/1689',
    description: 'Neue EU-Verordnung für KI-Systeme. Hochrisiko-KI-Anforderungen ab 2. Dezember 2027 (Anhang III) bzw. 2. August 2028 (Anhang I), geändert durch den Digital Omnibus (VO (EU) 2026/1744). Betrifft jeden, der KI-Systeme in der EU entwickelt, betreibt oder einsetzt.',
    category: 'EU-Recht / KI',
    controls: '17 Controls (Annex III/IV)',
    color: 'text-purple-500',
  },
  {
    key: 'TISAX',
    feature: 'tisax',
    name: 'TISAX® / VDA ISA',
    fullName: 'TISAX® — Trusted Information Security Assessment Exchange (VDA ISA 6.0)',
    description: 'Verbindlicher Informationssicherheitsstandard der Automobilindustrie. Pflicht für Zulieferer mit Zugang zu sensitiven OEM-Daten (BMW, Mercedes, VW, Bosch, Continental).',
    category: 'Automotive',
    controls: '39 Controls (Kap. 1–15)',
    color: 'text-red-500',
    // Aus dem Angebot genommen (v0.42.20) — Katalog-Eintrag muss das spiegeln,
    // sonst zeigt der Button "Aktivieren", obwohl das Backend jede Aktivierung
    // ablehnt (Backend führt TISAX zusätzlich als draft in plugins.go).
    notOffered: true,
  },
  {
    key: 'ISO42001',
    feature: 'iso_42001',
    name: 'ISO 42001',
    fullName: 'ISO/IEC 42001:2023',
    description: 'KI-Managementsystem-Standard für verantwortungsvolle Entwicklung und Nutzung von KI. Ergänzt den EU AI Act mit einem strukturierten Managementrahmen.',
    category: 'International / KI',
    controls: '16 Controls',
    color: 'text-cyan-500',
    // Bis 2026-08-08 hing ISO 42001 an einem Tier, den kein Signierpfad
    // ausstellen konnte: JEDE Aktivierung lief ins 402, während der Katalog
    // einen aktiven Knopf zeigte. Der Tier ist weg, ISO 42001 bleibt unverkauft
    // und damit gegated (features.UnsoldFeatures).
    notOffered: true,
  },
  // S131-G5 (R-M08/V08-E): ISO 27017/27018 sind FeatureMultiFramework-gegatete
  // Pro-Frameworks, die serverseitig (plugins.go) existieren, aber im hartkodierten
  // Katalog fehlten — bezahlte Funktionalität ohne UI-Aktivierungspfad. Enable→402
  // für Community wird vom bestehenden Flow gehandhabt (wie CRA/EUAIACT/ISO42001).
  {
    key: 'ISO27017',
    feature: 'multi_framework',
    name: 'ISO 27017',
    fullName: 'ISO/IEC 27017:2015',
    description: 'Code of Practice für Informationssicherheit in Cloud-Diensten (CSP & CSC). Ergänzt ISO 27001 um 7 Cloud-spezifische Controls.',
    category: 'International / Cloud',
    controls: '7 Cloud-Controls',
    color: 'text-sky-500',
  },
  {
    key: 'ISO27018',
    feature: 'multi_framework',
    name: 'ISO 27018',
    fullName: 'ISO/IEC 27018:2019',
    description: 'Schutz personenbezogener Daten (PII) in Public Clouds — für Auftragsverarbeiter nach Art. 28 DSGVO.',
    category: 'International / Cloud',
    controls: 'PII-Cloud-Controls',
    color: 'text-teal-500',
  },
  // S131-G1/V08-D (Produktentscheidung 2026-07-23): die vier zuvor ungegateten
  // Zusatz-Frameworks sind jetzt Pro (FeatureMultiFramework, siehe handler.go) und
  // im Katalog sichtbar wie CRA/EUAIACT — Community bekommt beim Aktivieren 402.
  {
    key: 'DSGVO-TOM',
    feature: 'multi_framework',
    name: 'DSGVO Art. 32 TOM',
    fullName: 'DSGVO Art. 32 — Technische und organisatorische Maßnahmen',
    description: 'Technische und organisatorische Maßnahmen nach Art. 32 DSGVO, abgeleitet aus ISO 27001 — Nachweis angemessener Sicherheit der Verarbeitung.',
    category: 'EU-Recht / Datenschutz',
    controls: 'Art. 32 TOMs',
    color: 'text-blue-400',
  },
  {
    key: 'CIS',
    feature: 'multi_framework',
    name: 'CIS Controls v8',
    fullName: 'CIS Controls v8 — Center for Internet Security',
    description: '18 Kontrollgruppen des Center for Internet Security. IG1-Safeguards als Mindestanforderung für alle Organisationen.',
    category: 'International',
    controls: '18 Kontrollgruppen',
    color: 'text-emerald-500',
  },
  {
    key: 'KRITIS',
    feature: 'multi_framework',
    name: 'KRITIS-DachG',
    fullName: 'KRITIS-Dachgesetz — Kritische Infrastrukturen',
    description: 'Resilienzanforderungen für Betreiber kritischer Anlagen in Deutschland (§§ 8, 12, 13, 16, 18, 20 KRITIS-DachG).',
    category: 'Deutschland / KRITIS',
    controls: 'Resilienz-Controls',
    color: 'text-rose-500',
  },
  {
    key: 'C5',
    feature: 'multi_framework',
    name: 'BSI C5:2020',
    fullName: 'BSI C5:2020 — Cloud Computing Compliance Criteria Catalogue',
    description: 'Prüfgrundlage des BSI für die Sicherheit von Cloud-Diensten in Deutschland (Hetzner, IONOS, AWS-DE, Azure-DE).',
    category: 'Deutschland / Cloud',
    controls: 'Cloud-Kriterien',
    color: 'text-violet-500',
  },
  {
    key: 'CRA',
    feature: 'cra',
    name: 'EU CRA',
    fullName: 'EU Cyber Resilience Act (EU) 2024/2847',
    description: 'Sicherheitsanforderungen für Produkte mit digitalen Elementen. Gilt für Hersteller und Händler in der EU. SBOM, Patch-Management und Meldepflichten verpflichtend.',
    category: 'EU-Recht / Produkte',
    controls: '13 Controls',
    color: 'text-indigo-500',
  },
  {
    key: 'prEN18286',
    name: 'prEN 18286',
    fullName: 'prEN 18286 — EU AI Act harmonisierter Standard',
    description: 'Harmonisierter Standard zum EU AI Act für KI-Managementsysteme. Komplementär zu ISO/IEC 42001:2023. Publikation erwartet Ende 2026 — Entwurf bereits in Vakt hinterlegt.',
    category: 'EU-Recht / KI',
    controls: '8 Abschnitte (Entwurf)',
    color: 'text-amber-500',
    draft: true,
  },
]

/** Schlüssel ohne Groß-/Kleinschreibung und Trennzeichen: „dsgvo_tom" ≙ „DSGVO-TOM". */
function normalizeKey(key: string): string {
  return key.toLowerCase().replace(/[^a-z0-9]/g, '')
}

const DISPLAY_NAMES = new Map<string, string>(
  FRAMEWORK_CATALOGUE.map((fw) => [normalizeKey(fw.key), fw.name]),
)

/**
 * Anzeigename eines Frameworks aus dem Katalog.
 *
 * Das Backend liefert in `name` den internen Schlüssel („ISO27001", „EUAIACT").
 * Der gehört nicht in die Oberfläche — dort steht, was der Katalog schreibt
 * („ISO 27001", „EU AI Act"). Ein Name, den der Katalog nicht kennt (eigenes
 * Plugin-Framework), wird unverändert angezeigt: Er ist dann bereits der Name,
 * den der Betreiber vergeben hat.
 */
export function frameworkDisplayName(key: string | null | undefined): string {
  if (!key) return ''
  return DISPLAY_NAMES.get(normalizeKey(key)) ?? key
}
