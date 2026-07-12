// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

/**
 * Where a customer buys Vakt Pro.
 *
 * This pointed at a Polar.sh checkout until 2026-07-12. Polar sells as Merchant
 * of Record in its own name and therefore adds VAT on top — so this button quoted
 * EUR 3,558.10 while the website advertised EUR 2,990 (small-business rule, no
 * VAT). Two prices for the same product, and the higher one was the one inside
 * the product, where nobody was looking.
 *
 * Sales now run through the quote form, which issues a Lexware invoice.
 */
export const VAKT_PRO_CHECKOUT_URL = 'https://vakt.norvikops.de/angebot'

/** Same page — the form asks for the billing interval. */
export const VAKT_PRO_ANNUAL_URL = VAKT_PRO_CHECKOUT_URL

/**
 * Where a customer renews.
 *
 * There is no self-service portal any more: a renewal is simply a new invoice, so
 * it goes through the same form. Kept as its own constant because "buy" and
 * "renew" are different intents at the call sites and may well diverge again.
 */
export const VAKT_LICENSE_RENEW_URL = 'https://vakt.norvikops.de/angebot'
