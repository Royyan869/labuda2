# LABUDA — CANONICAL MODERATION BUSINESS TRUTH v1
## Draft — Design Decision Gate

### Status

**BUSINESS TRUTH DRAFT — BELUM IMPLEMENTASI**

Dokumen ini menggantikan semantics moderation lama apabila disetujui.

PRD bukan sumber kebenaran final. Current filesystem juga bukan otomatis business truth. Business truth ditetapkan berdasarkan keputusan Owner setelah factual audit dan design review.

---

# 1. Tujuan Moderation

Moderation Labuda bertujuan menangani laporan pengguna terhadap:

- Content
- Comment
- For Sale
- Auction
- Profile

Moderation harus mampu:

1. menerima laporan;
2. mengelompokkan laporan yang berkaitan;
3. melakukan review;
4. menyimpan keputusan moderator secara historis;
5. menjalankan consequence melalui domain yang memiliki authority;
6. mengetahui apakah consequence benar-benar berhasil;
7. menyediakan appeal;
8. menyediakan warning bila keputusan memang menghasilkan warning;
9. menjaga audit trail;
10. tidak mengambil alih authority commerce, identity, atau domain target.

Moderation bukan pemilik state bisnis target.

---

# 2. Fundamental Governance Model

Canonical relationship:

```text
Report
   ↓
Case
   ↓
Decision
   ↓
Enforcement
   ↓
Target Domain
```

Dengan:

```text
Report
= user allegation / signal

Case
= moderation investigation/workflow

Decision
= moderator's authoritative decision

Enforcement
= execution of a decision consequence

Target Domain
= authority atas state target
```

Invariant:

> **Report ≠ Case ≠ Decision ≠ Enforcement**

Tidak boleh digabung kembali menjadi satu super-entity.

---

# 3. Report

## 3.1 Report adalah first-class concept

**LOCKED — RECOMMENDED**

Setiap laporan user adalah record tersendiri.

Report minimal memiliki:

- reporter;
- subject type;
- subject id;
- report reason;
- created timestamp;
- lifecycle information yang diperlukan.

Report tidak menyimpan:

- moderator decision;
- enforcement state;
- warning state;
- appeal outcome.

Report adalah fakta:

> "User X melaporkan subject Y karena alasan Z."

---

# 4. Report Cardinality

**LOCKED — RECOMMENDED**

Satu subject dapat memiliki banyak report.

```text
Report A ─┐
Report B ─┼──→ Case X
Report C ─┘
```

Satu report hanya berada pada satu Case.

Untuk Labuda v1:

> **Satu moderation subject memiliki maksimal satu active Case pada satu waktu.**

Kita tidak membuat case-grouping engine kompleks.

Correlation cukup berdasarkan canonical moderation subject:

```text
subject_type + subject_id
```

Tidak berdasarkan reporter.

Dengan demikian dua user yang melaporkan listing yang sama tidak menghasilkan dua case terpisah hanya karena mereka berbeda reporter.

---

# 5. Duplicate Report

Duplicate tidak berarti report harus dibuang.

Rule yang saya rekomendasikan:

> Reporter yang sama tidak dapat membuat duplicate active report terhadap subject yang sama.

Tetapi:

> Reporter berbeda tetap dapat melaporkan subject yang sama.

Report tersebut menjadi evidence/signal tambahan pada Case yang sama.

Tujuannya sederhana:

- mencegah spam report dari reporter yang sama;
- tetap mempertahankan signal dari banyak user;
- tidak membuat moderation queue berisi case duplikat.

---

# 6. Self Report

**OWNER DECISION — RECOMMENDATION: DENY**

User tidak perlu dapat melaporkan:

- content miliknya sendiri;
- comment miliknya sendiri;
- for sale miliknya sendiri;
- auction miliknya sendiri;
- profile miliknya sendiri.

Alasannya sederhana: self-report bukan kebutuhan moderation yang meaningful untuk v1.

Jika suatu hari diperlukan workflow khusus untuk meminta review terhadap objek sendiri, itu lebih baik dibuat sebagai capability tersendiri daripada menyalahgunakan Report.

---

# 7. Case

Case adalah **unit investigasi**, bukan report.

Case memiliki:

- subject;
- collection of Reports;
- workflow state;
- timestamps;
- current operational information yang memang diperlukan.

