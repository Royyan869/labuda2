// ============================================================================
// CANONICAL PROMOTION FUNDING — CREATE SCREEN CONTRACT
//
// Proves the mobile seller funding boundary against the canonical backend:
//   F1: the reusable funding number rendered on the create screen comes from
//       GET /promote-balance (never computed locally).
//   A:  reuse path — when the backend reports no payment is required, Create
//       runs directly: no disclosure call, no initiation call, no picker.
//   B:  payment-required path — POST /promotions/contracts/payment-intent is
//       the funding gate, the disclosure endpoint supplies every method with its
//       fee and gross, and an underfunded promotion is never created.
//   C:  the selected canonical method_code is sent verbatim to the pay
//       endpoint, the canonical payment WebView opens, and creation resumes
//       through the same gate once funding is settled.
//   D:  404 / 403 / 409 / transport failures surface an explicit message with a
//       retry — never a locally invented method list or fee.
//   E:  no legacy status vocabulary and no wallet/top-up wording.
// ============================================================================

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/promotion/presentation/screens/canonical_promotion_create_screen.dart';

const _gatePath = '/promotions/contracts/payment-intent';
const _payPath = '/promotions/contracts/payment-intent/intent-1/pay';
const _disclosurePath =
    '/promotions/contracts/payment-intent/intent-1/payment-methods';

/// Api client that dispatches canned envelopes by path and records every call.
///
/// It models the backend truth: the gate reports the exact shortage until a
/// payment has been initiated, and the disclosure numbers are backend-computed
/// values the client must render verbatim.
class _FundingApiClient implements ApiClient {
  int balance;
  bool paymentRequired;
  Object? gateError;
  Object? disclosureError;
  Object? payError;

  final List<String> calls = [];
  final List<dynamic> postPayloads = [];

  _FundingApiClient({this.balance = 0, this.paymentRequired = true});

  Response<dynamic> _ok(String path, Map<String, dynamic> data) =>
      Response<dynamic>(
        requestOptions: RequestOptions(path: path),
        data: data,
        statusCode: 200,
      );

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    calls.add('GET $path');
    if (path == '/promote-balance') {
      return _ok(path, {
            'data': {'balance': balance},
          })
          as Response<T>;
    }
    if (path == _disclosurePath) {
      if (disclosureError != null) throw disclosureError!;
      return _ok(path, {
            'data': {
              'shortage_amount': 20000,
              'currency': 'IDR',
              'methods': [
                {
                  'method_code': 'gopay',
                  'display_name': 'GoPay',
                  'service_fee_amount': 300,
                  'gross_amount': 20300,
                },
                {
                  'method_code': 'ovo',
                  'display_name': 'OVO',
                  'service_fee_amount': 250,
                  'gross_amount': 20250,
                },
              ],
            },
          })
          as Response<T>;
    }
    return _ok(path, {'data': <String, dynamic>{}}) as Response<T>;
  }

  @override
  Future<Response<T>> post<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    calls.add('POST $path');
    postPayloads.add(data);
    if (path == _gatePath) {
      if (gateError != null) throw gateError!;
      if (!paymentRequired) {
        return _ok(path, {
              'data': {
                'intent': {
                  'payment_required': false,
                  'shortage': 0,
                  'required_cost': 30000,
                  'available_funding': 30000,
                },
              },
            })
            as Response<T>;
      }
      return _ok(path, {
            'data': {
              'intent': {
                'payment_required': true,
                'shortage': 20000,
                'intent_id': 'intent-1',
                'billing_id': 'bill-1',
                'required_cost': 30000,
                'available_funding': 10000,
              },
            },
          })
          as Response<T>;
    }
    if (path == _payPath) {
      if (payError != null) throw payError!;
      // A settled payment credits reusable PROMOTE_BALANCE, so the gate that
      // runs right after the WebView now reports no payment required.
      paymentRequired = false;
      return _ok(path, {
            'data': {
              'payment_id': 'pay-1',
              'payment_url': 'https://snap.example/redirect',
              'gross_amount': 20300,
              'shortage': 20000,
              'intent_id': 'intent-1',
            },
          })
          as Response<T>;
    }
    if (path == '/promotions/contracts') {
      return _ok(path, {
            'data': {
              'contract': {
                'id': 'ctr-new',
                'seller_id': 'seller-1',
                'kind': 'internal',
                'status': 'active',
                'budget_rupiah': 30000,
                'cpm_rupiah': 7500,
                'planned_start': '2026-09-01T00:00:00Z',
                'planned_finish': '2026-09-04T00:00:00Z',
                'allocation_account_id': 'alloc-1',
                'paused_at': null,
                'finalized_at': null,
                'created_at': '2026-09-01T00:00:00Z',
                'updated_at': '2026-09-01T00:00:00Z',
                'city_ids': <String>[],
              },
            },
          })
          as Response<T>;
    }
    return _ok(path, {'data': <String, dynamic>{}}) as Response<T>;
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

