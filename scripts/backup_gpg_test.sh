#!/usr/bin/env bash
set -euo pipefail

# S114 test: the GPG symmetric encrypt/decrypt roundtrip used by backup.sh +
# backup-verify.sh.
#
# Why this file was rewritten (K2-01, 2026-07-30)
# -----------------------------------------------
# The previous version claimed in its header to test "the exact gpg flags used
# in both scripts" and then hardcoded those flags in a second literal. It never
# opened backup.sh or backup-verify.sh. Copied into an EMPTY directory it still
# printed "ALL TESTS PASSED" with rc=0 — it was testing GnuPG, not Vakt. Drop
# `--cipher-algo AES256` from backup.sh and this file stayed green.
#
# That is the "two separate literals" class ci.yml already names for
# INTEGRATION_PKGS ("zwei getrennte Listen haetten genau ESK-2 reproduziert"),
# and SA-03 criterion 5 (a gate that is green over an empty tree).
#
# What it does now: it EXTRACTS the gpg invocation out of the two production
# scripts, asserts the required flags on the extracted text, and then runs the
# roundtrip by evaluating the EXTRACTED command itself — so there is exactly one
# literal, and it lives in the production script. A flag that disappears there
# disappears from this test's run too, and the assertion names it.
#
# Its denominator — what this test does NOT look at:
#   * Only the two scripts below. Other gpg users (none today) are not covered;
#     the call-site ratchet at the bottom fails if a third one appears.
#   * It does not test pg_dump, the archive format, the HMAC signature or the
#     retention (restore_test.sh / backup_cron_test.sh own those).
#   * It runs against a synthetic plaintext, not a real dump — the flags and the
#     roundtrip are the subject, not the payload.
#   * REACHABILITY, and this is the sharp edge (R-04, 2026-07-30). unfold() only
#     solved the COMMENTED-OUT form of "still there, but no longer effective";
#     the same call switched off IN PLACE was still extracted, still evaluated
#     here, and still reported as a pass — while the backup shipped plaintext.
#     Check 1b now refuses a call inside a CONSTANT-false guard (`if false`,
#     `while false`, `if [ 1 -eq 0 ]`, `if ! true`, nested). Everything else on
#     that axis remains UNSEEN, named rather than implied:
#       - a guard with a REAL condition that happens to be false at runtime
#         (`if [ -n "$SKIP_ENCRYPT" ]`). This test does not execute backup.sh,
#         so it cannot evaluate one — and backup-verify.sh's own
#         `if [ -f "$WORK_DIR/db.pgdump.gpg" ]` shows why flagging real
#         conditions would only get this test deleted.
#       - an `exit`/`return` on the path ABOVE the call, or the call sitting in
#         a function nobody calls: block structure is tracked, control flow is
#         not.
#       - what happens AFTER the call. `cp "$WORK_DIR/db.pgdump"
#         "$WORK_DIR/db.pgdump.gpg"` two lines further down would hand the
#         plaintext over under the encrypted name and this test would not
#         notice: it evaluates the EXTRACTED command in isolation, it does not
#         run backup.sh end to end. Covering that means running the real script
#         against a real dump — restore_test.sh's territory, not this file's.

fail() { echo "FAIL: $1" >&2; exit 1; }
pass() { echo "PASS: $1"; }

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENCRYPT_SRC="$REPO_ROOT/scripts/backup.sh"
DECRYPT_SRC="$REPO_ROOT/scripts/backup-verify.sh"

# A gpg call site this test is expected to cover. Both directions are red:
# fewer means a production encrypt/decrypt path vanished (or moved out of
# reach of the extractor) while this test kept reporting success; more means a
# new gpg invocation appeared that nothing here checks.
GPG_CALLSITE_BASELINE=2

# ── 0: the subject must exist ─────────────────────────────────────────────
# The empty-tree hole: without this, every check below is vacuous and rc=0.
missing=0
for f in "$ENCRYPT_SRC" "$DECRYPT_SRC"; do
	[ -f "$f" ] || { echo "  MISSING: $f" >&2; missing=$((missing + 1)); }
done
[ "$missing" -eq 0 ] || fail "$missing production script(s) not found under $REPO_ROOT — this test has no subject and must not report success"
pass "subject present: scripts/backup.sh + scripts/backup-verify.sh"