Case tidak menjadi tempat menyimpan seluruh history decision.

Case juga tidak menjadi authority enforcement.

---

# 8. Case Lifecycle

Case lifecycle dan Decision lifecycle harus dipisahkan.

Secara konseptual:

```text
OPEN
  ↓
UNDER_REVIEW
  ↓
DECIDED / CLOSED
```

Namun kita tidak perlu memaksakan terlalu banyak status.

Untuk v1, lifecycle harus mampu membedakan:

1. Case masih membutuhkan review.
2. Case sudah memiliki final decision.
3. Case sudah selesai secara governance.

**Tidak boleh menggunakan `enforced` sebagai Case state yang berarti enforcement sukses.**

---

# 9. Decision

**LOCKED — RECOMMENDED**

Decision adalah first-class persisted concept.

Setiap keputusan moderator disimpan sebagai historical record.

Decision bersifat append-only secara governance history.

Contoh:

```text
Case #123

Decision #1
Moderator A
Outcome: violation_confirmed
Action: remove_content
Time: T1
```

Kemudian appeal:

```text
Appeal
   ↓
Decision #2
Moderator B
Outcome: reversed
Time: T2
```

Decision #1 tidak diubah menjadi Decision #2.

History harus tetap dapat direkonstruksi.

---

# 10. Decision ≠ Enforcement

Ini invariant kritikal.

```text
Decision
= apa yang diputuskan

Enforcement
= apakah consequence tersebut berhasil diterapkan
```

Maka:

```text
Decision FINAL
Enforcement PENDING
```

adalah state yang valid.

Demikian juga:

```text
Decision FINAL
Enforcement SUCCEEDED
```

atau:

```text
Decision FINAL
Enforcement FAILED
```

`enforced` tidak boleh lagi menjadi proxy:

> "event sudah dibuat."

Audit membuktikan current implementation melakukan hal tersebut.

---

# 11. Enforcement

**LOCKED — RECOMMENDED**

Enforcement adalah first-class persisted execution record.

Enforcement harus dapat menjawab:

- consequence apa;
- target apa;
- berasal dari Decision mana;
- kapan dibuat;
- kapan dieksekusi;
- berapa attempt;
- apakah berhasil;
- jika gagal, kenapa;
- apakah dapat retry.

Minimal lifecycle:

```text
PENDING
   ↓
PROCESSING
   ↓
SUCCEEDED

atau

FAILED
   ↓
RETRY
   ↓
SUCCEEDED / PERMANENT_FAILURE
```

Tidak perlu workflow enterprise yang kompleks.

---

# 12. Outbox

Outbox tetap digunakan.

Tetapi:

> **Outbox bukan domain authority Enforcement.**

Outbox adalah mekanisme reliable delivery.

Canonical relationship:

```text
Decision
   ↓
Enforcement
   ↓
Outbox
   ↓
Worker
   ↓
Target Domain
```

Outbox tidak menggantikan Enforcement record.

Audit membuktikan current architecture menjadikan outbox event sebagai representasi enforcement sehingga execution result tidak durable.

---

# 13. Target Domain Authority

Moderation memutuskan.

Domain target mengeksekusi mutation.

```text
Moderation
    = governance authority

Content Domain
    = content state authority

Comment Domain
    = comment state authority

For Sale Domain
    = listing lifecycle authority

Auction Domain
    = auction/bid lifecycle authority

User Domain
    = user lifecycle authority

Commerce
    = order/payment/ledger authority
```

Moderation tidak boleh langsung memanipulasi database domain lain jika target domain memiliki service/authority yang benar.

---

# 14. Content Moderation

Admin dapat memutuskan content violation.

Consequence dapat berupa:

```text
remove / hide content
```

Target mutation dilakukan oleh Content Domain.

Moderation tidak boleh mengubah `contents.visibility` langsung dari repository moderation.

Deleted content tetap terminal.

Moderation tidak boleh menghidupkan kembali content yang sudah permanently deleted.

---

# 15. Comment Moderation

Admin dapat memutuskan comment violation.

Consequence:

```text
remove comment
```

Comment Domain adalah authority mutation.

Parent content tetap menjadi authority sendiri.

Restoration tidak boleh mengembalikan comment yang deletion-nya bukan berasal dari moderation.

