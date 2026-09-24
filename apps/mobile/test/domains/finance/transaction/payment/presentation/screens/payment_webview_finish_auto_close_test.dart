import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// Regression contract for the Payment WebView FINISH AUTO-CLOSE and the
/// EXTERNAL-APP HANDOFF (wallet channels).
///
/// Every Snap transaction is created server-side with
/// `callbacks.finish = {FRONTEND_URL}/payment/finish`. Before the auto-close
/// existed, that redirect rendered a dead frontend route as an error page
/// inside the WebView (user report: paying by card "succeeds" into an error
/// screen). And before the handoff existed, the Snap wallet pages'
/// custom-scheme deep links (gojek://, ovo://, dana://, shopee://, intent://)
/// dead-ended with ERR_UNKNOWN_URL_SCHEME inside the WebView.
///
/// These tests lock both behaviors. They never lock a settlement path: the
/// screen must remain backend-authoritative-only.
///
/// Source-contract style (repo convention): the WebView plugin cannot run
/// under `flutter test`, so the behavior is pinned against the screen source.
void main() {
  const screenPath =
      'lib/domains/finance/transaction/payment/presentation/screens/payment_webview_screen.dart';

  group('PaymentWebviewScreen — finish auto-close contract', () {
    test('detects the canonical backend finish path /payment/finish', () {
      final source = File(screenPath).readAsStringSync();

      expect(
        source.contains("_finishPath = '/payment/finish'"),
        isTrue,
        reason:
            'the finish path is a fixed backend contract '
            '(midtrans_snap_builder.go / seller initiate handler); the webview '
            'must keep matching it to auto-close for ALL payment machines',
      );
    });

    test('finish redirect pops the webview and prevents further navigation', () {
      final source = File(screenPath).readAsStringSync();

      final navStart = source.indexOf('onNavigationRequest:');
      expect(navStart, greaterThan(-1), reason: 'navigation delegate required');
      final navEnd = source.indexOf('..loadRequest', navStart);
      expect(navEnd, greaterThan(navStart));
      final navBlock = source.substring(navStart, navEnd);

      expect(
        navBlock.contains('_isPaymentFinishRedirect(request.url)'),
        isTrue,
        reason: 'the finish check must run inside onNavigationRequest',
      );
      final popIndex = navBlock.indexOf('Navigator.of(context).pop()');
      final preventIndex = navBlock.indexOf('NavigationDecision.prevent');
      expect(popIndex, greaterThan(-1), reason: 'auto-close must pop the screen');
      expect(
        preventIndex,
        greaterThan(popIndex),
        reason: 'the finish page itself must never render inside the webview',
      );
    });

    test('finish redirect only matches OUR frontend host, not the gateway host', () {
      final source = File(screenPath).readAsStringSync();

      expect(
        source.contains('target.host != startHost'),
        isTrue,
        reason:
            'path-only matching without the host guard would auto-close on any '
            'gateway-hosted page that happens to end in /payment/finish',
      );
    });
  });

  group('PaymentWebviewScreen — external-app handoff contract (P1)', () {
    test('custom schemes (wallets) are handed to the OS, never webview-loaded', () {
      final source = File(screenPath).readAsStringSync();

      final navStart = source.indexOf('onNavigationRequest:');
      final navEnd = source.indexOf('..loadRequest', navStart);
      final navBlock = source.substring(navStart, navEnd);

      expect(
        navBlock.contains('_isExternalAppNavigation(request.url)'),
        isTrue,
        reason:
            'gojek://, ovo://, dana://, shopee:// and intent:// navigations '
            'must be intercepted in onNavigationRequest — loading them in the '
            'webview dead-ends with ERR_UNKNOWN_URL_SCHEME',
      );
      expect(
        navBlock.contains('_launchExternalApp(request.url)'),
        isTrue,
        reason: 'the intercepted scheme must be launched outside the webview',
      );
    });

    test('launch uses external-application mode with error swallowing', () {
      final source = File(screenPath).readAsStringSync();

      expect(
        source.contains('launchUrl(uri, mode: LaunchMode.externalApplication)'),
        isTrue,
        reason:
            'wallet apps only respond to external launches; in-app launches '
            'would fail or open a browser on top of the payment flow',
      );
      expect(
        source.contains('catch (_) {'),
        isTrue,
        reason:
            'a missing wallet app must never surface as an error — the Snap '
            'page stays usable inside the webview',
      );
    });

    test('intent:// URLs resolve the browser fallback before launching', () {
      final source = File(screenPath).readAsStringSync();

      expect(
        source.contains('S.browser_fallback_url'),
        isTrue,
        reason:
            'devices without the wallet app need the intent browser fallback; '
            'launching the raw intent:// scheme would fail for them',
      );
    });

    test('internal webview schemes never trigger the handoff', () {
      final source = File(screenPath).readAsStringSync();

      for (final scheme in ["'http'", "'https'", "'javascript'", "'data'"]) {
        expect(
          source.contains(scheme),
          isTrue,
          reason:
              '$scheme must stay webview-internal: only custom schemes may '
              'leave the webview',
        );
      }
    });

    test('a guidance banner explains the wallet handoff', () {
      final source = File(screenPath).readAsStringSync();

      expect(
        source.contains('_ExternalAppGuidanceBanner'),
        isTrue,
        reason:
            'users need to know why their wallet app opened and that they '
            'must return to Labuda afterwards',
      );
      expect(
        source.contains('aplikasi terkait akan dibuka'),
        isTrue,
      );
    });
  });

  group('PaymentWebviewScreen — HARD RULE: backend-authoritative only', () {
    test('the webview never settles, confirms, or marks payments', () {
      final source = File(screenPath).readAsStringSync();

      const forbiddenSettlementIdentifiers = [
        'markAsPaid',
        'PaymentStatus.settled',
        'PaymentStatus.paid',
        'settlePayment',
        'confirmPayment',
      ];
      for (final identifier in forbiddenSettlementIdentifiers) {
        expect(
          source.contains(identifier),
          isFalse,
          reason:
              'closing and the handoff are UX only — settlement is exclusively '
              'backend-authoritative (webhook/workers); "$identifier" must '
              'never appear in the payment webview',
        );
      }
    });
  });
}