# Join backslash-continued lines so a gpg call split over three lines (which is
# how both scripts write it) is one searchable command. Whole-line comments are
# dropped first: without that, commenting the encrypt call OUT would leave the
# text in place, the flag assertions would still find it, and this test would
# report success for encryption that no longer happens — the exact
# "still there, but no longer effective" shape K2-05 is about.
unfold() {
	awk '
		/^[ \t]*#/ { next }
		{
			line = $0
			while (sub(/\\$/, "", line) > 0) {
				if ((getline nxt) <= 0) break
				if (nxt ~ /^[ \t]*#/) continue
				sub(/^[ \t]+/, " ", nxt)
				line = line nxt
			}
			print line
		}' "$1"
}

# A gpg invocation = the word gpg followed by at least one long flag. Excludes
# `command -v gpg`, comments and .gpg filenames.
GPG_CALL_RE='(^|[|;&[:space:]])gpg[[:space:]]+-'

extract_call() { # file, extra-required-substring
	# `|| true`: a no-match grep exits 1, and under `set -euo pipefail` that
	# would abort the whole test right here — rc=1 with NO message naming what
	# is wrong, which is a silent failure dressed as a red gate. The emptiness
	# check at the call site is what reports it, by name.
	unfold "$1" | grep -E "$GPG_CALL_RE" | grep -F -- "$2" | head -1 || true
}

count_calls() { unfold "$1" | grep -cE "$GPG_CALL_RE" || true; }

ENC_CMD="$(extract_call "$ENCRYPT_SRC" '--symmetric')"
DEC_CMD="$(extract_call "$DECRYPT_SRC" '--decrypt')"

[ -n "$ENC_CMD" ] || fail "no gpg --symmetric invocation found in scripts/backup.sh — the dump is no longer encrypted, or the call moved out of reach of this test"
[ -n "$DEC_CMD" ] || fail "no gpg --decrypt invocation found in scripts/backup-verify.sh"
pass "extracted both gpg invocations from the production scripts"

# ── 1: required flags, asserted against the EXTRACTED text ────────────────
# Each flag is load-bearing: --batch/--pinentry-mode loopback/--passphrase-fd
# keep it non-interactive (a prompt in cron is a hung backup), --symmetric +
# --cipher-algo AES256 are the encryption contract, --decrypt/--output are the
# recovery path.
assert_flags() { # label, command-text, flags...
	local label="$1" cmd="$2"; shift 2
	local flag missing_flags=()
	for flag in "$@"; do
		case "$cmd" in
			*"$flag"*) ;;
			*) missing_flags+=("$flag") ;;
		esac
	done
	if [ ${#missing_flags[@]} -gt 0 ]; then
		echo "  in $label:" >&2
		echo "    $cmd" >&2
		fail "$label lost required gpg flag(s): ${missing_flags[*]}"
	fi
	pass "$label carries all $# required gpg flag(s)"
}

assert_flags "scripts/backup.sh (encrypt)" "$ENC_CMD" \
	--batch --yes --passphrase-fd --pinentry-mode "loopback" \
	--symmetric --cipher-algo "AES256" --output
assert_flags "scripts/backup-verify.sh (decrypt)" "$DEC_CMD" \
	--batch --yes --passphrase-fd --pinentry-mode "loopback" \
	--decrypt --output

# ── 1b: the extracted call must be REACHABLE, not just present (R-04) ─────
# unfold() solves the COMMENTED-OUT form of "still there, but no longer
# effective". R-04: the same call can be switched off IN PLACE, and then the
# extraction above reads dead text, the eval below encrypts happily, and the
# backup ships plaintext:
#
#     if false; then                      # encryption switched off in place
#     printf '%s' "$PASSPHRASE" | gpg --batch … --symmetric …
#     fi
#     cp "$WORK_DIR/db.pgdump" "$WORK_DIR/db.pgdump.gpg"
#
# So: track block structure and refuse to certify a call that sits inside a
# CONSTANT-false guard. Constant is the operative word and the limit — see the
# denominator at the top. A guard with a real condition (backup-verify.sh's
# `if [ -f "$WORK_DIR/db.pgdump.gpg" ]`) is normal engineering and is NOT an
# error here; this test cannot evaluate it, and a test that flagged it would be
# deleted within a month.
guard_state() { # file, grep-regex -> "REACHABLE" | "DEAD <cond>"
	unfold "$1" | awk -v re="$2" '
		function trim(s) { sub(/^[ \t]+/, "", s); sub(/[ \t]+$/, "", s); return s }
		{
			# Struktur wird am Code entschieden, nicht am Zeilen-Kommentar
			# dahinter: `if false; then   # abgeschaltet` ist derselbe Fall wie
			# `if false; then`, und genau so wurde er in der Probe geschrieben.
			line = trim($0)
			sub(/[ \t]+#.*$/, "", line)
			line = trim(line)
			if (line ~ /^(if|while|until)[ \t]/) {
				cond = line
				sub(/^(if|while|until)[ \t]+/, "", cond)
				sub(/[ \t]*;[ \t]*(then|do)([ \t]*;[ \t]*.*)?$/, "", cond)
				cond = trim(cond)
				dead = (cond ~ /^(false|!([ \t]*)true|\[\[?[ \t]*(false|0[ \t]*-eq[ \t]*1|1[ \t]*-eq[ \t]*0|0[ \t]*=[ \t]*1|1[ \t]*=[ \t]*0)[ \t]*\]\]?)$/)
				depth++; frame[depth] = dead; framecond[depth] = cond
				# einzeiliges `if false; then x; fi` schliesst sich selbst
				if (line ~ /(^|[ \t;])(fi|done)[ \t]*$/) depth--
				next
			}
			if (line ~ /^(case|for|select)[ \t]/)  { depth++; frame[depth] = 0; next }
			if (line ~ /^(elif|else)([ \t]|$)/)    { if (depth > 0) frame[depth] = 0; next }
			if (line ~ /^(fi|done|esac)([ \t;]|$)/){ if (depth > 0) depth--; next }
			if ($0 ~ re) {
				for (i = 1; i <= depth; i++)
					if (frame[i]) { print "DEAD " framecond[i]; found = 1; exit }
			}
		}
		END { if (!found) print "REACHABLE" }
	'
}

for pair in "$ENCRYPT_SRC|--symmetric|scripts/backup.sh (encrypt)" \
            "$DECRYPT_SRC|--decrypt|scripts/backup-verify.sh (decrypt)"; do
	src="${pair%%|*}"; rest="${pair#*|}"; needle="${rest%%|*}"; label="${rest#*|}"
	state="$(guard_state "$src" "${GPG_CALL_RE}.*${needle}")"
	case "$state" in
		REACHABLE) ;;
		DEAD*) fail "$label: the gpg call is inside a constant-false guard (\`${state#DEAD }\`) — it is still in the file, so the flag assertions above pass, but it NEVER RUNS. The backup would ship whatever else writes the .gpg path." ;;
		*) fail "$label: could not determine reachability of the gpg call (guard_state said '$state') — this test must not certify what it did not read" ;;
	esac
