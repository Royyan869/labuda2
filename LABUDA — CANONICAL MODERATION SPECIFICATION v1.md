# LABUDA — CANONICAL MODERATION SPECIFICATION v1

## 1. Tujuan

Labuda memiliki sistem governance/moderation yang sederhana, deterministic, dan scalable secukupnya.

Model canonical:

**Report → Case → Decision → Enforcement**

Supporting governance:

**Warning, Appeal, Audit History**

Tidak ada kewajiban membuat entity `Violation`, `Strike`, `Penalty`, atau subsystem moderation lain hanya karena istilah tersebut umum di industri.

---

# 2. Domain Boundary

## 2.1 Report

Report adalah **allegation/input dari seorang user**.

Makna:

> "Saya melaporkan subject ini karena alasan tersebut."

Report tidak berarti pelanggaran sudah terbukti.

Report memiliki:

- reporter
- subject
- reason
- waktu dibuat

Report tidak memiliki authority untuk mengubah target.

### Authority

**Report domain** hanya menyimpan fakta bahwa laporan dibuat.

---

# 3. Case

Case adalah **unit moderation terhadap satu subject**.

Case bukan report.

Satu Case dapat memiliki banyak Report.

Contoh:

```text
Case #C1
Subject: For Sale #F1

Reports:
- R1 — reporter A
- R2 — reporter B
- R3 — reporter C
```

## Case grouping

Untuk v1:

**satu subject memiliki maksimal satu active Case.**

Identity subject:

```text
subject_type + subject_id
```

Tidak menggunakan algoritma correlation kompleks.

Jika Case sudah terminal dan kemudian diperlukan moderation baru, report baru dapat membentuk Case baru.

Case tidak mengambil alih authority target.

---

# 4. Case Lifecycle

Case lifecycle harus sederhana:

```text
open
  ↓
resolved
```

Detail status internal boleh disesuaikan saat implementation design selama tidak mencampurkan decision dengan enforcement.

Prinsip:

- Case terbuka selama moderation belum menghasilkan final outcome.
- Case dapat selesai tanpa enforcement.
- Case selesai ketika moderation decision telah final.
- Kegagalan enforcement tidak membuat Case kembali menjadi pending.
- Retry dilakukan pada Enforcement, bukan membuka kembali Case.

Case history harus dapat direkonstruksi melalui Decision dan governance audit history.

---

# 5. Decision

Decision adalah **keputusan moderation terhadap Case**.

Decision tidak sama dengan Case status.

Minimum outcome:

```text
no_violation
violation
```

## no_violation

Artinya allegation tidak menghasilkan moderation violation/consequence.

Tidak ada enforcement yang diperlukan.

## violation

Artinya moderation memutuskan bahwa terdapat pelanggaran dan consequence dapat diterapkan.

Decision menyimpan minimal:

- Case
- outcome
- decision maker
- decision note/reasoning
- timestamp

Decision harus historical.

Jangan overwrite historical decision sehingga hanya keputusan terakhir yang tersisa tanpa history.

---

# 6. Enforcement

Enforcement adalah **pelaksanaan consequence dari Decision**.

Decision:

> "Pelanggaran terbukti."

Enforcement:

> "Consequence tersebut benar-benar diterapkan."

Enforcement memiliki lifecycle sendiri.

Minimum:

```text
pending
   ↓
succeeded
   ↓
failed
```

Retry terhadap enforcement yang gagal tidak membuat Decision baru dan tidak membuka kembali Case.

## Critical invariant

`Case resolved` atau `Decision violation` **tidak boleh dianggap sebagai bukti bahwa target sudah berubah**.

Keberhasilan enforcement hanya dapat dinyatakan setelah target domain berhasil melakukan mutation yang diminta.

---

# 7. Enforcement Authority

Moderation bukan owner dari target domain.

Flow canonical:

```text
Moderation
   ↓
Decision
   ↓
Enforcement
   ↓
Target Domain Authority
```

Target:

### Content

Content domain tetap menjadi authority content state.

### Comment

Comment domain tetap menjadi authority comment state.

### For Sale

For Sale domain tetap menjadi authority listing state.

Moderation tidak mengambil alih:

- order
- payment
- ledger
- settlement
- shipping
- commerce lifecycle

### Auction

Auction domain tetap menjadi authority auction state.

Moderation tidak mengambil alih:

- bids
- winner
- payment
- settlement

### User/Profile

User domain tetap menjadi authority account/user lifecycle.