Widget _harness(_FundingApiClient client) {
  // The create route is nested under the list route so the router owns a real
  // back stack — the screen pops to the list after a successful create. The
  // canonical payment WebView is the only payment surface (never a browser).
  final router = GoRouter(
    initialLocation: '/seller/canonical-promotions/create',
    routes: [
      GoRoute(
        path: '/seller/canonical-promotions',
        builder: (context, state) =>
            const Scaffold(body: Text('promotion-list')),
        routes: [
          GoRoute(
            path: 'create',
            builder: (context, state) => const CanonicalPromotionCreateScreen(),
          ),
        ],
      ),
      GoRoute(
        path: '/payment-webview',
        builder: (context, state) => Scaffold(
          body: Column(
            children: [
              const Text('payment-webview-stub'),
              Text(state.uri.queryParameters['url'] ?? ''),
              ElevatedButton(
                onPressed: () => context.pop(),
                child: const Text('Tutup WebView'),
              ),
            ],
          ),
        ),
      ),
    ],
  );
  return ProviderScope(
    overrides: [apiClientProvider.overrideWithValue(client)],
    child: MaterialApp.router(routerConfig: router),
  );
}

/// Advances animations and microtasks without settling (the screens render
/// progress indicators that never settle).
Future<void> _flush(WidgetTester tester) async {
  for (var i = 0; i < 5; i++) {
    await tester.pump(const Duration(milliseconds: 120));
  }
  await tester.pump();
  await tester.pump();
}

Future<void> _openForm(WidgetTester tester) async {
  await tester.enterText(find.byType(TextFormField).at(0), '30000');
  await tester.enterText(find.byType(TextFormField).at(1), '3');
  await tester.tap(find.widgetWithText(ElevatedButton, 'Buat Promosi'));
  await _flush(tester);
}