done
pass "both gpg call sites are reachable (no constant-false guard around them)"

# ── 2: run the EXTRACTED commands (one literal, and it is the repo's) ─────
# Guard before eval: the extracted text must look like the pipe-into-gpg command
# we expect and nothing else. A grep that drifts onto another line must not turn
# this test into an arbitrary-command runner.
guard_evalable() { # label, command-text
	local label="$1" cmd="$2"
	case "$cmd" in
		*'`'* | *'$('* | *';'* | *'&'* | *'>'* | *'<'*)
			fail "$label: extracted command contains shell control characters — refusing to evaluate: $cmd" ;;
	esac
	case "$cmd" in
		*"| gpg "*) ;;
		*) fail "$label: extracted command is not a 'printf | gpg' pipeline — refusing to evaluate: $cmd" ;;
	esac
}
guard_evalable "encrypt" "$ENC_CMD"
guard_evalable "decrypt" "$DEC_CMD"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# Zur Laufzeit erzeugt statt als Literal committet (ESK-7). Einmal wuerfeln:
# der Encrypt-Aufruf verschluesselt damit, der Decrypt-Aufruf entschluesselt damit.
PASSPHRASE="s114-$(openssl rand -hex 12)"
PLAINTEXT="fake-db-dump-content-for-test"

# The variable names the production scripts use for their paths. Setting them
# here is what lets the extracted command run unmodified.
WORK_DIR="$WORK"
DUMP_TMP="$WORK/db.pgdump.dec"
printf '%s' "$PLAINTEXT" >"$WORK_DIR/db.pgdump"

eval "$ENC_CMD" 2>/dev/null
[ -f "$WORK_DIR/db.pgdump.gpg" ] || fail "backup.sh's own encrypt command produced no .gpg file"
pass "backup.sh's encrypt command produces a .gpg file"

