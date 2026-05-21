package secvitals

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// fetchOrgName liest den Organisations-Namen fuer Berichte/PDFs.
//
// Vor S13-18 nutzten 9+ Stellen das Muster
//
//	_ = s.db.QueryRow(ctx, `SELECT name FROM organizations WHERE id=$1::uuid`, orgID).Scan(&orgName)
//
// — DB-Fehler verschwanden lautlos und es entstanden PDFs/Reports mit leerem
// "Organisation: ". Dieser Helper macht den Fall sichtbar (Warn-Log mit
// Korrelations-Feldern), faellt aber bewusst auf einen leeren String zurueck:
// das Report-PDF soll weiter ausliefern, der Operator soll im Log erfahren,
// warum der Org-Name fehlt.
//
// Bewusst ohne Fallback-Konstante, weil der Aufrufer haeufig schon eine
// zero-value-Variable per `var orgName string` deklariert und das Ergebnis
// dort hineinschreiben moechte.
func fetchOrgName(ctx context.Context, db *pgxpool.Pool, orgID string) string {
	if db == nil || orgID == "" {
		return ""
	}
	var name string
	if err := db.QueryRow(ctx,
		`SELECT name FROM organizations WHERE id = $1::uuid`,
		orgID,
	).Scan(&name); err != nil {
		log.Warn().Err(err).
			Str("org_id", orgID).
			Str("module", "secvitals").
			Msg("fetchOrgName: SELECT failed — using empty org name for downstream report")
		return ""
	}
	return name
}
