# LABUDA — CLEANUP FIRST
## Root Truth, Canonical Authority, Anti-Resurrection & Cross-Session Operating Doctrine

**Status:** ACTIVE / CANONICAL

## 1. Kondisi Labuda

Labuda sedang dibangun dari zero-to-one. Current filesystem adalah factual implementation state, tetapi isi filesystem tidak otomatis merupakan business truth. Belum ada production compatibility obligation yang mengharuskan obsolete internal architecture dipertahankan.

Labuda telah melalui banyak putaran audit/fix/test-green/closure. Pola tersebut dapat meninggalkan A/B/C secara bergantian: satu branch diperbaiki, competitor/residue lain tertinggal; test kemudian dapat ikut melestarikan kontrak lama. Karena itu **GREEN ≠ CLEAN** dan **WORKING ≠ CONVERGED**.

Masalah utama yang harus dicegah adalah **multi-truth codebase**: satu konsep memiliki beberapa service, repository, DTO, provider, route, schema, test, fixture, fallback, adapter, komentar, atau dokumentasi yang semuanya tampak seolah-olah valid.

## 2. Perubahan Cara Kerja: CLEANUP FIRST

Mulai sekarang pola utama adalah:

```text
BUSINESS TRUTH
    ↓
CANONICAL AUTHORITY
    ↓
ROOT / PRODUCER / CONSUMER / LIFECYCLE
    ↓
CLASSIFY
    ↓
IMPLEMENT MINIMUM CANONICAL GAP
    ↓
PURGE COMPETITOR END-TO-END
    ↓
POSITIVE PROOF
    ↓
NEGATIVE PROOF / RESIDUE SWEEP
    ↓
SKEPTICAL REVIEW
    ↓
CLOSE + HANDOFF
    ↓
MOVE FORWARD
```

Bukan lagi:

```text
kode lama → tambal → tambah kondisi → compatibility → test green → lanjut
```

Cleanup bukan pekerjaan kosmetik setelah feature selesai. Cleanup adalah bagian dari convergence dan closure.

## 3. Hierarchy of Truth

Urutan authority:

1. **Owner / Business Truth**
2. **Canonical Architectural Decision**
3. **Current Filesystem — factual implementation state**
4. **Database / Schema / Runtime evidence**
5. **Tests**
6. **Git / GitHub**

Kode, test, jumlah caller, kompleksitas, dokumentasi lama, dan Git history tidak dapat mengangkat implementation obsolete menjadi canonical.

Mental model:

> **CODEBASE tells us what EXISTS.**
>
> **AUTHORITY tells us what SHOULD EXIST.**
>
> **DEPENDENCY tells us HOW TO CHANGE IT SAFELY.**
>
> **CLEANUP removes what SHOULD NO LONGER EXIST.**

## 4. Root First

Sebelum membangun atau memperbaiki branch, tentukan akarnya:

- business truth;
- canonical authority;
- invariant;
- producer;
- consumer;
- contract/wire contract;
- lifecycle/state;
- persistence authority;
- mutation authority;
- competing authority;
- legacy producer/consumer;
- schema residue;
- test/fixture residue;
- fallback/alias/adapter/compatibility path;
- documentation/comment/naming yang dapat menyebabkan resurrection.

**Dependency graph digunakan setelah authority ditentukan. Dependency graph bukan penentu kebenaran.**

## 5. Empat Klasifikasi Wajib

### CANONICAL / REQUIRED → KEEP

Current truth dan memang diperlukan. Pertahankan dan koreksi jika implementation salah.

### OBSOLETE → PURGE

Sudah digantikan, ditolak, atau tidak diperlukan. Hapus total seluruh jalur yang mempertahankannya.

### INCOMPLETE / MISSING IMPLEMENTATION

Capability memang canonical tetapi belum lengkap. **STOP CLEANUP → implement minimum canonical → proof → lanjut cleanup.**

### AMBIGUOUS

Authority belum jelas. **Jangan delete dan jangan implement.** Cari evidence sampai factual authority jelas atau kembalikan business decision kepada Owner.

## 6. Total Purge

Jika sesuatu terbukti obsolete, jangan hanya menghapus file utama. Periksa dan bersihkan lapisan yang relevan:

- UI/widget;
- route/navigation;
- provider/controller/state;
- model/entity;
- DTO/request/response;
- mapper;
- repository/interface;
- service/domain;
- handler/API;
- schema/migration;
- cache/worker;
- test/fixture;
- export/import;
- configuration/feature flag;
- fallback/alias/adapter/compatibility;
- comment/documentation/naming.

Tujuannya bukan sekadar grep nol. Tujuannya adalah **tidak ada jalur aktif yang dapat menghidupkan desain lama kembali**.

