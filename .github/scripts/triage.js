module.exports = async ({github, context, core}) => {
  const issue = context.payload.issue;
  const labels = issue.labels.map(l => l.name);

  // ── Claude: Triage ──────────────────────────────────────────────────────────
  const triagePrompt = [
    "Du bist der Triage-Agent fuer Vakt, eine selbst gehostete ISMS-Plattform fuer den DACH-Mittelstand.",
    "Antworte AUSSCHLIESSLICH mit einem JSON-Objekt ohne Markdown:",
    '{"type":"bug|feature|question|spam","labels_add":[],"labels_remove":[],"severity":"critical|high|medium|low|none","comment":"Kommentar auf Deutsch","jira_create":true,"jira_summary":"Zusammenfassung max 80 Zeichen"}',
    "Regeln:",
    "- type: bug/feature/question/spam erkennen",
    "- labels_add: bug, feature, question, severity: critical, severity: high, severity: medium, severity: low, status: needs-info, status: confirmed",
    "- labels_remove: 'status: needs-triage' entfernen wenn Typ klar",
    "- severity: critical NUR bei Datenverlust oder Auth-Bypass",
    "- comment: Spezifisch auf Deutsch, 3-5 Saetze. Kein generisches Danke. Bei Bugs: bestaetigen was du verstanden hast, kurz erklaeren was als naechstes passiert (z.B. wir pruefen das in der naechsten Version, wir reproduzieren das intern), und falls relevant eine Einschaetzung zur Prioritaet geben. Bei lueckenhaften Reports: gezielt nachfragen was fehlt. Bei Features: ehrliche Einschaetzung ob/wann realistisch und warum.",
    "- jira_create: true bei reproduzierbaren Bugs und sinnvollen Features",
    "- jira_summary: max 80 Zeichen Deutsch"
  ].join("\n");

  const res = await fetch("https://api.anthropic.com/v1/messages", {
    method: "POST",
    headers: {"Content-Type": "application/json", "x-api-key": process.env.ANTHROPIC_API_KEY, "anthropic-version": "2023-06-01"},
    body: JSON.stringify({
      model: "claude-haiku-4-5-20251001", max_tokens: 1024,
      system: triagePrompt,
      messages: [{role: "user", content: "Issue #" + issue.number + ": " + issue.title + "\nLabels: " + (labels.join(", ") || "keine") + "\n\n" + (issue.body || "(leer)")}]
    })
  });
  const data = await res.json();
  let result;
  try {
    const m = data.content[0].text.trim().match(/\{[\s\S]*\}/);
    result = JSON.parse(m ? m[0] : data.content[0].text.trim());
  } catch(e) { core.error("Triage Parse Error: " + data.content[0].text); return; }

  core.info("Triage: " + JSON.stringify(result));

  // Labels
  if (result.labels_add && result.labels_add.length) {
    await github.rest.issues.addLabels({owner: context.repo.owner, repo: context.repo.repo, issue_number: issue.number, labels: result.labels_add});
  }
  for (const label of (result.labels_remove || [])) {
    try { await github.rest.issues.removeLabel({owner: context.repo.owner, repo: context.repo.repo, issue_number: issue.number, name: label}); } catch(e) {}
  }

  // Kommentar GitHub (immer kurz und oeffentlich)
  if (result.comment) {
    await github.rest.issues.createComment({
      owner: context.repo.owner, repo: context.repo.repo, issue_number: issue.number,
      body: result.comment + "\n\n_\u2014 Vakt Team_"
    });
  }

  // Jira Issue anlegen
  let jiraKey = null;
  if (result.jira_create && process.env.JIRA_API_TOKEN && process.env.JIRA_EMAIL) {
    const jr = await fetch("https://norvikops.atlassian.net/rest/api/3/issue", {
      method: "POST",
      headers: {"Content-Type": "application/json", "Authorization": "Basic " + Buffer.from(process.env.JIRA_EMAIL + ":" + process.env.JIRA_API_TOKEN).toString("base64")},
      body: JSON.stringify({fields: {
        project: {key: "VAKT"},
        summary: (result.jira_summary || issue.title).substring(0, 80),
        description: {type: "doc", version: 1, content: [{type: "paragraph", content: [{type: "text", text: "GitHub #" + issue.number + ": " + issue.html_url + "\n\n" + (issue.body || "")}]}]},
        issuetype: {name: result.type === "bug" ? "Bug" : "Story"},
        parent: {key: "VAKT-866"}
      }})
    });
    const jd = await jr.json();
    if (jd.key) {
      jiraKey = jd.key;
      core.info("Jira: " + jiraKey);
      await github.rest.issues.createComment({
        owner: context.repo.owner, repo: context.repo.repo, issue_number: issue.number,
        body: "\uD83D\uDCCB Jira: [" + jiraKey + "](https://norvikops.atlassian.net/browse/" + jiraKey + ")"
      });
    }
  }

  // ── Bug-Diagnose: Code analysieren → nur in Jira ───────────────────────────
  if (result.type === "bug" && jiraKey && result.severity !== "none") {
    try {
      // Repo-Struktur holen fuer Kontext
      const treeRes = await github.rest.git.getTree({
        owner: context.repo.owner, repo: context.repo.repo,
        tree_sha: "main", recursive: "true"
      });
      const relevantFiles = treeRes.data.tree
        .filter(f => f.type === "blob" && (f.path.endsWith(".go") || f.path.endsWith(".ts") || f.path.endsWith(".tsx")))
        .map(f => f.path)
        .slice(0, 80);

      const diagPrompt = "Du bist ein erfahrener Go/TypeScript Entwickler und analysierst Bugs in Vakt, einer ISMS-Plattform. Gegeben ist ein Bug-Report und die Dateistruktur des Repos. Identifiziere welche Dateien wahrscheinlich betroffen sind und beschreibe wo der Fehler vermutlich liegt. Antworte auf Deutsch, technisch praezise, fuer interne Nutzung (nicht oeffentlich). Format: kurze Diagnose + verdaechtige Dateien/Bereiche + moegliche Ursache.";

      const diagRes = await fetch("https://api.anthropic.com/v1/messages", {
        method: "POST",
        headers: {"Content-Type": "application/json", "x-api-key": process.env.ANTHROPIC_API_KEY, "anthropic-version": "2023-06-01"},
        body: JSON.stringify({
          model: "claude-haiku-4-5-20251001", max_tokens: 1024,
          system: diagPrompt,
          messages: [{role: "user", content: "Bug-Report:\nTitel: " + issue.title + "\n\n" + (issue.body || "") + "\n\nDateistruktur:\n" + relevantFiles.join("\n")}]
        })
      });
      const diagData = await diagRes.json();
      const diagnosis = diagData.content[0].text;
      core.info("Diagnose: " + diagnosis);

      // Diagnose als interner Jira-Kommentar
      await fetch("https://norvikops.atlassian.net/rest/api/3/issue/" + jiraKey + "/comment", {
        method: "POST",
        headers: {"Content-Type": "application/json", "Authorization": "Basic " + Buffer.from(process.env.JIRA_EMAIL + ":" + process.env.JIRA_API_TOKEN).toString("base64")},
        body: JSON.stringify({
          body: {
            type: "doc", version: 1,
            content: [
              {type: "heading", attrs: {level: 3}, content: [{type: "text", text: "Automatische Bug-Diagnose"}]},
              {type: "paragraph", content: [{type: "text", text: diagnosis}]},
              {type: "paragraph", content: [{type: "text", text: "GitHub Issue: " + issue.html_url, marks: [{type: "em"}]}]}
            ]
          }
        })
      });
      core.info("Diagnose in Jira gepostet: " + jiraKey);
    } catch(e) {
      core.warning("Diagnose fehlgeschlagen: " + e.message);
    }
  }
};