---

# 16. For Sale Moderation

Admin dapat memoderasi For Sale.

Moderation consequence boleh memengaruhi lifecycle listing.

Tetapi:

> **Moderation bukan commerce settlement authority.**

Moderation tidak boleh secara otomatis:

- mengubah order;
- mengubah payment;
- mengubah ledger;
- mengubah seller proceeds;
- membuat refund finansial;
- mengubah settlement.

Audit menunjukkan current `Withdraw` hanya menyentuh lifecycle listing + shipping quote invalidation dan tidak menyentuh order/payment/ledger. Prinsip boundary ini kita pertahankan.

Jika listing sudah:

- sold;
- checkout;
- memiliki order;

moderation tidak boleh secara otomatis merusak commerce state.

Detail consequence untuk listing yang sudah berada pada terminal commerce state akan menjadi policy target-domain, bukan improvisasi moderation.

---

# 17. Auction Moderation

**LOCKED PRINCIPLE**

Moderation boleh menghentikan auction yang melanggar policy.

Tetapi:

> Moderation tidak boleh melakukan settlement/refund/ledger mutation secara langsung.

Auction Domain menjadi authority.

Jika auction memiliki bid:

```text
Moderation Decision
       ↓
Auction enforcement command
       ↓
Auction Domain
       ↓
auction lifecycle consequence
       ↓
bidder consequence
```

Auction Domain bertanggung jawab memastikan state bid/winner tetap konsisten.

Current `CancelForModeration` yang dapat membatalkan auction ber-bid tanpa menyelesaikan consequence bidder adalah desain yang harus dibongkar. Audit 2 mengklasifikasikannya sebagai P1 commerce boundary.

---

# 18. Profile Moderation

Profile report secara business terminology berarti:

> report terhadap user/profile identity.

Canonical target adalah User.

Tidak perlu membuat entity moderation `Profile` terpisah hanya untuk moderation.

Moderation dapat memutuskan account/user enforcement.

Namun:

> User Domain tetap menjadi authority perubahan lifecycle user.

Moderation tidak boleh menjadi owner `users` lifecycle state.

---

# 19. Warning

**LOCKED — RECOMMENDED**

Tidak ada standalone warning dari admin.

Warning adalah consequence dari governance decision.

```text
Decision
   ↓
Warning
```

Warning harus memiliki provenance yang dapat menjawab:

> "Warning ini diterbitkan karena keputusan moderation yang mana?"

Tidak perlu Strike system.

Tidak perlu reputation engine.

Tidak perlu automatic escalation engine.

Untuk v1:

```text
Decision → optional Warning
```

Warning memiliki lifecycle sendiri untuk:

- active;
- expired;
- revoked.

Tetapi provenance governance wajib dipertahankan.

---

# 20. Violation

Tidak perlu membuat `Violation` entity terpisah untuk v1.

Violation adalah **outcome/policy classification dari Decision**, bukan subsystem sendiri.

Contoh:

```text
Decision
outcome = violation_confirmed
policy = prohibited_content
```

Kita tidak perlu:

```text
ViolationService
ViolationRepository
ViolationEngine
```

kecuali kebutuhan bisnis nyata muncul.

---

# 21. Strike

**LOCKED**

Tidak ada Strike system.

Tidak ada:

- strike counter;
- automatic strike escalation;
- strike threshold;
- strike decay.

Jika suatu saat Labuda membutuhkan sistem penalti berulang, desain tersebut dibuat sebagai business capability baru berdasarkan kebutuhan nyata.

Jangan mempertahankan atau menghidupkan implementation lama yang tidak ada.

---

# 22. Reason Taxonomy

Current implementation menggunakan free-form reason. Audit menemukan tidak ada canonical taxonomy/versioning.

Saya merekomendasikan:

```text
Report
→ reason_code

Decision
→ decision/policy code + operator note

Warning
→ policy/reason code + optional note
```

Reason code harus controlled.

Operator note tetap boleh free text.

Namun kita **tidak perlu membuat configurable policy engine** sekarang.

Untuk v1, taxonomy dapat berupa canonical enum/code yang dimiliki backend.

---

# 23. Evidence

Report dapat membawa reference terhadap evidence/context yang relevan.