## 7. Kill Once, Lock Forever

Desain yang telah ditolak menjadi **forbidden design**.

Jika muncul lagi:

1. jangan anggap requirement baru;
2. jangan hidupkan untuk membuat test lama green;
3. jangan buat alias;
4. jangan buat compatibility parser;
5. jangan buat fallback;
6. trace producer/consumer/residue;
7. purge jika obsolete;
8. tambahkan negative proof bila resurrection risk tinggi.

Closure harus meninggalkan:

- canonical truth;
- forbidden design;
- removal manifest;
- positive proof;
- negative proof/residue search;
- status `CLOSED — CANONICAL DESIGN LOCKED`.

## 8. Test Bukan Authority

Test hanya proof terhadap canonical behavior.

- Test canonical → KEEP.
- Test obsolete → update atau PURGE.
- Jangan mengubah production architecture agar test obsolete tetap PASS.
- Jika kontrak canonical berubah, test harus ikut konvergen.

Test fixture yang mempertahankan legacy adalah **residue**, bukan alasan compatibility.

## 9. Compatibility Bukan Default

Labuda tidak boleh otomatis memelihara:

- dual-read;
- dual-write;
- legacy endpoint;
- deprecated DTO support;
- alias;
- fallback;
- adapter chain;
- compatibility repository/service;
- version bridge;
- historical behavior.

Pertanyaan wajib:

> **Apakah ada current requirement nyata untuk compatibility ini?**

Jika tidak, jangan pertahankan obsolete complexity.

## 10. Positive + Negative Proof

**Positive proof:** canonical implementation benar-benar bekerja.

**Negative proof:** desain lama tidak lagi memiliki jalur aktif.

Negative proof dapat berupa:

- field lama tidak dikirim;
- route lama tidak terdaftar;
- old provider/helper/DTO tidak digunakan;
- old producer/consumer hilang;
- old schema writer hilang;
- residue search bersih;
- canonical authority tetap singular.

Domain yang pernah mengalami resurrection wajib mendapat residue search + authority review, dan formal negative test bila memang diperlukan.

## 11. Runtime Proof

Tests pass tidak otomatis berarti integration selesai.

Untuk masalah runtime, proof mengikuti jalur nyata:

```text
DB → Backend → API → Auth → Mobile → State → UI
```

Mock/fixture membuktikan contract tertentu; runtime proof membuktikan sistem nyata.

## 12. Satu Scope

Hanya satu scope implementasi aktif, satu invariant utama, dan satu acceptance gate.

Cleanup harus agresif **di dalam scope yang sudah terbukti**, tetapi tidak berubah menjadi global cleanup.

Temuan di luar scope:

- catat;
- klasifikasikan;
- jangan langsung dikerjakan;
- kecuali P0/P1 benar-benar memblokir scope/release safety.

## 13. Siklus Kerja Wajib

```text
PROBLEM
  ↓
BUSINESS TRUTH
  ↓
ROOT AUTHORITY
  ↓
CLASSIFICATION
  ↓
TRACE
  ↓
DESIGN DECISION
  ↓
IMPLEMENT MINIMUM CANONICAL GAP
  ↓
PURGE OBSOLETE COMPETITOR
  ↓
POSITIVE PROOF
  ↓
NEGATIVE PROOF / RESIDUE SEARCH
  ↓
SKEPTICAL REVIEW
  ↓
CLOSE
  ↓
HANDOFF
```

Audit harus berhenti ketika wrong behavior, root cause, authority, producer, consumer, invariant, lifecycle, impacted paths, competing design, dan required changes sudah cukup terbukti.

Setelah itu:

> **STOP AUDITING. START EXECUTING.**

## 14. Area CLOSED

Area CLOSED tidak dibuka kembali hanya karena:

- agent menemukan nama lama;
- test lama ingin dibuat green;
- refactor terlihat lebih bagus;
- historical implementation mudah dipakai;
- kemungkinan future requirement.

Trigger sah:

- runtime bug;
- regression;
- failing build/test/migration/integration proof;
- authority conflict nyata;
- contract berubah;
- security/data/financial/authorization risk;
- business truth berubah melalui Owner.

## 15. Cross-Session Handoff

Mental state tidak boleh hanya berada di chat. Setiap scope selesai harus meninggalkan handoff yang dapat dipakai chat/agent berikutnya:

```text
DOMAIN / SCOPE:
STATUS:
BUSINESS TRUTH:
CANONICAL AUTHORITY:
FORBIDDEN / KILLED:
IMPLEMENTATION:
CLEANUP:
POSITIVE PROOF:
NEGATIVE PROOF:
KNOWN LIMITATIONS:
UNRESOLVED OWNER DECISIONS:
OUT OF SCOPE:
NEXT SCOPE:
```

