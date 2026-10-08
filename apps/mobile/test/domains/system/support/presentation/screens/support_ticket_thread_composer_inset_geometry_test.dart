// SAFE-AREA-14 — SUPPORT TICKET THREAD COMPOSER: BOTTOM-INSET GEOMETRY.
//
// Locks the Owner invariant on the one surface that was still missing it:
//   * the composer touches the bottom edge, so the `SafeArea` mounted inside
//     the composer container is THE live system-bottom-inset authority —
//     exactly the contract of `ChatInputArea` and
//     `CommentInputWithCommerceReference` (SAFE-AREA-13 gap #1);
//   * it consumes the REAL, LIVE inset: a system navigation bar appearing,
//     changing or disappearing moves the composer with it (0 → 24 → 34 → 48);
//   * inset 0 leaves NO artificial fixed clearance behind — the composer
//     ends exactly at its own design padding (`AppMetrics.p16`);
//   * the keyboard stays the Scaffold's business (`resizeToAvoidBottomInset`
//     body resize): open → body lifts, SafeArea yields (padding is what the
//     viewInsets did not consume); closed → the composer returns to the
//     system-inset position with no stale gap.
//
// RENDER-BASED on purpose: this measures the actual rendered composer row of
// the real screen under injected window metrics — never a source string, and
// never a bare `find.byType(SafeArea)` presence check.
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/support/domain/domain.dart';
import 'package:labuda/domains/system/support/domain/repositories/support_repository.dart';
import 'package:labuda/domains/system/support/presentation/presentation.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/providers/authenticated_account_provider.dart';
import 'package:labuda/shared/widgets/composer_action_buttons.dart';

const _ticketId = 't1';

/// Minimal read-only repository: ticket + thread + events resolve
/// immediately so the real screen settles into a static state.
class _FakeSupportRepository implements SupportRepository {
  _FakeSupportRepository({this.messages = const <SupportMessage>[]});

  final List<SupportMessage> messages;

  @override
  Future<Result<SupportTicket>> getTicket(String ticketId) async =>
      Result.success(_ticket(ticketId));

  @override
  Future<Result<List<SupportMessage>>> getMessages(
    String ticketId, {
    int limit = 100,
  }) async => Result.success(messages);

  @override
  Future<Result<List<SupportEvent>>> getEvents(
    String ticketId, {
    int limit = 100,
  }) async => Result.success(const <SupportEvent>[]);

  @override
  Future<Result<String>> createTicket({
    required String userId,
    required String userName,
    String? userAvatar,
    required SupportCategory category,
    SupportPriority priority = SupportPriority.medium,
    String? subject,
    String? description,
    String? linkedOrderId,
  }) => throw UnimplementedError();

  @override
  Future<Result<List<SupportTicket>>> getMyTickets({int limit = 50}) =>
      throw UnimplementedError();

  @override
  Future<Result<void>> reopenTicket(ReopenTicketRequest request) =>
      throw UnimplementedError();

  @override
  Future<Result<void>> sendMessage({
    required String ticketId,
    required String message,
  }) => throw UnimplementedError();
}

SupportTicket _ticket(String id) => SupportTicket(
  id: id,
  userId: 'buyer-1',
  userName: 'Buyer',
  category: SupportCategory.paymentIssue,
  priority: SupportPriority.medium,
  status: SupportStatus.open,
  subject: 'Payment issue',
  createdAt: DateTime.utc(2026, 1, 1),
);

AuthUser _user() => AuthUser(
  id: 'buyer-1',
  createdAt: DateTime.utc(2026, 8, 1),
  updatedAt: DateTime.utc(2026, 8, 1),
  email: 'buyer@example.com',
  username: 'buyer',
  isEmailVerified: true,
  accountStatus: AccountStatus.active,
  roles: const [],
  provider: AuthProvider.email,
);

SupportMessage _message() => SupportMessage(
  id: 'm1',
  roomId: 't1',
  senderId: 'admin-1',
  senderType: SupportSenderType.admin,
  messageType: SupportMessageType.text,
  body: 'Halo, ada yang bisa kami bantu?',
  createdAt: DateTime.utc(2026, 8, 2, 9),
);

/// Window metrics for the TEST VIEW (physical pixels, like a device).
///
/// Platform semantics: `padding` is whatever `viewInsets` (the keyboard) has
/// NOT consumed of `viewPadding` — so an open keyboard zeroes the bottom
/// padding exactly like Android does, and the SafeArea then yields to it.
void _setWindowInsets(
  WidgetTester tester, {
  double systemBottom = 0,
  double keyboard = 0,
}) {
  final double dpr = tester.view.devicePixelRatio;
  tester.view.padding = FakeViewPadding(
    bottom: math.max(0.0, systemBottom - keyboard) * dpr,
  );
  tester.view.viewPadding = FakeViewPadding(bottom: systemBottom * dpr);
  tester.view.viewInsets = FakeViewPadding(bottom: keyboard * dpr);
}

