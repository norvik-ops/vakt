# ADR-0008: Kein MSP-Portal — Phone-Home-Verstoß

**Status:** Accepted  
**Datum:** 2026-05-20

## Kontext

Eine externe Produkt-Analyse (Mai 2026) schlug ein „MSP-Portal" vor: ein zentrales Dashboard, das einem Managed Service Provider Übersicht über alle seine Kunden-Instanzen verschafft (Health-Status, Update-Tracking, Queue-Tiefe pro Kunde).

Das wäre kommerziell attraktiv — der DACH-MSP-Markt ist groß und MSPs schätzen zentrale Operability.

## Entscheidung

**Kein MSP-Portal in Vakt.** Dauerhaft ausgeschlossen.

## Alternativen

- **Zentrales SaaS-MSP-Portal** — verworfen: erfordert, dass jede Kunden-Instanz Health-/Status-Daten an einen zentralen Server sendet. Das verletzt direkt ADR-0001 (kein Phone-Home).
- **Federation-Protokoll** — verworfen: Kunden-Daten würden über mehrere Vakt-Instanzen propagiert; Trust-Boundary verschwimmt; ein Bug in der Federation kann Daten zwischen Kunden lecken.
- **Optional aktivierbares MSP-Portal mit Customer-Consent** — verworfen: schafft zwei Produkt-Linien, untergräbt die einheitliche „Daten bleiben lokal"-Marketing-Aussage.

## Konsequenzen

### Positive

- Konsistenz mit dem Kern-Versprechen.
- Keine kompromittierte Trust-Boundary.
- Vakt bleibt eindeutig DSGVO-positioniert.

### Negative

- MSPs deployen pro Kunde eine eigene Instanz und verwalten die mit ihrem eigenen Tooling (Ansible, Terraform, Pulumi, etc.).
- Kein Co-Selling mit „seht alles an einem Ort" — Argument.

### Neutrale

- Helm-Chart und docker-compose erlauben skriptbares Deployment — MSPs müssen das selbst orchestrieren.
- Vakt liefert ggf. Beispiel-Ansible-Playbooks für MSPs (separates Repo, optional).

## Referenzen

- ADR-0001 (Phone-Home-Verbot)
- Roadmap-Entscheidung 2026: MSP-Portal dauerhaft ausgeschlossen (Phone-Home-Prinzip)
