# Pricing UX Enhancements - Visual Guide

## 📱 BEFORE & AFTER COMPARISONS

---

## 1️⃣ SAVINGS DISPLAY

### BEFORE
```
━━━━━━━━━━━━━━━━━━━━━━
Rincian Harga

Harga Barang (1x)     Rp 100.000
Subtotal              Rp 100.000
Ongkos Kirim           Rp 15.000

🏷️ DISKON            -Rp 10.000

━━━━━━━━━━━━━━━━━━━━━━
Total Pembayaran      Rp 105.000
━━━━━━━━━━━━━━━━━━━━━━
```

### AFTER ✨
```
━━━━━━━━━━━━━━━━━━━━━━
Rincian Harga

Harga Barang (1x)     Rp 100.000
Subtotal              Rp 100.000
Ongkos Kirim           Rp 15.000

🏷️ DISKON            -Rp 10.000

💰 Total Hemat         Rp 15.000
━━━━━━━━━━━━━━━━━━━━━━
Total Pembayaran      Rp 100.000
━━━━━━━━━━━━━━━━━━━━━━
```

**Impact:** User immediately sees total savings (discount + coins)

---

## 2️⃣ NEGOTIATION VISUAL

### BEFORE
```
━━━━━━━━━━━━━━━━━━━━━━
Rincian Harga

Harga Barang (1x)      Rp 80.000
Subtotal               Rp 80.000
Ongkos Kirim            Rp 15.000

━━━━━━━━━━━━━━━━━━━━━━
Total Pembayaran       Rp 95.000
━━━━━━━━━━━━━━━━━━━━━━

🤝 Harga Nego | 🪙 Max koin 20%
```

### AFTER ✨
```
┌─────────────────────────────────────┐
│ 🤝 Harga Negosiasi                  │
│                                     │
│ Harga Awal        Harga Deal        │
│ Rp 120.000        Rp 95.000         │
│ ──────────────                       │
│                                     │
│ 💰 Hemat Rp 25.000                  │
└─────────────────────────────────────┘

━━━━━━━━━━━━━━━━━━━━━━
Rincian Harga

Harga Barang (1x)      Rp 80.000
Subtotal               Rp 80.000
Ongkos Kirim            Rp 15.000
━━━━━━━━━━━━━━━━━━━━━━
Total Pembayaran       Rp 95.000
━━━━━━━━━━━━━━━━━━━━━━
```

**Impact:** Clear visual comparison shows deal value

---

## 3️⃣ SHIPPING CONTEXT

### BEFORE
```
Harga Barang (1x)      Rp 100.000
Subtotal               Rp 100.000
Ongkos Kirim            Rp 15.000
```

### AFTER ✨
```
Harga Barang (1x)      Rp 100.000
Subtotal               Rp 100.000
Ongkos Kirim            Rp 15.000
ℹ️ Ongkir ditentukan oleh seller (pengiriman ikan hidup)
```

**Impact:** Users understand shipping cost rationale for seafood

---

## 4️⃣ TOKEN URGENCY

### BEFORE (Static)
```
⏱️ Harga segar: 8 menit  [🔄]
```

### AFTER ✨ (Dynamic - Real-time Countdown)

**Normal State (5-10 minutes):**
```
┌─────────────────────────────────────┐
│ ⏱️ Harga berlaku 10 menit    [🔄] │
│ Segarkan harga jika waktu habis     │
└─────────────────────────────────────┘
```

**Warning State (2-5 minutes):**
```
┌─────────────────────────────────────┐
│ ⏱️ Harga berlaku 3 menit 45 detik [🔄] │
│ Harga mungkin berubah sewaktu-waktu │
└─────────────────────────────────────┘
```

**Critical State (< 2 minutes):**
```
┌─────────────────────────────────────┐
│ ⚠️ Harga kadaluarsa dalam 58 detik! [🔄] │
│ Segera selesaikan pesanan           │
└─────────────────────────────────────┘
```

**Expired State:**
```
┌─────────────────────────────────────┐
│ ❌ Waktu harga habis          [🔄] │
│ Silakan refresh harga terbaru       │
└─────────────────────────────────────┘
```

**Impact:** Creates urgency, encourages faster checkout

---

## 5️⃣ COINS EXPLANATION

### BEFORE
```
Harga Barang (1x)      Rp 100.000
Subtotal               Rp 100.000
Ongkos Kirim            Rp 15.000
🏷️ DISKON             -Rp 10.000
━━━━━━━━━━━━━━━━━━━━━━
Total Pembayaran       Rp 105.000
```

### AFTER ✨

**In Pricing Breakdown:**
```
Harga Barang (1x)      Rp 100.000
Subtotal               Rp 100.000
Ongkos Kirim            Rp 15.000
🏷️ DISKON             -Rp 10.000

┌─────────────────────┐
│ 🪙 Coins           -Rp 5.000 │
│ Coins digunakan sebagai      │
│ potongan harga tambahan      │
└─────────────────────┘

💰 Total Hemat          Rp 15.000
━━━━━━━━━━━━━━━━━━━━━━
Total Pembayaran       Rp 100.000
```

