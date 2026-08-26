//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

// Container images for the integration suite, pinned by digest.
//
// Why (R1-INT-02, 2026-07-30): all three images used to be referenced by a
// MOVING tag — axllent/mailpit:latest, postgres:16-alpine, redis:7-alpine. A
// moving tag means the test environment can change without any commit changing,
// so a green run yesterday says nothing about today, and a red run says nothing
// about the diff. `scripts/check_image_tags.py` did not see any of this: it read
// docker-compose*.yml and helm/ only, never Go sources.
//
// Form is `repo:tag@sha256:…` rather than a bare digest on purpose:
//
//   - the digest is the enforcement — it is the only reference Docker resolves
//     to exactly one manifest, today and in a year;
//   - the tag stays for humans, so a reader can see WHICH postgres this is
//     without asking a registry, and so drift from the version the production
//     compose stack runs stays visible instead of hiding behind a hash.
//
// Docker resolves the digest and ignores the tag when both are present, so the
// tag cannot lie about what gets pulled — at worst it is stale prose.
//
// Note that an exact PATCH tag alone would not have been enough. It is close:
// measured against the Hub tags API on 2026-07-30, official-image patch tags are
// not rebuilt (postgres 16.9-alpine had not been touched since 2025-07-17), so
// `postgres:16.14-alpine` is immutable in practice. "In practice" is an
// observation about upstream's habits, not something this repo can enforce — and
// for the image that actually flaked the habit is provably absent: mailpit's
// `v1.30` and `v1.30.6` share one digest, i.e. the minor tag is itself a moving
// alias that upstream re-points.
//
// Updating: change tag AND digest together, then run
// `python3 scripts/check_image_tags.py` — it verifies the tag still exists in
// the registry and refuses any reference that has lost its digest.
//
// ── OPEN, FIX BEFORE TOUCHING rotate_key_real_test.go OR cmd/rotate-key ──────
//
// `.claude/hooks/guardrails.sh` blocks EVERY Bash call while a staged file
// matches one of its secret patterns, and it has NO allowlist — so the two
// files the repo has already sanctioned in `.secretlintignore`
// (`backend/internal/integration_test/rotate_key_real_test.go` and
// `backend/cmd/rotate-key/rotate_test.go`, both carrying a literal
// `-----BEGIN PRIVATE KEY-----` as encryption test input) cannot be staged
// without the hook firing. There is no `--no-verify` for a PreToolUse hook; the
// only way through is to stage and commit in one invocation, so that nothing is
// staged at the moment the hook samples the index.
//
// That is not a bypass — the guard truthfully found nothing in an empty index —
// but its PURPOSE is not served, and a guard one routinely dances around is
// worse than none: every successor has to learn the same dance, and the next
// person may instead "fix" the fixture (which is what happened once already and
// had to be reverted). The hook needs to honour `.secretlintignore`, or grow its
// own allowlist. The hook is not owned by this work — hence: reported, not
// patched. Fix it before the next change to either of those two files.
const (
	// postgres:16.14-alpine — 16.x matches the major the production compose
	// stack and helm chart deploy; only the pin is stricter here.
	imagePostgres = "postgres:16.14-alpine@sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777"

	// redis:7.4.10-alpine
	imageRedis = "redis:7.4.10-alpine@sha256:e7723ff73d963f5cc6d9c4643ea3d989527a402a319239054e9472a7fb9219a2"

	// axllent/mailpit:v1.30.6 — was `:latest`, the reference that broke
	// TestVaktaware_E2E_MailpitToClickRate's environment guarantee outright.
	imageMailpit = "axllent/mailpit:v1.30.6@sha256:7f33095f80e901f6ad08028f06ca284aa58fe84942be5496008d041d3b9f4d4d"
)