void main() {
  testWidgets(
    'F1: reusable funding renders the backend PROMOTE_BALANCE value',
    (tester) async {
      final client = _FundingApiClient(balance: 42500, paymentRequired: false);

      await tester.pumpWidget(_harness(client));
      await _flush(tester);

      expect(client.calls, contains('GET /promote-balance'));
      expect(find.textContaining('42.500'), findsOneWidget);
    },
  );

  testWidgets(
    'A: sufficient reusable funding creates with no payment surface at all',
    (tester) async {
      final client = _FundingApiClient(balance: 30000, paymentRequired: false);

      await tester.pumpWidget(_harness(client));
      await _flush(tester);
      await _openForm(tester);

      expect(client.calls, contains('POST $_gatePath'));
      expect(client.calls, contains('POST /promotions/contracts'));
      expect(
        client.calls.where((c) => c.contains('/payment-methods')),
        isEmpty,
        reason: 'the reuse path must never open the payment-method disclosure',
      );
      expect(
        client.calls.where((c) => c.endsWith('/pay')),
        isEmpty,
        reason: 'the reuse path must never initiate a payment',
      );
      expect(find.text('Kekurangan dana promosi'), findsNothing);
      expect(find.text('Promosi berhasil dibuat'), findsOneWidget);
    },
  );

  testWidgets(
    'B: exact shortage loads backend methods/fee/total and blocks creation',
    (tester) async {
      final client = _FundingApiClient(balance: 10000);

      await tester.pumpWidget(_harness(client));
      await _flush(tester);
      await _openForm(tester);

      expect(client.calls, contains('POST $_gatePath'));
      expect(client.calls, contains('GET $_disclosurePath'));
      expect(
        client.calls,
        isNot(contains('POST /promotions/contracts')),
        reason: 'an underfunded promotion must never be created',
      );

      // Backend numbers rendered verbatim in the sheet: required cost, available
      // funding and the exact shortage — the shortage is the obligation, not the
      // promotion budget.
      final sheet = find.byType(BottomSheet);
      expect(find.text('Kekurangan dana promosi'), findsOneWidget);
      expect(
        find.descendant(of: sheet, matching: find.text('Rp 30.000')),
        findsOneWidget,
      );
      expect(
        find.descendant(of: sheet, matching: find.text('Rp 10.000')),
        findsOneWidget,
      );
      expect(
        find.descendant(of: sheet, matching: find.text('Rp 20.000')),
        findsOneWidget,
      );

      // Methods come from the backend disclosure, each with its own fee/total.
      await tester.tap(find.byIcon(Icons.chevron_right));
      await _flush(tester);
      expect(find.text('GoPay'), findsOneWidget);
      expect(find.text('OVO'), findsOneWidget);
      expect(find.text('Biaya layanan: Rp 300'), findsOneWidget);
      expect(find.text('Rp 20.300'), findsOneWidget);
      expect(find.text('Rp 20.250'), findsOneWidget);
    },
  );

  testWidgets(
    'C: selected method code is sent verbatim, then creation resumes after payment',
    (tester) async {
      final client = _FundingApiClient(balance: 10000);

      await tester.pumpWidget(_harness(client));
      await _flush(tester);
      await _openForm(tester);

      // Pick the canonical method code from the backend list.
      await tester.tap(find.byIcon(Icons.chevron_right));
      await _flush(tester);
      await tester.tap(find.text('GoPay'));
      await _flush(tester);

      expect(find.text('Rp 300'), findsOneWidget); // service fee F
      expect(find.text('Rp 20.300'), findsOneWidget); // gross = shortage + F

      await tester.tap(find.widgetWithText(ElevatedButton, 'Bayar Kekurangan'));
      await _flush(tester);

      expect(
        client.calls,
        contains('POST $_payPath'),
        reason: 'initiation uses the canonical pay path',
      );
      // The payload carries exactly one key — the canonical method code. The
      // client never sends an amount, a fee, or a gross total.
      final payPayloads = client.postPayloads
          .whereType<Map>()
          .where((p) => p['payment_method_code'] == 'gopay')
          .toList();
      expect(payPayloads, hasLength(1));
      expect(payPayloads.single.keys.toList(), ['payment_method_code']);

      // The canonical internal WebView is the only payment surface.
      expect(find.text('payment-webview-stub'), findsOneWidget);
      expect(find.text('https://snap.example/redirect'), findsOneWidget);

      await tester.tap(find.text('Tutup WebView'));
      await _flush(tester);

      // Returned from payment: the same gate now reports no payment required,
      // so the canonical Create proceeds. No duplicate obligation, no second
      // payment attempt.
      expect(
        client.calls.where((c) => c == 'POST $_payPath').length,
        1,
        reason: 'a retry must never create a second payment',
      );
      expect(client.calls, contains('POST /promotions/contracts'));
      expect(find.text('Promosi berhasil dibuat'), findsOneWidget);
      expect(
        find.text(
          'Pembayaran belum terkonfirmasi. Tunggu sebentar, lalu coba lagi.',
        ),
        findsNothing,
      );
    },
  );

  testWidgets(
    'D: 404 and 403 obligations surface explicit messages with retry',
    (tester) async {
      final notFound = _FundingApiClient(balance: 10000)
        ..disclosureError = const NotFoundException(message: 'not found');
      await tester.pumpWidget(_harness(notFound));
      await _flush(tester);
      await _openForm(tester);

      expect(
        find.textContaining('Pengajuan dana promosi tidak ditemukan'),
        findsOneWidget,
      );
      expect(find.text('Coba lagi'), findsOneWidget);
      expect(find.text('GoPay'), findsNothing);
      expect(notFound.calls, isNot(contains('POST /promotions/contracts')));

      final forbidden = _FundingApiClient(balance: 10000)
        ..disclosureError = const ForbiddenException(message: 'not yours');
      await tester.pumpWidget(_harness(forbidden));
      await _flush(tester);
      await _openForm(tester);

      expect(
        find.textContaining('Anda hanya dapat membayar kekurangan dana'),
        findsOneWidget,
      );
    },
  );

  testWidgets('D: 409 on initiation is surfaced and retryable', (tester) async {
    final client = _FundingApiClient(balance: 10000)
      ..payError = const ConflictException(message: 'billing is not pending');

    await tester.pumpWidget(_harness(client));
    await _flush(tester);
    await _openForm(tester);

    await tester.tap(find.byIcon(Icons.chevron_right));
    await _flush(tester);
    await tester.tap(find.text('OVO'));
    await _flush(tester);

    await tester.tap(find.widgetWithText(ElevatedButton, 'Bayar Kekurangan'));
    await _flush(tester);

    expect(
      find.textContaining('Pengajuan pembayaran ini sudah tidak berlaku'),
      findsOneWidget,
    );
    // The sheet stays open on the same obligation so the seller can retry.
    expect(find.text('Kekurangan dana promosi'), findsOneWidget);
    expect(find.text('payment-webview-stub'), findsNothing);
    expect(client.calls, isNot(contains('POST /promotions/contracts')));
  });

  testWidgets(
    'D: transport failure on the funding gate never fabricates funding',
    (tester) async {
      final client = _FundingApiClient(balance: 10000)
        ..gateError = const NetworkException(message: 'Koneksi bermasalah');

      await tester.pumpWidget(_harness(client));
      await _flush(tester);
      await _openForm(tester);

      expect(find.text('Koneksi bermasalah'), findsOneWidget);
      expect(client.calls, isNot(contains('POST /promotions/contracts')));
      expect(
        client.calls.where((c) => c.contains('/payment-methods')),
        isEmpty,
        reason: 'no obligation means no disclosure request',
      );
    },
  );

  testWidgets('E: no legacy status or wallet/top-up vocabulary is used', (
    tester,
  ) async {
    final client = _FundingApiClient(balance: 30000);
    await tester.pumpWidget(_harness(client));
    await _flush(tester);
    await _openForm(tester);

    for (final banned in <String>[
      'Wallet',
      'Top Up',
      'Top-up',
      'Deposit',
      'Didanai',
      'Eligible',
      'Dibatalkan',
      'Isi Saldo',
    ]) {
      expect(
        find.textContaining(banned),
        findsNothing,
        reason: 'obsolete/non-canonical wording "$banned" must not appear',
      );
    }
  });
}