Future<void> _pumpThread(
  WidgetTester tester, {
  List<SupportMessage> messages = const <SupportMessage>[],
}) async {
  await tester.pumpWidget(
    ProviderScope(
      // Retry is disabled so a failed load never schedules timers.
      retry: (retryCount, error) => null,
      overrides: [
        authenticatedUserProvider.overrideWith((ref) => _user()),
        supportRepositoryProvider.overrideWithValue(
          _FakeSupportRepository(messages: messages),
        ),
      ],
      child: MaterialApp(
        theme: AppTheme.lightTheme,
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('id'),
        home: const SupportTicketThreadScreen(ticketId: _ticketId),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

double _surfaceBottom(WidgetTester tester) =>
    tester.getRect(find.byType(Scaffold)).bottom;

/// Bottom edge of the composer's send affordance — a direct child of the
/// composer `Row`, so its bottom IS the composer content bottom.
double _composerBottom(WidgetTester tester) =>
    tester.getBottomRight(find.byType(ComposerSendButton)).dy;

/// Bottom edge of the composer text field — must share the row bottom.
double _fieldBottom(WidgetTester tester) =>
    tester.getBottomRight(find.byType(TextField)).dy;

void main() {
  testWidgets(
    'system inset = 0: the composer ends at its own design padding, '
    'with no artificial fixed clearance',
    (tester) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 0);
      await _pumpThread(tester);

      expect(find.byType(SupportTicketThreadScreen), findsOneWidget);
      expect(find.byType(ComposerSendButton), findsOneWidget);
      expect(find.byType(TextField), findsOneWidget);

      final double surface = _surfaceBottom(tester);
      // No inset → the ONLY space below the composer content is the
      // container's own p16 design padding. A leftover inset-sized gap
      // (or a fixed 80/96/100 stand-in) would break this.
      expect(
        _composerBottom(tester),
        closeTo(surface - AppMetrics.p16, 0.01),
        reason:
            'composer carried an artificial bottom clearance at inset 0 '
            '(bottom ${_composerBottom(tester)} != ${surface - AppMetrics.p16})',
      );
      expect(
        _fieldBottom(tester),
        closeTo(surface - AppMetrics.p16, 0.01),
        reason: 'the text field must share the composer row bottom',
      );
    },
  );

  testWidgets(
    'system inset = 24 / 34 / 48: the composer follows the LIVE inset exactly',
    (tester) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 24);
      await _pumpThread(tester);

      final Map<double, double> bottoms = <double, double>{};
      for (final double inset in const <double>[24, 34, 48]) {
        _setWindowInsets(tester, systemBottom: inset);
        await tester.pumpAndSettle();

        final double surface = _surfaceBottom(tester);
        final double composerBottom = _composerBottom(tester);
        bottoms[inset] = composerBottom;

        expect(
          composerBottom,
          closeTo(surface - inset - AppMetrics.p16, 0.01),
          reason:
              'the composer did not consume the live system inset $inset '
              '(bottom $composerBottom, expected '
              '${surface - inset - AppMetrics.p16})',
        );
        expect(
          composerBottom,
          lessThanOrEqualTo(surface - inset),
          reason:
              'the composer entered the system navigation region at '
              'inset $inset',
        );
        expect(
          _fieldBottom(tester),
          closeTo(composerBottom, 0.01),
          reason: 'the text field must share the composer row bottom',
        );
      }

      // A changed inset moves the composer 1:1 — no stale gap, no stand-in.
      expect(
        bottoms[24]! - bottoms[48]!,
        closeTo(24, 0.01),
        reason: 'the composer did not track a changed system inset 1:1',
      );
      expect(
        bottoms[34]! - bottoms[48]!,
        closeTo(14, 0.01),
        reason: 'the composer did not track a changed system inset 1:1',
      );
    },
  );

  testWidgets(
    'keyboard open: the Scaffold lifts the body, the SafeArea yields — '
    'and closing it leaves no stale gap',
    (tester) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 24, keyboard: 300);
      // A message thread (not the empty state): the messages area shrinks
      // to a sliver under the keyboard, and the ListView clips/scrolls it
      // instead of overflowing. The empty-thread overflow under a squeezed
      // body is a PRE-EXISTING screen issue, reported separately — this
      // bounded test locks the composer authority only.
      await _pumpThread(tester, messages: <SupportMessage>[_message()]);

      final double surface = _surfaceBottom(tester);

      // Keyboard handling stays the Scaffold's (`resizeToAvoidBottomInset`):
      // the body bottom is surface - viewInsets, and the SafeArea padding is
      // 0 because the keyboard consumed the bottom padding — never a second,
      // hand-rolled keyboard reservation inside the composer.
      expect(
        _composerBottom(tester),
        closeTo(surface - 300 - AppMetrics.p16, 0.01),
        reason:
            'composer geometry with keyboard open must be body bottom '
            '(${surface - 300}) minus the design padding p16',
      );
      expect(
        _composerBottom(tester),
        lessThanOrEqualTo(surface - 300),
        reason: 'the composer was pushed under the keyboard',
      );

      // Keyboard closes → the composer returns to the live system-inset
      // position; a keyboard-sized gap must not survive.
      _setWindowInsets(tester, systemBottom: 24, keyboard: 0);
      await tester.pumpAndSettle();
      expect(
        _composerBottom(tester),
        closeTo(surface - 24 - AppMetrics.p16, 0.01),
        reason: 'a stale keyboard-sized gap survived the closed keyboard',
      );
    },
  );
}