# The file must actually be AES256-symmetric-encrypted, not merely present — a
# --symmetric that silently degraded (or an --output of the plaintext) would
# otherwise pass the existence check above.
# `gpg --list-packets` exits non-zero on a file it cannot decrypt, so its exit
# code is not the signal here — its output is. (Under `set -o pipefail` a direct
# `gpg | grep` would have failed on gpg's rc, never on the missing packet.)
PACKETS="$(gpg --batch --list-packets "$WORK_DIR/db.pgdump.gpg" 2>/dev/null || true)"
case "$PACKETS" in
	*"symkey enc packet"*) ;;
	*) fail "output of backup.sh's encrypt command is not a GPG symmetric-key packet — --symmetric no longer takes effect" ;;
esac
if grep -qF "$PLAINTEXT" "$WORK_DIR/db.pgdump.gpg"; then
	fail "plaintext survives verbatim in the encrypted file — encryption did not happen"
fi
pass "encrypted output is a GPG symmetric-key packet ($(printf '%s' "$PACKETS" | sed -n 's/.*:symkey enc packet: \(.*\)/\1/p' | head -1)), plaintext not recoverable verbatim"

rm "$WORK_DIR/db.pgdump" # backup.sh removes the original at its next line

eval "$DEC_CMD" 2>/dev/null
RESULT=$(cat "$DUMP_TMP")
[ "$RESULT" = "$PLAINTEXT" ] || fail "backup-verify.sh's decrypt command did not recover the original content: got '$RESULT'"
pass "backup-verify.sh's decrypt command recovers the original content"

# ── 3: wrong passphrase fails (the extracted decrypt call, wrong secret) ──
set +e
(
	# shellcheck disable=SC2034  # von $DEC_CMD im eval unten gelesen — shellcheck
	# kann nicht durch ein eval hindurchsehen. Die Variable entfernen wuerde diesen
	# Testfall entwerten: er prueft gerade, dass die FALSCHE Passphrase scheitert.
	PASSPHRASE="wrong-passphrase"
	DUMP_TMP="$WORK/bad.dec"
	eval "$DEC_CMD" 2>/dev/null
)
RC=$?
set -e
[ "$RC" -ne 0 ] || fail "wrong passphrase should fail"
pass "wrong passphrase correctly rejected"

# ── 4: call-site ratchet ──────────────────────────────────────────────────
ENC_N=$(count_calls "$ENCRYPT_SRC")
DEC_N=$(count_calls "$DECRYPT_SRC")
TOTAL=$((ENC_N + DEC_N))
if [ "$TOTAL" -lt "$GPG_CALLSITE_BASELINE" ]; then
	fail "only $TOTAL gpg call site(s) left in the two backup scripts, baseline is $GPG_CALLSITE_BASELINE — an encrypt/decrypt path disappeared and this test would have stayed green"
fi
if [ "$TOTAL" -gt "$GPG_CALLSITE_BASELINE" ]; then
	unfold "$ENCRYPT_SRC" | grep -nE "$GPG_CALL_RE" | sed 's/^/    backup.sh:/' >&2 || true
	unfold "$DECRYPT_SRC" | grep -nE "$GPG_CALL_RE" | sed 's/^/    backup-verify.sh:/' >&2 || true
	fail "$TOTAL gpg call site(s) found, this test covers $GPG_CALLSITE_BASELINE — a new gpg invocation is UNCHECKED. Extend the extraction above and raise GPG_CALLSITE_BASELINE."
fi
pass "gpg call sites: $TOTAL checked, 0 unchecked (baseline $GPG_CALLSITE_BASELINE)"

echo "ALL TESTS PASSED — checked: $TOTAL gpg call site(s) read from the repo, unchecked: 0, skipped: 0"
# Wo dieser Test wegschaut — gedruckt, nicht nur im Kopf (R-04). Die
# Erreichbarkeitspruefung oben deckt KONSTANT-falsche Guards; alles andere auf
# dieser Achse steht hier, damit "ALL TESTS PASSED" nicht mehr behauptet, als
# gemessen wurde.
cat <<'DENOM'
   blind by design  : (1) ein Guard mit ECHTER Bedingung, die zur Laufzeit falsch
                      ist — dieser Test fuehrt backup.sh nicht aus. (2) ein
                      exit/return oberhalb des Aufrufs oder ein Aufruf in einer
                      Funktion, die niemand ruft: Blockstruktur wird verfolgt,
                      Kontrollfluss nicht. (3) was NACH dem Aufruf passiert — ein
                      `cp db.pgdump db.pgdump.gpg` zwei Zeilen weiter liefert
                      Klartext unter dem verschluesselten Namen aus, und dieser
                      Test wertet den EXTRAHIERTEN Befehl isoliert aus.
                      (4) pg_dump, Archivformat, HMAC, Retention (restore_test.sh
                      und backup_cron_test.sh besitzen die).
DENOM
