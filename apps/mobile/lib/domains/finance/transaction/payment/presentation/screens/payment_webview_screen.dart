import 'package:flutter/material.dart';
import 'package:webview_flutter/webview_flutter.dart';

/// Payment WebView — SINGLE CANONICAL PAYMENT PRESENTATION SURFACE.
///
/// Payment URLs are presented exclusively inside Labuda's internal WebView.
/// External-browser payment navigation is obsolete and must not be reintroduced.
/// Payment completion remains backend-authoritative (webhook → order state →
/// PaymentResultNotifier). This screen never parses success URLs or marks
/// payments/orders as paid.
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
  late final WebViewController _controller;
  bool _isLoading = true;

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
          // No external-browser fallback for payment URLs per hard rule §6.
          // All payment navigation stays inside this WebView.
          onNavigationRequest: (NavigationRequest request) {
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
          if (_isLoading)
            const Center(child: CircularProgressIndicator()),
        ],
      ),
    );
  }
}