Namun evidence bukan alasan untuk membuat snapshot system besar.

Rekomendasi v1:

> **Hybrid sederhana.**

Simpan reference terhadap object dan metadata yang diperlukan untuk investigation.

Jika target dapat berubah/deleted, governance history tidak boleh bergantung sepenuhnya pada current live object state.

Detail immutable evidence snapshot akan ditentukan pada design phase setelah melihat kebutuhan aktual per target.

---

# 24. Appeal

Appeal adalah challenge terhadap **Decision**.

Canonical:

```text
Case
 └── Decision
       ↑
     Appeal
```

Bukan:

```text
Appeal → report_id
```

Appeal tidak boleh menyamarkan Case sebagai Report.

Appeal harus memiliki:

- decision;
- appellant;
- submission;
- review state;
- reviewer;
- outcome;
- timestamps.

Appeal reviewer harus independen dari original decision maker jika memungkinkan dalam role/permission model yang sederhana.

---

# 25. Appeal Outcome

Appeal bukan otomatis berarti:

```text
approved → blindly restore
```

Canonical flow:

```text
Appeal
   ↓
Appeal Decision
   ↓
Reversal / new governance decision
   ↓
Enforcement
```

Jika target masih dalam state yang dapat dibalik, target domain melakukan reversal.

Jika target sudah berubah secara bisnis:

- sold;
- ended;
- deleted;
- settlement completed;

appeal tidak boleh secara membabi buta mengembalikan state lama.

Appeal hanya dapat menghasilkan consequence yang valid terhadap current domain state.

---

# 26. Deleted Target

Deleted adalah terminal.

Jika object sudah permanently deleted:

> Moderation tidak boleh restore object tersebut.

Report yang sudah ada tetap menjadi governance history.

Case/Decision tetap dapat direkonstruksi tanpa membutuhkan object untuk kembali hidup.

---

# 27. Hidden / Moderated Target

Moderation state dan privacy state harus tetap berbeda.

`is_hidden` bukan moderation authority.

Moderation tidak boleh menggunakan privacy state sebagai substitute moderation state.

Content visibility tetap memiliki domain authority sendiri.

---

# 28. Audit Trail

Setiap governance mutation penting harus dapat direkonstruksi:

```text
Report created
Case correlated
Decision made
Enforcement requested
Enforcement attempted
Enforcement succeeded/failed
Warning issued
Appeal submitted
Appeal decided
Reversal executed
```

Audit harus reliable.

`LogSafe` best-effort tidak cukup sebagai satu-satunya governance history.

Audit 2 membuktikan current mutation dapat berhasil walaupun audit logging dapat gagal.

Canonical design harus memastikan governance history tidak silently hilang.

---

# 29. Admin Capability

Admin membutuhkan satu Moderation Case workspace, bukan lima subsystem terpisah.

Namun action dan evidence bergantung pada target.

## Content

Admin dapat:

- inspect;
- dismiss;
- enforce removal/hide;
- issue warning.

## Comment

Admin dapat:

- inspect context;
- dismiss;
- remove;
- issue warning.

## For Sale

Admin dapat:

- inspect listing;
- inspect seller;
- inspect commerce-relevant state;
- dismiss;
- enforce listing moderation consequence;
- issue warning.

Tidak boleh langsung mengubah order/payment/ledger.

## Auction

Admin dapat:

- inspect auction;
- inspect bid state;
- inspect winner/highest bidder;
- dismiss;
- enforce auction moderation consequence;
- issue warning.

Auction domain menangani bid consequence.

## Profile

Admin dapat:

- inspect user;
- inspect relevant moderation history;
- dismiss;
- enforce account/user consequence;
- issue warning.

---

# 30. Admin Must Not Pretend Success

UI admin tidak boleh menampilkan:

> "Enforced"

ketika baru:

```text
Decision persisted
+
Outbox created
```

UI harus dapat membedakan:

```text
Decision finalized
Enforcement pending
```

dari:

```text
Enforcement succeeded
```

dan:

```text
Enforcement failed
```

Ini merupakan requirement correctness, bukan sekadar UX.

---

# 31. Chat Message

**LOCKED — OUT OF CANONICAL V1 SCOPE**

Target moderation yang canonical:

- content;
- comment;
- for_sale;
- auction;
- user/profile.

