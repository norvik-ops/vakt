module.exports = async ({github, context, core}) => {
  const payload = context.payload.client_payload;
  const jiraKey = payload.jira_key;

  if (!jiraKey) { core.error("Kein jira_key im Payload"); return; }
  core.info("Jira-Issue erledigt: " + jiraKey);

  // Jira-Issue holen um GitHub-Issue-Nummer zu extrahieren
  const jiraRes = await fetch("https://norvikops.atlassian.net/rest/api/3/issue/" + jiraKey + "?fields=description,summary,fixVersions", {
    headers: {
      "Authorization": "Basic " + Buffer.from(process.env.JIRA_EMAIL + ":" + process.env.JIRA_API_TOKEN).toString("base64"),
      "Accept": "application/json"
    }
  });
  const jiraData = await jiraRes.json();
  core.info("Jira: " + JSON.stringify(jiraData.fields));

  // GitHub Issue-Nummer aus der Beschreibung extrahieren
  let githubIssueNumber = null;
  try {
    const descContent = JSON.stringify(jiraData.fields.description);
    const match = descContent.match(/GitHub #(\d+)/);
    if (match) githubIssueNumber = parseInt(match[1]);
  } catch(e) { core.warning("Konnte GitHub Issue nicht aus Beschreibung lesen: " + e.message); }

  if (!githubIssueNumber) {
    core.warning("Keine GitHub Issue-Nummer gefunden fuer " + jiraKey);
    return;
  }

  core.info("Schliesse GitHub Issue #" + githubIssueNumber);

  // Fix-Version aus Jira holen falls vorhanden
  const fixVersion = jiraData.fields.fixVersions && jiraData.fields.fixVersions.length > 0
    ? jiraData.fields.fixVersions[0].name
    : null;

  // Kommentar auf GitHub
  const commentBody = fixVersion
    ? "Dieser Bug wurde behoben und ist in **" + fixVersion + "** enthalten. Bitte aktualisiere deine Vakt-Installation und melde dich falls das Problem weiterhin besteht.\n\n_\u2014 Vakt Team_"
    : "Dieser Bug wurde behoben. Das Fix wird mit dem naechsten Release ausgerollt. Bitte aktualisiere deine Vakt-Installation sobald das Update verfuegbar ist und melde dich falls das Problem weiterhin besteht.\n\n_\u2014 Vakt Team_";

  await github.rest.issues.createComment({
    owner: context.repo.owner, repo: context.repo.repo,
    issue_number: githubIssueNumber,
    body: commentBody
  });

  // Issue schliessen
  await github.rest.issues.update({
    owner: context.repo.owner, repo: context.repo.repo,
    issue_number: githubIssueNumber,
    state: "closed",
    state_reason: "completed"
  });

  core.info("GitHub Issue #" + githubIssueNumber + " geschlossen.");
};
