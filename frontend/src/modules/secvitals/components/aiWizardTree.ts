export type RiskClass = 'minimal' | 'limited' | 'high' | 'unacceptable'

export interface WizardResult {
  riskClass: RiskClass
  rationale: string
  article: string
}

export interface WizardStep {
  id: string
  question: string
  explanation: string
  article: string
  yesLeadsTo: string | null
  noLeadsTo: string | null
  yesResult?: WizardResult
  noResult?: WizardResult
}

export const WIZARD_STEPS: WizardStep[] = [
  {
    id: 'step_prohibited',
    question: 'Fällt das KI-System unter die verbotenen Anwendungen nach Art. 5 EU AI Act?',
    explanation:
      'Verbotene Systeme umfassen: Social Scoring durch Behörden, biometrische Echtzeit-Massenüberwachung in öffentlichen Räumen, Systeme zur Ausnutzung von Schwachstellen (Alter, Behinderung), unterschwellige Manipulation sowie KI-gestützte Vorhersage-Polizeiarbeit auf Basis persönlicher Merkmale.',
    article: 'Art. 5 EU AI Act (Verbote)',
    yesLeadsTo: null,
    noLeadsTo: 'step_high_risk',
    yesResult: {
      riskClass: 'unacceptable',
      rationale: 'Das System fällt unter die verbotenen KI-Anwendungen nach Art. 5 EU AI Act und darf nicht eingesetzt werden.',
      article: 'Art. 5 EU AI Act',
    },
  },
  {
    id: 'step_high_risk',
    question: 'Wird das System in einem der Hochrisikobereiche nach Annex III eingesetzt?',
    explanation:
      'Hochrisikobereiche (Annex III): Biometrische Identifizierung/Kategorisierung, kritische Infrastruktur (Energie, Wasser, Verkehr), Bildung und Berufsausbildung, Beschäftigung (Recruiting, Leistungsbewertung, Entlassungen), grundlegende öffentliche/private Dienste (Kredit, Sozialleistungen, Notfalldienste), Strafverfolgung, Migrations- und Grenzkontrolle, Rechtspflege.',
    article: 'Annex III EU AI Act (Hochrisiko-Kategorien)',
    yesLeadsTo: null,
    noLeadsTo: 'step_transparency',
    yesResult: {
      riskClass: 'high',
      rationale: 'Das System ist in einem Hochrisikobereich nach Annex III eingesetzt und unterliegt den vollständigen Konformitätsanforderungen (Art. 8–15 EU AI Act).',
      article: 'Annex III EU AI Act',
    },
  },
  {
    id: 'step_transparency',
    question: 'Interagiert das System direkt mit Menschen (z.B. als Chatbot) oder erzeugt es Inhalte ohne erkennbaren KI-Ursprung?',
    explanation:
      'Transparenzpflicht besteht für: Chatbots und konversationale KI-Systeme, KI-generierte Texte, Bilder, Audio oder Video (Deepfakes) sowie Emotion-Recognition- und biometrische Kategorisierungssysteme. Nutzer müssen erkennbar über die KI-Interaktion informiert werden.',
    article: 'Art. 50 EU AI Act (Transparenzpflicht)',
    yesLeadsTo: null,
    noLeadsTo: null,
    yesResult: {
      riskClass: 'limited',
      rationale: 'Das System unterliegt der Transparenzpflicht nach Art. 50 EU AI Act (begrenztes Risiko).',
      article: 'Art. 50 EU AI Act',
    },
    noResult: {
      riskClass: 'minimal',
      rationale: 'Das System fällt in die Kategorie minimales Risiko und unterliegt keinen spezifischen EU AI Act-Anforderungen.',
      article: 'Art. 6 Abs. 3 EU AI Act',
    },
  },
]

export function getStep(id: string): WizardStep | undefined {
  return WIZARD_STEPS.find((s) => s.id === id)
}
