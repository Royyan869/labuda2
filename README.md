# Labuda Monorepo

Labuda is a monorepo for the backend, admin console, mobile client, and the
current authority docs that describe how those pieces fit together.

## Active Areas

- `backend/` - Go services, workers, migrations, and runtime APIs
- `apps/admin/` - React admin console
- `apps/mobile/` - Flutter client
- `docs/` - current operational, workflow, and domain guidance

## Current Authority

- Treat the current filesystem as the source of truth.
- Use the code and the live docs in `docs/operations/` and `docs/flows/` for
  current behavior.
- Do not rely on old Git history as architecture authority.

## Local Development

- Backend: `cd backend && go run ./cmd/migrate && go run ./cmd/core_server`
- Admin: `cd apps/admin && npm test && npm run build`
- Mobile: `cd apps/mobile && flutter test && flutter analyze`

## Notes

- The repository contains multiple apps and shared docs, so review the relevant
  app directory before making behavior changes.
- When a doc conflicts with code, the code and its tests win.