`chat_message` tidak dipertahankan hanya karena current implementation memilikinya.

Jika kebutuhan bisnis nyata muncul kemudian:

> buat ulang dari business requirement → canonical design.

Jangan menghidupkan implementation lama.

---

# 32. Case Correlation

Kita tidak membuat AI/grouping system.

Canonical correlation v1:

```text
subject_type + subject_id
```

Contoh:

```text
content:abc
```

Semua active reports terhadap content tersebut masuk ke active Case yang sama.

Case baru dapat dibuat ketika case sebelumnya sudah terminal sesuai lifecycle policy.

---

# 33. Concurrency

Canonical invariant:

> Untuk satu subject hanya boleh ada satu active Case.

Database harus menjadi enforcement terakhir terhadap invariant ini.

Application check saja tidak cukup.

Concurrency proof wajib dilakukan setelah implementasi.

---

# 34. Enforcement Idempotency

Setiap Enforcement harus memiliki identity yang memungkinkan repeated delivery aman.

Worker dapat menerima event yang sama lebih dari sekali.

Target Domain harus tetap menghasilkan state yang benar.

Tidak boleh:

```text
retry
→ duplicate financial action
```

atau:

```text
retry
→ duplicate warning
```

atau:

```text
retry
→ invalid target mutation
```

---

# 35. Commerce Safety

Moderation tidak boleh menjadi jalan belakang untuk:

- payment mutation;
- ledger mutation;
- settlement mutation;
- arbitrary refund;
- arbitrary coin mutation.

Jika moderation menyebabkan consequence commerce:

```text
Moderation
→ domain command
→ Commerce/Auction domain
```

Domain tersebut menentukan consequence yang valid.

---

# 36. Restoration Principle

Restoration hanya boleh membalikkan state yang:

1. memang disebabkan oleh moderation;
2. masih reversible;
3. masih valid terhadap current business state.

Tidak boleh:

```text
Appeal approved
→ blindly restore
```

dan tidak boleh menghapus bukti bahwa moderation pernah terjadi.

---

# 37. No Compatibility Layer

Karena Labuda from zero dan belum memiliki real production data:

Jika canonical design berbeda dari implementation lama:

> **bongkar implementation lama.**

Tidak boleh:

- alias;
- backward-compatible endpoint;
- duplicate field;
- duplicate authority;
- legacy enum;
- wrapper hanya demi old consumer;
- old schema writer;
- old test contract;
- parked implementation.

Current implementation yang bertentangan akan dihapus setelah canonical replacement terbukti.

---

# 38. Target Architecture

Secara konseptual:

```text
                    ┌──────────────┐
                    │    Report    │
                    └──────┬───────┘
                           │
                    many reports
                           │
                           ▼
                    ┌──────────────┐
                    │     Case     │
                    └──────┬───────┘
                           │
                      decisions
                           │
                           ▼
                    ┌──────────────┐
                    │   Decision   │
                    └──────┬───────┘
                           │
                     consequences
                           │
                           ▼
                    ┌──────────────┐
                    │ Enforcement  │
                    └──────┬───────┘
                           │
                         outbox
                           │
                           ▼
                    ┌──────────────┐
                    │    Worker    │
                    └──────┬───────┘
                           │
          ┌────────────────┼────────────────┐
          ▼                ▼                ▼
       Content          Commerce          User
       Domain            Domain           Domain
          │
       Comment
       Domain
          │
       Auction
       Domain
```

Moderation orchestrates governance.

Domain masing-masing memiliki state authority.

---

# 39. Things We Explicitly Do NOT Build

Untuk menjaga sistem tetap sederhana:

- no Strike system;
- no reputation engine;
- no ML moderation;
- no automatic severity engine;
- no investigator assignment;
- no SLA engine;
- no case merge/split engine;
- no generic policy scripting engine;
- no moderation microservice;
- no duplicate moderation subsystem;
- no DomainAction resurrection;
- no standalone violation subsystem;
- no chat_message moderation pada v1.

---

# 40. Canonical Invariants

Setelah business truth ini disetujui, invariant berikut menjadi acceptance criteria:

### I1
Report dan Case adalah entity berbeda.

### I2
Multiple Reports dapat berada pada satu Case.

### I3
Decision adalah historical persisted record.