Profile tidak dibuat menjadi moderation subsystem terpisah.

---

# 8. For Sale / Auction Special Rule

Labuda from zero dan business rule saat ini sederhana.

Jika product/listing sudah checkout dan tidak lagi public:

- moderation tidak membangun machinery khusus untuk membatalkan transaksi;
- moderation tidak mengambil alih commerce settlement;
- rare conflict tidak boleh menyebabkan generic moderation mutation yang merusak commerce state.

Jika suatu hari diperlukan action khusus terhadap commerce state, harus dibuat melalui explicit domain integration.

Jangan membuat moderation langsung melakukan destructive database mutation terhadap commerce.

---

# 9. Warning

Warning dipertahankan sebagai supporting governance concept.

Warning harus mempunyai provenance yang jelas.

Canonical relationship:

```text
Decision
   ↓
Warning
```

Artinya warning tidak boleh muncul tanpa konteks governance yang dapat ditelusuri.

Tidak membuat:

```text
Violation
   ↓
Strike
   ↓
Warning
```

karena Labuda belum membutuhkan model tersebut.

Warning memiliki lifecycle sendiri, termasuk:

- active
- expired/revoked sesuai kebutuhan implementation

History revoke/expiry harus tetap dapat direkonstruksi.

---

# 10. Appeal

Appeal adalah keberatan terhadap moderation outcome.

Canonical relationship:

```text
Decision
   ↓
Appeal
```

Bukan:

```text
Report
   ↓
Appeal
```

Karena Report hanyalah allegation.

User mengajukan appeal terhadap hasil moderation yang benar-benar diputuskan.

Appeal review menghasilkan governance outcome yang historical.

Jika appeal menyebabkan restoration/reversal, reversal tetap harus melalui authority domain target dan tidak boleh sekadar menghapus history decision sebelumnya.

---

# 11. Audit History

Audit history adalah governance infrastructure.

Tujuan:

> memungkinkan kita merekonstruksi apa yang terjadi, siapa yang melakukan, kapan, dan hasilnya.

Minimal harus dapat ditelusuri:

```text
Report created
Case created/updated
Decision made
Enforcement requested
Enforcement succeeded/failed
Warning issued/revoked
Appeal created/reviewed
```

Audit history bukan business authority.

Current-state table tidak boleh menjadi satu-satunya sumber history jika historical reconstruction dibutuhkan.

---

# 12. Target Scope

Canonical moderation target v1:

```text
content
comment
for_sale
auction
user/profile
```

`profile` secara canonical direpresentasikan oleh User/Profile identity yang sudah ada.

`chat_message` tidak menjadi target business baru hanya karena current implementation pernah mendukungnya.

Jika implementation lama masih memilikinya, statusnya harus diaudit dan kemudian diputuskan untuk dipertahankan atau dibunuh.

---

# 13. Authority Rules

Tidak boleh ada duplicate authority.

| Concept | Authority |
|---|---|
| Report | Report persistence/service |
| Case | Case persistence/service |
| Decision | Decision persistence/service |
| Enforcement state | Enforcement persistence/service |
| Target state | Target domain |
| User lifecycle | User domain |
| Warning | Warning governance |
| Appeal | Appeal governance |
| Audit history | Governance audit infrastructure |

Moderation hanya menjadi authority atas **moderation decision dan enforcement orchestration**, bukan state domain target.

---

# 14. Core Invariants

Implementation nantinya wajib menjaga:

### I1 — Report ≠ Case

Satu Report bukan otomatis satu Case.

### I2 — Multiple Reports

Multiple reports terhadap subject yang sama dapat berada pada satu active Case.

### I3 — Case ≠ Decision

Case tidak menyimpan decision hanya sebagai mutable status yang menghilangkan history.

### I4 — Decision ≠ Enforcement

Decision final tidak berarti enforcement berhasil.

### I5 — Enforcement Success

Enforcement hanya `succeeded` setelah target domain mengonfirmasi mutation berhasil.

### I6 — Failure Isolation

Enforcement failure tidak mengubah Decision menjadi failure dan tidak membuat Case kembali pending.

### I7 — Domain Authority

Moderation tidak boleh menjadi direct authority atas target state.

### I8 — Historical Truth

Decision, enforcement outcome, warning, dan appeal harus dapat direkonstruksi.

### I9 — No Fake Concepts

Jangan membuat Violation/Strike/Penalty/Sanction entity tanpa kebutuhan bisnis nyata.

### I10 — No Legacy Authority

