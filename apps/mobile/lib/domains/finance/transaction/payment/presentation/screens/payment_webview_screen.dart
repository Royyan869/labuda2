import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:webview_flutter/webview_flutter.dart';

/// Payment WebView — SINGLE CANONICAL PAYMENT PRESENTATION SURFACE.
///
/// Payment URLs are presented exclusively inside Labuda's internal WebView.
/// External-browser payment navigation is obsolete and must not be reintroduced.
/// Payment completion remains backend-authoritative (webhook → order state →
/// PaymentResultNotifier). This screen never parses success URLs or marks
/// payments/orders as paid.
///
/// FINISH AUTO-CLOSE (universal — one authority for every payment machine:
/// VA, QRIS, wallets, cards, retail): every Snap transaction is created
/// server-side with `callbacks.finish = {FRONTEND_URL}/payment/finish` (see
/// `backend/internal/serverboot/midtrans_snap_builder.go` and the seller
/// subscription initiate handler in `seller_handler.go`). Previously that
/// redirect rendered a dead frontend route as an error page inside this
/// WebView. Now the redirect IS the close signal: when the finish path
/// appears, the gateway-side flow is done and this screen pops itself so the
/// awaiting flow resumes (checkout result, seller activation/renewal polling,
/// promotion payment).
///
/// EXTERNAL-APP HANDOFF (wallet & e-commerce channels): the Snap pages for
/// gopay/ovo/dana/shopeepay attempt to open the payer's wallet app through
/// custom schemes (`gojek://`, `ovo://`, `dana://`, `shopee://`, `intent://`).
/// Loading such a scheme inside a WebView dead-ends (Android:
/// ERR_UNKNOWN_URL_SCHEME). Instead, non-http navigations are handed to the
/// OS (external app launch, with the intent's browser-fallback URL when no
/// wallet app is installed) and the in-WebView load is prevented. A persistent
/// guidance banner explains the handoff. Launch failures never surface as
/// errors — the user completes payment on the Snap page or in the wallet.
///
/// HARD RULE: the close and the handoff are UX decisions only. This screen
/// NEVER settles, confirms, or marks any payment — settlement happens
/// exclusively through the backend (Midtrans webhook → settlement pipeline,
/// plus the discovery/reconciliation workers).
class PaymentWebviewScreen extends StatefulWidget {
  final String paymentUrl;

  /// Optional orderId for return-context wiring (buyer) or future resume.
  /// Seller subscription flow may omit it.
  final String? orderId;

  const PaymentWebviewScreen({
    super.key,
    required this.paymentUrl,
    this.orderId,
  });

  @override
  State<PaymentWebviewScreen> createState() => _PaymentWebviewScreenState();
}

class _PaymentWebviewScreenState extends State<PaymentWebviewScreen> {
  /// Canonical finish path of the server-created Snap callback. The host is
  /// FRONTEND_URL (unknown to the client), but the path is a fixed backend
  /// contract, so matching on the path alone is env-safe.
  static const String _finishPath = '/payment/finish';

  /// Schemes that belong to the WebView itself and must never trigger an
  /// external handoff.
  static const Set<String> _webviewInternalSchemes = {
    'http', 'https', 'about', 'data', 'blob', 'javascript',
  };

  late final WebViewController _controller;
  bool _isLoading = true;

  /// The finish redirect must target OUR frontend route, not a page on the
  /// gateway host the WebView started on (e.g. app.midtrans.com). Path-only
  /// matching plus this host guard keeps the auto-close precise.
  bool _isPaymentFinishRedirect(String url) {
    final target = Uri.tryParse(url);
    if (target == null || target.path != _finishPath) return false;
    final startHost = Uri.tryParse(widget.paymentUrl)?.host;
    if (startHost == null || startHost.isEmpty) return true;
    return target.host != startHost;
  }

  /// Wallet/e-commerce deep links (custom schemes) must be handled by the OS,
  /// not loaded into this WebView.
  static bool _isExternalAppNavigation(String url) {
    final scheme = Uri.tryParse(url)?.scheme.toLowerCase() ?? '';
    if (scheme.isEmpty) return false;
    return !_webviewInternalSchemes.contains(scheme);
  }

