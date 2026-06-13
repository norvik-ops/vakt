# Contributing to Vakt

Danke für dein Interesse an Vakt — einer selbst gehosteten ISMS-Plattform für den DACH-Mittelstand.

---

## 🐛 Bug melden

1. Prüfe die [bestehenden Issues](https://github.com/norvik-ops/vatk/issues), ob der Bug bereits gemeldet wurde.
2. Öffne ein neues Issue über **[Bug Report](https://github.com/norvik-ops/vatk/issues/new?template=bug_report.yml)**.
3. Fülle das Formular vollständig aus — insbesondere Version, Deployment-Typ und Reproduktionsschritte.

> **Sicherheitslücken** bitte **nicht** als öffentliches Issue melden, sondern vertraulich an **security@norvikops.de**.

## 💡 Feature vorschlagen

1. Prüfe die [bestehenden Issues](https://github.com/norvik-ops/vatk/issues) und [Diskussionen](https://github.com/norvik-ops/vatk/discussions).
2. Öffne ein neues Issue über **[Feature-Wunsch](https://github.com/norvik-ops/vatk/issues/new?template=feature_request.yml)**.

## 💬 Fragen & Diskussion

Für allgemeine Fragen und Feedback nutze bitte [GitHub Discussions](https://github.com/norvik-ops/vatk/discussions).

---

## 🤝 Mitarbeit & Mitstreiter

Vakt ist ein Solo-Projekt und sucht aktiv Mitstreiter in zwei Bereichen:

### Technische Mitarbeit
- Bugfixes, Feature-Entwicklung (Go / Next.js / TypeScript)
- Code Reviews, Security-Input, Architektur-Feedback
- DevOps / Infrastruktur (Docker, CI/CD, Hardening)

### Vertrieb & Go-to-Market
- Direkte Ansprache von DACH-KMUs, IT-Leitern, MSPs
- Partnerschaften / Reseller-Aufbau
- Deutschsprachiger Content, Community

### Was du bekommst
- Credit im README und Changelog für alle gemergten Beiträge
- Zugang zu den privaten Repos für aktive Mitwirkende
- Offenes Gespräch über tiefere Beteiligung — bei gutem Fit nach der Zusammenarbeit ist eine Revenue-Share- oder Co-Founder-Rolle möglich

Kein formaler Bewerbungsprozess. Meld dich einfach in den [Discussions](https://github.com/norvik-ops/vatk/discussions) oder schreib direkt an **stefan@norvikops.de**.

> Vakt ist bootstrapped. Es gibt aktuell keine Bounties oder Vorauszahlungen.

### Technischer Ablauf für PRs

```bash
git clone https://github.com/norvik-ops/vatk.git
cd vatk
docker compose up
```

- Branch-Namen: `fix/kurze-beschreibung` · `feat/kurze-beschreibung`
- PRs fokussiert halten — ein Thema pro PR
- Tests für neue Funktionalität werden erwartet

---

## ⏱ Response-Zeit (Beta)

Bug-Reports werden in der Regel innerhalb von **48 Stunden** triagiert.
