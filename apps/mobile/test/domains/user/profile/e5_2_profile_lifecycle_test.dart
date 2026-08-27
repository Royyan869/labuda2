// E5.2 — Mobile profile-detail lifecycle ingestion + render-passive unit
// tests.
//
// Scope is pinned to two seams:
//   1) the JSON → AuthUser.lifecycle parse (`response.identity.lifecycle`),
//   2) the redaction-vocabulary helpers used by the profile screen.
//
// Widget-level golden tests for the rendered header would require a full
// Riverpod harness and existing test infrastructure that is not present in
// this repo yet; this slice intentionally stays at the pure-data layer so
// the contract is pinned without dragging in a new test harness.

import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/domains/user/profile/presentation/utils/profile_lifecycle_redaction.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

void main() {
  group('E5.2 — ContentLifecycle parse from /users/:id identity.lifecycle', () {
    test('null wire → unavailable (FAIL CLOSED)', () {
      expect(
        ContentLifecycleParse.fromWire(null),
        ContentLifecycle.unavailable,
      );
    });

    test('empty wire → unavailable (FAIL CLOSED)', () {
      expect(ContentLifecycleParse.fromWire(''), ContentLifecycle.unavailable);
    });

    test('unknown wire → unavailable (FAIL CLOSED)', () {
      expect(
        ContentLifecycleParse.fromWire('shadowbanned'),
        ContentLifecycle.unavailable,
      );
    });

    test('"active" wire → active', () {
      expect(ContentLifecycleParse.fromWire('active'), ContentLifecycle.active);
    });

    test('"unavailable" wire → unavailable', () {
      expect(
        ContentLifecycleParse.fromWire('unavailable'),
        ContentLifecycle.unavailable,
      );
    });

    test('"removed" wire → removed', () {
      expect(
        ContentLifecycleParse.fromWire('removed'),
        ContentLifecycle.removed,
      );
    });
  });

  group('E5.2 — profile lifecycle redaction vocabulary', () {
    test('active → no placeholder label', () {
      expect(profileLifecycleRedactionLabel(ContentLifecycle.active), isNull);
    });

    test('unavailable → "Pengguna tidak tersedia"', () {
      expect(
        profileLifecycleRedactionLabel(ContentLifecycle.unavailable),
        'Pengguna tidak tersedia',
      );
    });

    test('removed → "Pengguna dihapus"', () {
      expect(
        profileLifecycleRedactionLabel(ContentLifecycle.removed),
        'Pengguna dihapus',
      );
    });

    test('active → sensitive sections NOT suppressed', () {
      expect(
        profileLifecycleSuppressesSensitiveSections(ContentLifecycle.active),
        isFalse,
      );
    });

    test('unavailable → sensitive sections suppressed', () {
      expect(
        profileLifecycleSuppressesSensitiveSections(
          ContentLifecycle.unavailable,
        ),
        isTrue,
      );
    });

    test('removed → sensitive sections suppressed', () {
      expect(
        profileLifecycleSuppressesSensitiveSections(ContentLifecycle.removed),
        isTrue,
      );
    });

    test('active → target-user actions NOT disabled', () {
      expect(
        profileLifecycleDisablesTargetActions(ContentLifecycle.active),
        isFalse,
      );
    });

    test('unavailable → target-user actions disabled', () {
      expect(
        profileLifecycleDisablesTargetActions(ContentLifecycle.unavailable),
        isTrue,
      );
    });

    test('removed → target-user actions disabled', () {
      expect(
        profileLifecycleDisablesTargetActions(ContentLifecycle.removed),
        isTrue,
      );
    });
  });

  group(
    'PUBLIC_RENDERING_LIFECYCLE_PARITY — defensive client-side coarsening',
    () {
      // Doctrine (3-state truth): the public mobile rendering layer
      // recognises only {active, unavailable, removed}. Internal trust /
      // seller vocabulary that the backend may emit (deleted, banned,
      // suspended, degraded, verification_pending, subscription_expired,
      // seller_revoked) MUST be coarsened DEFENSIVELY at the mobile seam
      // so it never leaks past fromWire into widget branches.
      test('"deleted" / "banned" coarsen to removed', () {
        for (final raw in const ['deleted', 'banned']) {
          expect(
            ContentLifecycleParse.fromWire(raw),
            ContentLifecycle.removed,
            reason: 'raw "$raw" must coarsen to removed at the mobile seam',
          );
        }
      });

      test('"suspended" / "degraded" / "verification_pending" / '
          '"subscription_expired" / "seller_revoked" / "limited" / "restricted" '
          'coarsen to unavailable', () {
        const raws = [
          'suspended',
          'degraded',
          'verification_pending',
          'subscription_expired',
          'seller_revoked',
          'limited',
          'restricted',
        ];
        for (final raw in raws) {
          expect(
            ContentLifecycleParse.fromWire(raw),
            ContentLifecycle.unavailable,
            reason: 'raw "$raw" must coarsen to unavailable at the mobile seam',
          );
        }
      });
    },
  );

  group(
    'PUBLIC_RENDERING_LIFECYCLE_PARITY — canonical 2-string vocabulary',
    () {
      test('removed → "Pengguna dihapus"', () {
        expect(
          ContentLifecycle.removed.publicRedactionLabel,
          'Pengguna dihapus',
        );
      });

      test('unavailable → "Pengguna tidak tersedia"', () {
        expect(
          ContentLifecycle.unavailable.publicRedactionLabel,
          'Pengguna tidak tersedia',
        );
      });

      test('active → empty string (caller renders real identity)', () {
        expect(ContentLifecycle.active.publicRedactionLabel, '');
      });
    },
  );
}