Chat baru harus dapat melanjutkan dari dokumen/codebase, bukan menebak sejarah.

## 16. Agent Contract

Setiap prompt agent wajib memuat:

- Objective;
- Canonical truth;
- Forbidden design;
- Scope;
- Protected / Out of scope;
- Required proof;
- Cleanup requirement;
- Stop conditions.

Agent adalah executor/auditor, bukan penentu business policy.

Agent wajib STOP jika authority tidak jelas, business decision diperlukan, root cause belum terbukti, protected path harus disentuh, baseline tidak dapat dipercaya, atau solusi membutuhkan desain terlarang.

## 17. Agent Report = Claim

Laporan agent bukan final truth.

Klaim seperti:

- “root cause found”;
- “safe to delete”;
- “no competing authority”;
- “production clean”;
- “tests pass”;
- “scope closed”

harus memiliki evidence.

Jika evidence tidak cukup:

> **UNPROVEN ≠ PASS.**

ChatGPT melakukan skeptical review dan mencari counter-evidence sebelum menerima closure.

## 18. Git / GitHub

**Git/GitHub adalah backup/reference, bukan authority dan bukan mesin waktu.**

Dilarang menggunakan Git untuk menghidupkan kembali sejarah atau melakukan rollback/restore, termasuk:

- checkout implementation lama;
- reset;
- revert;
- cherry-pick legacy;
- stash/stash-pop;
- restore historical code;
- mengambil desain lama sebagai dasar architecture.

Jika kebutuhan nyata muncul kembali, buat ulang dari:

```text
CURRENT BUSINESS REQUIREMENT
→ CURRENT CANONICAL AUTHORITY
→ CURRENT ARCHITECTURE
→ CURRENT PROOF
```

## 19. Test Data

Development/test data bukan business authority. Data stale, inconsistent, invalid, atau berasal dari desain lama boleh dihapus/reset/reseed/dibuat ulang.

Jangan mempertahankan architecture lama hanya agar test data lama tetap cocok.

## 20. Definisi Selesai

Scope hanya boleh `CLOSED — CANONICAL DESIGN LOCKED` jika:

- business truth jelas;
- canonical authority singular;
- competing authority mati;
- producer lama hilang;
- consumer lama dikonvergensikan atau dihapus;
- obsolete DTO/schema/route/provider/mapper/worker/fixture dibersihkan bila relevan;
- fallback/alias/compatibility tidak tersisa tanpa requirement;
- positive proof PASS;
- negative proof/residue search PASS bila diperlukan;
- runtime proof tersedia bila relevan;
- protected paths untouched;
- tidak ada unexplained residue;
- handoff siap untuk sesi berikutnya.

Tidak ada kategori **“green tapi masih ada residu”** untuk obsolete residue yang sudah terbukti.

## 21. Mental Model Sederhana

```text
KODE LAMA BUKAN KEBENARAN.
TEST LAMA BUKAN KEBENARAN.
JUMLAH CALLER BUKAN KEBENARAN.
KOMPLEKSITAS BUKAN KEBENARAN.
GIT HISTORY BUKAN KEBENARAN.

BUSINESS TRUTH
      ↓
AUTHORITY
      ↓
CANONICAL IMPLEMENTATION
      ↓
PROOF
      ↓
PURGE COMPETITOR
      ↓
LOCK
```

## 22. Final Doctrine

> **Satu konsep bisnis = satu authority.**
>
> **Tentukan akar kebenaran sebelum menyentuh ranting.**
>
> **Jika authority ganda, pilih satu dan bunuh yang lain.**
>
> **Jika canonical capability belum ada, implement minimum canonical.**
>
> **Jika desain terbukti obsolete, PURGE TOTAL.**
>
> **Jika consumer legacy masih hidup, convergence belum selesai.**
>
> **Jika fallback lama masih hidup, canonical authority belum berdiri sendiri.**
>
> **Positive proof membuktikan desain baru hidup. Negative proof/residue search memastikan desain lama tetap mati.**
>
> **Cleanup adalah bagian dari implementation, bukan pekerjaan nanti.**
>
> **Green bukan berarti clean.**
>
> **Closed berarti canonical design locked.**
>
> **Chat baru harus dapat melanjutkan dari handoff, bukan menebak sejarah.**
>
> **Jika kebutuhan lama benar-benar muncul kembali, buat ulang dari truth saat itu. Jangan hidupkan zombie.**

# TARGET AKHIR: TOTAL CONVERGENCE

**Tentukan akar kebenarannya. Bangun hanya di atas akar yang benar. Kubur competitor sampai tidak ada jalur resurrection. Buktikan. Tutup. Lanjut maju.**
