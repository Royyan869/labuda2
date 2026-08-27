#!/bin/bash
set -e

# B4.2 / G15 — *Actor governance fields MUST NOT be assigned outside
# constructors and the canonical injection seam.
#
# Doctrine: docs/contracts/governance-constitution.md §1 verdict 10 +
# §3 F9 + §8.1 G15. Actor is the middleware-layer capability cache;
# it is not the visibility authority. Its fields are exported by Go
# convention but mutation is forbidden by doctrine. This guard
# enforces the doctrine since the language cannot.
#
# Detection heuristic: assignments of the form `actor.<Field> = ...`
# where <Field> is one of Actor's governance-critical fields. The
# heuristic targets the conventional local variable name `actor`
# returned by `middleware.GetActorFromContext`; it does not flag
# method-receiver assignments inside the Actor type's own package
# (those are constructor-side and allowlisted by path).
#
# Allowlisted paths (Actor construction / injection seams):
#   - backend/internal/platform/capability/        (Actor type + ResolveActor)
#   - backend/internal/middleware/actor_context.go (canonical injection)
#   - *_test.go                                    (fixture builders)

echo "🔒 G15: Actor mutation forbidden outside construction seams"

# Pattern: actor.<Field> = <not =>
# - `\b` word boundary stops `foo_actor.` from matching.
# - `=[^=]` excludes comparison `==` and `===`.
# - `(=$)?` would catch end-of-line `=` but unnecessary in practice.
PATTERN='\bactor\.(Role|Capabilities|AccountStatus|SellerStatus|EmailVerified|IsIdentityComplete|ID)[[:space:]]*=[^=]'

VIOLATIONS=$(grep -rnE "$PATTERN" backend \
  --include='*.go' 2>/dev/null \
  | grep -v '/platform/capability/' \
  | grep -v 'middleware/actor_context\.go' \
  | grep -v '_test\.go' || true)

if [ -n "$VIOLATIONS" ]; then
  echo "❌ BLOCKED: *Actor field mutation outside construction seams"
  echo ""
  echo "Forbidden pattern (G15): actor.<Field> = ..."
  echo "Actor is a read-only middleware-layer cache. Construct a new"
  echo "Actor (or use the existing constructor) instead of mutating."
  echo "See: docs/contracts/governance-constitution.md §1 verdict 10,"
  echo "     §3 F9, §8.1 G15"
  echo ""
  echo "Allowed sites (excluded from this guard):"
  echo "  - backend/internal/platform/capability/"
  echo "  - backend/internal/middleware/actor_context.go"
  echo "  - any *_test.go"
  echo ""
  echo "Offending mutations:"
  echo "$VIOLATIONS"
  exit 1
fi

echo "✅ G15 passed: no Actor mutation outside construction seams"
