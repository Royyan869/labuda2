# Business Flows

Dokumentasi business flow Labuda — siapa actor, apa yang mereka lakukan, dan apa dampaknya ke domain lain.

> **Tujuan folder ini:** menjadi pegangan tentang **flow bisnis canonical** — bukan deskripsi implementasi mobile/backend, bukan API contract.
> Bila ada drift antara mobile, backend, dan dokumen ini → dokumen ini yang menjadi referensi truth.

---

## Cakupan

Folder ini menjawab:

> *Apa yang dilakukan aplikasi ini, untuk siapa, dan dengan akibat apa?*

Folder ini **bukan**:

- Dokumentasi API.
- Dokumentasi schema database.
- ADR (lihat [`../adr/`](../adr/) — ADR-001..010).
- Architecture overview (lihat [`../architecture.md`](../architecture.md)).

---

## Isi

### Peta domain

| File | Isi |
|------|-----|
| [`domain-map.md`](./domain-map.md) | Peta business domain + canonical truth per domain |
| [`actor-map.md`](./actor-map.md) | Peta actor (Guest, Buyer, Seller, dst) + lifecycle layered identity |
| [`cross-domain-relations.md`](./cross-domain-relations.md) | Rantai event antar domain |

### Per-domain flow

- [`foundation/`](./foundation/) — Identity, Profile, Address, Seller Capability. 11 flow (sign-up, sign-in-email, sign-in-google, email-verification, complete-profile, manage-profile, manage-address-book, manage-preferences, become-seller, submit-seller-verification, seller-verification-review).
- [`doctrine/`](./doctrine/) — 8 doctrine canonical (layered identity, email gating, username lifecycle, capability matrix, revocable trust, seller authority separation, trust escalation safety, verification review governance).

---

## Konvensi Penulisan Flow

Setiap flow file punya struktur:

```
# [Flow Name]

> Status: STABLE | DRAFT
> Domain: [Domain]
> Last reviewed: [tanggal]

## Purpose
[1–2 paragraf: untuk siapa flow ini, kenapa ada]

## Actors
[Daftar actor yang terlibat]

## Preconditions
[Apa yang harus benar sebelum flow dimulai]

## Main Flow
[3–7 langkah owner-readable]

## Alternate Flows
[Variasi flow]

## Failure / Rejection Cases
[Setiap penyebab penolakan didaftar; tidak boleh "dll"]

## State Changes
[State apa berubah, dari nilai apa ke nilai apa]

## Business Rules
[Aturan validasi, format, konstanta]

## Doctrine references
[Link ke ../doctrine/ files yang mengikat flow ini]
```

Panduan kuantitatif:

- Satu flow ≈ 20–80 baris markdown.
- Main Flow = 3–7 langkah owner-readable.
- Failure / Rejection Cases = setiap penyebab didaftar; tidak boleh "dll".

---

## Konvensi Status

| Status | Arti |
|--------|------|
| **CANONICAL** | Doctrine docs (di [`./doctrine/`](./doctrine/)) — invariant resmi. Mengalahkan flow docs jika konflik. |
| **STABLE** | Flow doc disetujui owner sebagai canonical. Code wajib mengikuti. |
| **DRAFT** | Belum disetujui owner. Boleh dipakai sebagai diskusi, tidak boleh dijadikan acuan implementasi. |

Status seperti "POSSIBLE DRIFT" / "POSSIBLE LEGACY" tidak dipakai di folder ini — itu artefak audit, bukan canonical truth.

---

## Aturan Penambahan Flow Baru

1. Cek `domain-map.md` — flow ini masuk domain mana.
2. Cek apakah ada doctrine yang sudah mengikat (di [`./doctrine/`](./doctrine/)).
3. Tulis dengan template di atas, status awal `DRAFT`.
4. Owner / PM review → status berubah ke `STABLE`.
5. Update `cross-domain-relations.md` kalau flow ini memicu event ke domain lain.

> Bila menulis flow yang menyentuh uang (escrow, refund, payout, dispute), wajib ikut [Money Flow](../guide.md#money-flow) — buyer wallet **bukan** money authority, gateway adalah otoritas uang nyata, ledger adalah canonical settlement record.

---

## Dokumen Pendukung

- [`../foundation.md`](../foundation.md) — business truth + canonical authority + domain boundaries.
- [`../architecture.md`](../architecture.md) — arsitektur teknis lima area.
- [`../adr/`](../adr/) — ADR-001..010 (authority + card families).
- [`../glossary.md`](../glossary.md) — istilah teknis Labuda.