### I4
Decision tidak sama dengan Enforcement.

### I5
Enforcement memiliki durable execution state.

### I6
Case tidak boleh menjadi bukti enforcement success.

### I7
Warning memiliki governance provenance.

### I8
Appeal menunjuk Decision.

### I9
Target domain tetap menjadi authority target state.

### I10
Moderation tidak mengambil alih order/payment/ledger.

### I11
Auction moderation tidak boleh meninggalkan bid/settlement state inconsistent.

### I12
Deleted target tetap terminal.

### I13
Repeated enforcement delivery aman/idempotent.

### I14
Governance mutation memiliki durable audit history.

### I15
Tidak ada competing moderation authority.

---

# 41. Business Decisions Still Required

Berikut yang saya sengaja belum mengunci tanpa keputusan Owner:

1. **Case reopening**
   - apakah case terminal boleh dibuka kembali oleh report baru?
   - atau selalu membuat Case baru setelah terminal?

2. **Report terhadap sold/ended object**
   - Audit membuktikan saat ini for-sale/auction yang sudah sold/ended/cancelled masih dapat direport.
   - Saya cenderung mempertahankannya karena report dapat datang setelah state berubah.

3. **Auction dengan bid**
   - Saya merekomendasikan moderation tetap boleh menghentikan auction yang melanggar.
   - Auction Domain wajib menangani bidder consequence.
   - Tidak ada direct moderation → payment/ledger mutation.

4. **Warning policy**
   - apakah warning boleh berulang untuk decision berbeda?
   - apakah ada cooldown/cap?
   - Saya menyarankan **jangan membuat cap kompleks untuk v1**.

5. **Reason taxonomy**
   - daftar reason code final.
   - Saya menyarankan controlled code + free-form operator note.

6. **Evidence retention**
   - seberapa lama evidence/report history dipertahankan.
   - Saya menyarankan governance history tidak bergantung pada live object.

7. **Appeal eligibility**
   - apakah setiap Decision tertentu dapat di-appeal?
   - atau hanya enforcement decision?

---

# 42. Recommended Default Decisions

Jika kita ingin menjaga kompleksitas rendah, default yang saya rekomendasikan adalah:

| Decision | Recommendation |
|---|---|
| Report first-class | YES |
| Multiple reports → Case | YES |
| One active Case / subject | YES |
| Self-report | NO |
| Decision history | YES |
| Enforcement record | YES |
| Outbox | YES |
| Strike | NO |
| Violation subsystem | NO |
| Warning standalone | NO |
| Warning provenance | YES |
| Appeal → Decision | YES |
| Chat message target | NO |
| Auction moderation | YES |
| Auction direct financial mutation | NO |
| For Sale direct financial mutation | NO |
| Target mutation | Target Domain |
| Audit history | Durable |
| Compatibility layer | NO |
| Legacy implementation | DELETE after replacement proof |

---

# 43. Design Gate

Jika seluruh bagian di atas disetujui, maka current `GovernanceCase` tidak lagi dianggap sebagai candidate canonical implementation.

Ia menjadi:

> **legacy implementation yang harus diganti.**

Bukan:

> implementation yang perlu ditambal agar mendukung Report/Case/Decision/Enforcement.

Canonical target:

```text
Report
Case
Decision
Enforcement
Warning
Appeal
Governance Audit History
```

dengan Outbox sebagai infrastructure mechanism.

---

# 44. Next Step

Setelah Business Truth disetujui:

**JANGAN langsung implement.**

Langkah berikutnya adalah:

### CANONICAL DESIGN AUDIT

Kita turunkan business truth menjadi:

- exact entity boundaries;
- cardinality;
- lifecycle;
- authority;
- transaction boundaries;
- API boundaries;
- event contracts;
- target-domain commands;
- audit semantics;
- DB constraints;
- admin behavior;
- mobile behavior;
- cleanup map.

Baru setelah itu:

```text
Canonical Design
      ↓
Implementation Plan
      ↓
Bounded Agent Prompt
      ↓
Implementation
      ↓
Positive Proof
      ↓
Negative / Residue Proof
      ↓
Destructive Cleanup
      ↓
Regression Proof
```

**BUSINESS TRUTH STATUS: DRAFT — READY FOR OWNER REVIEW**