**Additional Info Box (Bottom):**
```
┌─────────────────────────────────────┐
│ 🪙 Info Coins                       │
│                                     │
│ Coins digunakan sebagai potongan    │
│ harga tambahan. Maksimal penggunaan │
│ coins adalah 20% dari total         │
│ pembayaran.                         │
└─────────────────────────────────────┘
```

**Impact:** Users understand coins value and usage

---

## 🎨 COLOR SCHEME

### Primary Colors (Negotiation)
- Background: Gradient `primaryContainer` (50% → 20% opacity)
- Border: `primary` (30% opacity)
- Text: `primary`
- Icon: `handshake_outlined`

### Secondary Colors (Savings)
- Background: `secondaryContainer` (30% opacity)
- Border: `secondary` (30% opacity)
- Text: `secondary`
- Icon: `savings_outlined`

### Tertiary Colors (Coins)
- Background: `tertiaryContainer` (30% opacity)
- Text: `tertiary`
- Icon: `monetization_on_outlined`

### Urgency Colors (Token Expiry)
- **Normal:** `primary` / `primaryContainer`
- **Warning:** `secondary` / `secondaryContainer`
- **Critical:** `error` / `errorContainer` (thicker border)
- **Expired:** `error` / `errorContainer`

---

## 📐 LAYOUT HIERARCHY

### Order Preview Screen Structure
```
┌─────────────────────────────────────┐
│ ← Preview Pesanan                   │
├─────────────────────────────────────┤
│                                     │
│ [Trust Labels]                      │
│ 🤝 Harga Nego | 🪙 Max koin 20%     │
│                                     │
│ [Token Expiry Widget]               │
│ ⏱️ Harga berlaku 10 menit    [🔄] │
│                                     │
│ [Negotiation Visual]                │
│ ┌─────────────────────────────────┐ │
│ │ 🤝 Harga Negosiasi              │ │
│ │ Original: Rp 120.000            │ │
│ │ Deal:     Rp 95.000             │ │
│ │ 💰 Hemat Rp 25.000              │ │
│ └─────────────────────────────────┘ │
│                                     │
│ [Pricing Breakdown]                 │
│ ┌─────────────────────────────────┐ │
│ │ Rincian Harga                   │ │
│ │ Harga Barang (1x)  Rp 100.000   │ │
│ │ Subtotal            Rp 100.000   │ │
│ │ Ongkos Kirim        Rp 15.000   │ │
│ │ ℹ️ Ongkir ditentukan oleh...   │ │
│ │                                 │ │
│ │ 🏷️ DISKON          -Rp 10.000  │ │
│ │ 🪙 Coins           -Rp 5.000    │
│ │ Coins digunakan sebagai...      │ │
│ │                                 │ │
│ │ 💰 Total Hemat      Rp 15.000   │ │
│ │ ─────────────────────────────  │ │
│ │ Total Pembayaran    Rp 100.000  │ │
│ └─────────────────────────────────┘ │
│                                     │
│ [Shipping Information]              │
│ Alamat Pengiriman                   │
│                                     │
│ [Important Notes]                   │
│ ✅ Garansi Harga                    │
│                                     │
│ [Coins Explanation]                 │
│ 🪙 Info Coins                       │
│                                     │
├─────────────────────────────────────┤
│ Total Pembayaran      Rp 100.000    │
│ [Konfirmasi Pesanan]                │
└─────────────────────────────────────┘
```

---

## 🎯 UX IMPROVEMENTS SUMMARY

### Clarity ✅
- Total savings prominently displayed
- Clear price comparison for negotiations
- Shipping cost explained
- Coins usage clarified

### Urgency ✅
- Real-time countdown timer
- Progressive urgency levels (normal → warning → critical)
- Clear call-to-actions at each stage

### Trust ✅
- Professional negotiation visual
- Transparent pricing breakdown
- Clear shipping context
- Coins explanation builds confidence

### Persuasion ✅
- Savings highlighted (deals feel valuable)
- Negotiation shows deal value
- Token expiry creates FOMO
- Professional presentation builds trust

---

## 📱 SCREENSHOT LOCATIONS

When implementing, take screenshots of:

1. **Negotiation Checkout**
   - Show original vs negotiated price
   - Display savings badge
   - Include trust labels

2. **Token Urgency States**
   - Normal state (5-10 min)
   - Warning state (2-5 min)
   - Critical state (<2 min)
   - Expired state

3. **Savings Display**
   - With discount only
   - With coins only
   - With both discount + coins

4. **Coins Explanation**
   - In pricing breakdown
   - Dedicated info box

---

## 🚀 NEXT STEPS

1. **Backend Team:**
   - Add new fields to pricing preview API
   - Calculate total_savings (discount + coins)
   - Provide original_price for negotiations
   - Provide coins_amount separately

2. **Mobile Team:**
   - Test all 5 enhancements
   - Verify token expiry timer
   - Check urgency level transitions
   - Validate coins calculation

3. **QA Team:**
   - E2E testing for all scenarios
   - Verify no calculation in UI
   - Check backend data consistency
   - Test token refresh functionality

4. **Product Team:**
   - Monitor conversion metrics
   - Track coins usage rate
   - Measure negotiation completion rate
   - Gather user feedback

---

**Status:** ✅ Design Complete  
**Ready for:** Backend Implementation → Frontend Integration → QA Testing  
**Expected Impact:** ↑ Conversion, ↑ Trust, ↓ Confusion
