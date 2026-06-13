module.exports = async ({github, context, core}) => {
  const comment = context.payload.comment;
  const issue = context.payload.issue;

  // Bot-Loop verhindern
  if (comment.user.type === "Bot" || comment.user.login === "github-actions[bot]") {
    core.info("Bot-Kommentar, skip.");
    return;
  }

  const currentLabels = issue.labels.map(l => l.name);
  const needsInfo = currentLabels.includes("status: needs-info");

  const systemPrompt = [
    "Du bist der Triage-Agent fuer Vakt, eine selbst gehostete ISMS-Plattform fuer den DACH-Mittelstand.",
    "Ein User hat auf ein GitHub Issue geantwortet. Analysiere ob die Antwort die fehlenden Informationen liefert.",
    "Antworte AUSSCHLIESSLICH mit einem JSON-Objekt ohne Markdown:",
    '{"info_complete":true,"labels_add":[],"labels_remove":[],"comment":"Antwort auf Deutsch oder null"}',
    "Regeln:",
    "- info_complete: true wenn die Antwort genuegend Infos fuer Reproduktion/Bearbeitung liefert",
    "- labels_add: 'status: confirmed' wenn info_complete, sonst nichts",
    "- labels_remove: 'status: needs-info' wenn info_complete",
    "- comment: Kurze Bestaetigung auf Deutsch wenn info_complete (z.B. 'Danke, wir haben alle Infos und schauen uns das an.'). Wenn immer noch Infos fehlen: gezielt nachfragen was noch fehlt. null wenn nichts zu sagen."
  ].join("\n");

  const context_text = "Issue: " + issue.title + "\n\nOriginal Issue:\n" + (issue.body || "") + "\n\nNeuer Kommentar von " + comment.user.login + ":\n" + comment.body + "\n\nAktuelle Labels: " + currentLabels.join(", ");

  const res = await fetch("https://api.anthropic.com/v1/messages", {
    method: "POST",
    headers: {"Content-Type": "application/json", "x-api-key": process.env.ANTHROPIC_API_KEY, "anthropic-version": "2023-06-01"},
    body: JSON.stringify({
      model: "claude-haiku-4-5-20251001", max_tokens: 512,
      system: systemPrompt,
      messages: [{role: "user", content: context_text}]
    })
  });

  const data = await res.json();
  let result;
  try {
    const m = data.content[0].text.trim().match(/\{[\s\S]*\}/);
    result = JSON.parse(m ? m[0] : data.content[0].text.trim());
  } catch(e) { core.error("Parse Error: " + data.content[0].text); return; }

  core.info("Comment triage: " + JSON.stringify(result));

  if (result.labels_add && result.labels_add.length) {
    await github.rest.issues.addLabels({owner: context.repo.owner, repo: context.repo.repo, issue_number: issue.number, labels: result.labels_add});
  }
  for (const label of (result.labels_remove || [])) {
    try { await github.rest.issues.removeLabel({owner: context.repo.owner, repo: context.repo.repo, issue_number: issue.number, name: label}); } catch(e) {}
  }

  if (result.comment) {
    await github.rest.issues.createComment({
      owner: context.repo.owner, repo: context.repo.repo, issue_number: issue.number,
      body: result.comment + "\n\n_\u2014 Vakt Team_"
    });
  }
};
