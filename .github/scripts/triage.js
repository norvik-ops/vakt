module.exports = async ({github, context, core}) => {
const issue = context.payload.issue;
const labels = issue.labels.map(l => l.name);

const systemPrompt = "Du bist der Triage-Agent fuer Vakt, eine selbst gehostete ISMS-Plattform fuer den DACH-Mittelstand.

Analysiere eingehende GitHub Issues und antworte AUSSCHLIESSLICH mit einem JSON-Objekt ohne Markdown:

{\"type\":\"bug|feature|question|spam\",\"labels_add\":[],\"labels_remove\":[],\"severity\":\"critical|high|medium|low|none\",\"comment\":\"Dein Kommentar auf Deutsch\",\"jira_create\":true,\"jira_summary\":\"Kurze Zusammenfassung\"}

Regeln:
- type: Erkenne selbst ob Bug, Feature, Frage oder Spam
- labels_add: Nutze: bug, feature, question, severity: critical, severity: high, severity: medium, severity: low, status: needs-info, status: confirmed
- labels_remove: Entferne status: needs-triage wenn Typ klar ist
- severity: Nur critical bei Datenverlust oder Auth-Bypass
- comment: Spezifisch auf Deutsch, kein generisches Danke. Bei unvollstaendigen Reports gezielt nachfragen. Bei Features ehrliche Einschaetzung.
- jira_create: true bei reproduzierbaren Bugs und sinnvollen Features
- jira_summary: Max 80 Zeichen Deutsch";

const userMessage = "Neues Issue #" + issue.number + ":

Titel: " + issue.title + "
Labels: " + (labels.join(", ") || "keine") + "

Body:
" + (issue.body || "(leer)");

const res = await fetch("https://api.anthropic.com/v1/messages", {
  method: "POST",
  headers: {
    "Content-Type": "application/json",
    "x-api-key": process.env.ANTHROPIC_API_KEY,
    "anthropic-version": "2023-06-01"
  },
  body: JSON.stringify({
    model: "claude-haiku-4-5-20251001",
    max_tokens: 1024,
    system: systemPrompt,
    messages: [{role: "user", content: userMessage}]
  })
});

const data = await res.json();
core.info("Claude response: " + JSON.stringify(data));

let result;
try {
  const text = data.content[0].text.trim();
  const jsonMatch = text.match(/\{[\s\S]*\}/);
  result = JSON.parse(jsonMatch ? jsonMatch[0] : text);
} catch(e) {
  core.error("Parse Error: " + data.content[0].text);
  return;
}

core.info("Triage result: " + JSON.stringify(result));

if (result.labels_add && result.labels_add.length) {
  await github.rest.issues.addLabels({owner: context.repo.owner, repo: context.repo.repo, issue_number: issue.number, labels: result.labels_add});
}

for (const label of (result.labels_remove || [])) {
  try { await github.rest.issues.removeLabel({owner: context.repo.owner, repo: context.repo.repo, issue_number: issue.number, name: label}); } catch(e) {}
}

if (result.comment) {
  await github.rest.issues.createComment({owner: context.repo.owner, repo: context.repo.repo, issue_number: issue.number, body: result.comment + "

_\u2014 Vakt Triage Agent_"});
}

if (result.jira_create && process.env.JIRA_API_TOKEN && process.env.JIRA_EMAIL) {
  const jiraRes = await fetch("https://norvikops.atlassian.net/rest/api/3/issue", {
    method: "POST",
    headers: {"Content-Type": "application/json", "Authorization": "Basic " + Buffer.from(process.env.JIRA_EMAIL + ":" + process.env.JIRA_API_TOKEN).toString("base64")},
    body: JSON.stringify({fields: {project: {key: "VAKT"}, summary: (result.jira_summary || issue.title).substring(0, 80), description: {type: "doc", version: 1, content: [{type: "paragraph", content: [{type: "text", text: "GitHub #" + issue.number + ": " + issue.html_url}]}]}, issuetype: {name: result.type === "bug" ? "Bug" : "Story"}, parent: {key: "VAKT-866"}}})
  });
  const jiraData = await jiraRes.json();
  if (jiraData.key) {
    core.info("Jira: " + jiraData.key);
    await github.rest.issues.createComment({owner: context.repo.owner, repo: context.repo.repo, issue_number: issue.number, body: "\uD83D\uDCCB Jira: [" + jiraData.key + "](https://norvikops.atlassian.net/browse/" + jiraData.key + ")"});
  }
}
};
