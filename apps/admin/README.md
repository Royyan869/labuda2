# Labuda Admin Dashboard

React 19 + Vite SPA for internal platform operations. Connects to the Labuda Go backend API via Firebase-authenticated requests.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Framework | React 19 + Vite |
| Language | TypeScript |
| Styling | Tailwind CSS 3 |
| Routing | React Router v7 |
| Icons | Lucide React |
| Auth | Firebase Authentication (Google/Email) |
| API | Labuda Go backend — HTTP REST, Firebase ID token as Bearer |

---

## Auth Model

The admin uses **Firebase Authentication** for identity. On login the Firebase ID token is extracted and sent as `Authorization: Bearer <token>` on every API request to the Go backend.

The Go backend validates the token against Firebase and checks that the user has admin capability (`HasAdminCapability`). There is no separate admin credential store — access control is backend-enforced.

**First admin setup:** The first admin user cannot be created through the panel — no admin exists yet to grant capabilities. See [`docs/operations/admin-bootstrap.md`](../../docs/operations/admin-bootstrap.md) for the SQL bootstrap procedure.

---

## Prerequisites

- Node.js 20+
- A running Labuda backend (`backend/`) on `http://localhost:8080` (or configured via `VITE_API_BASE_URL`)
- Firebase project credentials (see env setup)

---

## Setup

### 1. Install dependencies

```bash
cd apps/admin
npm install
```

### 2. Configure environment

```bash
cp .env.local.example .env.local
# Edit .env.local with your Firebase project values and backend URL
```

Required environment variables:

| Variable | Description |
|---|---|
| `VITE_FIREBASE_API_KEY` | Firebase web API key |
| `VITE_FIREBASE_AUTH_DOMAIN` | Firebase auth domain |
| `VITE_FIREBASE_PROJECT_ID` | Firebase project ID |
| `VITE_FIREBASE_STORAGE_BUCKET` | Firebase storage bucket |
| `VITE_FIREBASE_MESSAGING_SENDER_ID` | Firebase messaging sender ID |
| `VITE_FIREBASE_APP_ID` | Firebase app ID |
| `VITE_API_BASE_URL` | Go backend URL (default: `http://localhost:8080`) |

### 3. Start dev server

```bash
npm run dev
# Opens on http://localhost:5173
```

---

## Commands

```bash
npm run dev          # Development server with hot reload
npm run build        # Production build (tsc -b && vite build)
npm run preview      # Preview production build locally
npm run lint         # ESLint
npm run test         # Vitest unit tests
npm run test:watch   # Vitest watch mode
```

TypeScript is checked as part of `npm run build` (`tsc -b`). Run `npx tsc --noEmit` for a standalone type check.

---

## Folder Structure

```
apps/admin/src/
├── components/
│   ├── auth/          RequireCapability guard
│   ├── common/        Shared UI components
│   ├── disputes/      Dispute workspace widgets
│   ├── finance/       Withdrawal and ledger widgets
│   ├── layout/        AppShell, Sidebar, MainLayout
│   ├── moderation/    Moderation case widgets
│   ├── orders/        Order detail widgets
│   └── users/         User detail modal
├── lib/
│   ├── api/           API client + per-domain fetch functions
│   └── firebase.ts    Firebase app init
├── pages/             One file per route/page
├── store/             Zustand global state slices
└── types/             Shared TypeScript interfaces
```

---

## API Integration

All API calls go through `src/lib/api/client.ts` which:
1. Reads `VITE_API_BASE_URL` (defaults to `http://localhost:8080`)
2. Attaches the Firebase ID token as `Authorization: Bearer <token>`

Per-domain API modules:
- `src/lib/api/finance.ts` — withdrawals, ledger, refunds
- `src/lib/api/disputes.ts` — dispute workspace
- `src/lib/api/users.ts` — user management
- `src/lib/api/orders.ts` — order operations

---

## Capability Model

Routes are protected by `RequireCapability`. Admin users are registered in the Go backend with specific capability flags. The frontend reads capabilities from the `/api/v1/admin/me` endpoint after login.

---

## Building for Production

```bash
npm run build
# Output: apps/admin/dist/
```

The `dist/` folder is gitignored. Deploy to any static host or Firebase Hosting.
