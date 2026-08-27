# Architecture Decision Records

Sepuluh ADR canonical Labuda. Diurutkan kronologis berdasarkan kapan keputusan dibuat — angka tidak menunjukkan prioritas.

## Authority ADRs (001 — 005)

Keputusan tentang **siapa otoritatif untuk apa** di Labuda.

| # | Judul | Keputusan singkat |
|---|-------|--------------------|
| [001](./001-pricing-token-authority.md) | Pricing Token Authority | Pricing token / snapshot adalah canonical pricing authority. Frontend tidak boleh otoritatif untuk subtotal, fee, discount, coins, shipping. |
| [002](./002-ledger-as-authority.md) | Ledger as Authority | Double-entry ledger adalah canonical financial authority. Wallet adalah display + derived state, bukan otoritas. |
| [003](./003-governance-evaluator.md) | Governance Evaluator | Visibility / governance evaluator adalah otoritas tunggal untuk allow / deny / tombstone / redact. |
| [004](./004-discovery-projection-boundary.md) | Discovery / Projection Boundary | Layered topology: Write Model → Projection → Evaluator → Public Card Boundary → Ranking. Projection bukan otoritas. |
| [005](./005-realtime-signal-not-authority.md) | Realtime Signal, Not Authority | WebSocket / realtime adalah signal-only. Truth tetap di domain DB / REST. |

## Card Family ADRs (006 — 010)

Keputusan exposure shape per entity di public surface.

| # | Judul | Keputusan singkat |
|---|-------|--------------------|
| [006](./006-user-card-family.md) | UserCard Family | Canonical user exposure shape. Email tidak pernah jadi public fallback. |
| [007](./007-seller-card-family.md) | SellerCard Family | Canonical seller exposure shape. Composes UserCard. |
| [008](./008-listing-card-family.md) | ListingCard Family | Canonical listing exposure shape. Distinct from AuctionCard. |
| [009](./009-auction-card-family.md) | AuctionCard Family | Canonical auction exposure shape. Distinct from ListingCard karena lifecycle berbeda. |
| [010](./010-content-card-family.md) | ContentCard Family | Canonical content (post / comment) exposure shape. Embeds ListingCard / AuctionCard untuk commerce share. |

## Aturan ADR

- **Status `Accepted`** = canonical, code wajib mengikuti.
- ADR tidak pernah dihapus. Bila keputusan berubah, tulis ADR baru yang **superseding** ADR lama, dan tandai ADR lama dengan status `Superseded by ADR-NNN`.
- Setiap ADR baru dapat nomor berikutnya (011, 012, dst.) — bukan slot yang dilewati.
- Setiap perubahan ADR (misal: status berubah dari Accepted → Superseded) wajib persetujuan owner.