Terminology atau implementation lama tidak boleh menjadi authority hanya karena masih ada di codebase.

---

# 15. Minimal Concept Model

Secara konseptual:

```text
┌────────────┐
│   REPORT   │
│ allegation │
└─────┬──────┘
      │
      │ belongs to
      ▼
┌────────────┐
│    CASE    │
│ moderation │
│   subject  │
└─────┬──────┘
      │
      │ decisions
      ▼
┌────────────┐
│  DECISION  │
│  outcome   │
└─────┬──────┘
      │
      │ produces
      ▼
┌──────────────┐
│ ENFORCEMENT  │
│ execution    │
└──────┬───────┘
       │
       ▼
┌────────────────────┐
│ TARGET DOMAIN      │
│ Content            │
│ Comment            │
│ For Sale           │
│ Auction            │
│ User/Profile       │
└────────────────────┘

Decision ─────► Warning
Decision ◄───── Appeal

All governance actions
        ↓
 Audit History
```

---

# 16. What We Explicitly Do NOT Build

Untuk v1 jangan membuat:

- Violation entity
- Strike system
- reputation score
- automated strike escalation
- complex case correlation engine
- investigator assignment system
- multi-level moderation hierarchy
- moderation SLA engine
- generic penalty framework
- moderation-owned commerce state
- duplicate compatibility model
- `GovernanceCase` alias untuk Report
- legacy `fixed_price_sale` alias
- `removed`/`enforced` semantic aliases

Jika kebutuhan bisnis nyata muncul kemudian, konsep baru harus memiliki business justification dan authority yang jelas.

---

# 17. Migration / Cleanup Principle

Karena Labuda from zero:

Current implementation tidak dianggap sacred.

Jika current:

```text
GovernanceCase
```

ternyata mencampur:

```text
Report
Case
Decision
Enforcement
```

maka implementation tersebut boleh dan seharusnya dibongkar total.

Cleanup mencakup seluruh residue:

- entity
- table
- migration
- DTO
- endpoint
- repository
- service
- worker
- event
- admin UI
- mobile UI
- tests
- docs
- comments
- terminology
- compatibility alias

Tidak boleh menyisakan zombie implementation yang dapat kembali menjadi authority.

---

# 18. Implementation Strategy

Implementasi tidak dilakukan sekaligus.

Urutan yang direkomendasikan:

```text
1. Schema/domain foundation
       ↓
2. Report
       ↓
3. Case
       ↓
4. Decision
       ↓
5. Enforcement
       ↓
6. Warning
       ↓
7. Appeal
       ↓
8. Admin workflow
       ↓
9. Mobile workflow
       ↓
10. Target integrations
       ↓
11. Full cleanup
       ↓
12. Regression + runtime proof
```

Setiap tahap harus:

- bounded;
- factual;
- memiliki acceptance criteria;
- diuji;
- diverifikasi;
- baru kemudian lanjut.

Tidak boleh memberi agent satu prompt raksasa untuk mengimplementasikan semuanya.

---

# 19. Definition of Done

Moderation baru dianggap selesai apabila:

1. Report, Case, Decision, dan Enforcement mempunyai boundary jelas.
2. Tidak ada duplicate authority.
3. Multiple reports dapat ditangani secara deterministic.
4. Decision history tidak hilang.
5. Enforcement success/failure dapat dibedakan.
6. Retry tidak menghasilkan duplicate consequence.
7. Target domain tetap menjadi authority.
8. Warning memiliki provenance.
9. Appeal menunjuk governance outcome yang benar.
10. Admin dapat melakukan workflow yang diperlukan.
11. Mobile dapat membuat dan melihat report sesuai kebutuhan.
12. Semua API contract konsisten.
13. Legacy moderation implementation sudah dibersihkan.
14. Legacy terminology tidak lagi menjadi authority.
15. Migration replay berhasil.
16. Backend tests berhasil.
17. Admin tests berhasil.
18. Mobile tests berhasil.
19. Runtime proof berhasil.
20. Negative proof menunjukkan desain lama tidak lagi hidup.

---

# 20. Status

**CANONICAL DESIGN DIRECTION: APPROVED FOR IMPLEMENTATION PLANNING**

Ini adalah desain minimum yang kita gunakan sebagai dasar untuk implementation planning.

Business requirement baru tidak boleh diam-diam dimasukkan ke implementation.

Jika ditemukan ambiguity baru selama implementasi:

**STOP → tandai UNKNOWN → kembali ke Owner/ChatGPT → putuskan → lanjutkan.**