  /// Launches a wallet deep link outside the WebView. `intent://` URLs get
  /// their `S.browser_fallback_url` extracted (device without the wallet app)
  /// or degrade to a `fallback://` host so Android routes them to the store.
  Future<void> _launchExternalApp(String url) async {
    var target = url;
    if (target.toLowerCase().startsWith('intent://')) {
      final fallbackMatch = RegExp(
        r'S\.browser_fallback_url=([^;]+)',
        caseSensitive: false,
      ).firstMatch(target);
      if (fallbackMatch != null) {
        final fallback = Uri.tryParse(
          Uri.decodeComponent(fallbackMatch.group(1)!),
        );
        if (fallback != null) target = fallback.toString();
      } else {
        final parts = target.split('/');
        if (parts.length > 2) parts[2] = 'fallback';
        target = parts.join('/');
      }
    }
    final uri = Uri.tryParse(target);
    if (uri == null) return;
    try {
      await launchUrl(uri, mode: LaunchMode.externalApplication);
    } catch (_) {
      // No matching app installed / launch refused: the guidance banner stays
      // visible and the Snap page remains usable in this WebView.
    }
  }

  @override
  void initState() {
    super.initState();
    _controller = WebViewController()
      ..setJavaScriptMode(JavaScriptMode.unrestricted)
      ..setNavigationDelegate(
        NavigationDelegate(
          onPageStarted: (String url) {
            if (mounted) setState(() => _isLoading = true);
          },
          onPageFinished: (String url) {
            if (mounted) setState(() => _isLoading = false);
          },
          onNavigationRequest: (NavigationRequest request) {
            if (_isPaymentFinishRedirect(request.url)) {
              // Gateway-side flow finished (user reached the Snap result →
              // redirect). Close immediately — the awaiting flow owns the
              // outcome and polls the backend. This pop NEVER implies a paid
              // state.
              if (mounted) Navigator.of(context).pop();
              return NavigationDecision.prevent;
            }
            if (_isExternalAppNavigation(request.url)) {
              // Wallet handoff: prevent the in-WebView load (which would
              // dead-end with ERR_UNKNOWN_URL_SCHEME) and give the URL to the
              // OS instead. UX only — settlement stays backend-authoritative.
              _launchExternalApp(request.url);
              return NavigationDecision.prevent;
            }
            return NavigationDecision.navigate;
          },
        ),
      )
      ..loadRequest(Uri.parse(widget.paymentUrl));
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Pembayaran'),
        leading: IconButton(
          icon: const Icon(Icons.close),
          onPressed: () => Navigator.of(context).pop(),
        ),
      ),
      body: Stack(
        children: [
          WebViewWidget(controller: _controller),
          const Positioned(
            top: 0,
            left: 0,
            right: 0,
            child: _ExternalAppGuidanceBanner(),
          ),
          if (_isLoading)
            const Center(child: CircularProgressIndicator()),
        ],
      ),
    );
  }
}

/// Persistent guidance for wallet/e-commerce handoffs. Dismissible by the
/// user; reappears per screen open (stateless on purpose — one payment
/// session, one reminder).
class _ExternalAppGuidanceBanner extends StatefulWidget {
  const _ExternalAppGuidanceBanner();

  @override
  State<_ExternalAppGuidanceBanner> createState() =>
      _ExternalAppGuidanceBannerState();
}

class _ExternalAppGuidanceBannerState
    extends State<_ExternalAppGuidanceBanner> {
  bool _dismissed = false;

  @override
  Widget build(BuildContext context) {
    if (_dismissed) return const SizedBox.shrink();
    return Material(
      color: Colors.transparent,
      child: Container(
        width: double.infinity,
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        color: Theme.of(context).colorScheme.secondaryContainer,
        child: Row(
          children: [
            Icon(
              Icons.open_in_new,
              size: 16,
              color: Theme.of(context).colorScheme.onSecondaryContainer,
            ),
            const SizedBox(width: 8),
            Expanded(
              child: Text(
                'Untuk GoPay/OVO/DANA/ShopeePay, aplikasi terkait akan dibuka. '
                'Selesaikan pembayaran di sana, lalu kembali ke Labuda.',
                style: TextStyle(
                  fontSize: 12,
                  color: Theme.of(context).colorScheme.onSecondaryContainer,
                ),
              ),
            ),
            GestureDetector(
              onTap: () => setState(() => _dismissed = true),
              child: Icon(
                Icons.close,
                size: 16,
                color: Theme.of(context).colorScheme.onSecondaryContainer,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
