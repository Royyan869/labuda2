import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/api/exceptions/api_exception.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_subscription.dart';
import 'package:labuda/domains/user/preference/seller/data/remote/seller_remote_datasource.dart';
import 'package:labuda/domains/user/preference/seller/data/repositories/seller_repository_impl.dart';

class _RecordingApiClient implements ApiClient {
  String? lastGetPath;
  String? lastPostPath;
  Map<String, dynamic>? lastGetQuery;
  int responseStatusCode = 200;

  dynamic getPayload = <String, dynamic>{'data': <String, dynamic>{}};
  dynamic postPayload = <String, dynamic>{'data': <String, dynamic>{}};

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    lastGetPath = path;
    lastGetQuery = queryParameters;
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      data: getPayload as T,
      statusCode: responseStatusCode,
    );
  }

  @override
  Future<Response<T>> post<T>(
    String path, {
    data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    lastPostPath = path;
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      data: postPayload as T,
      statusCode: responseStatusCode,
    );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeSellerRemoteDatasource extends SellerRemoteDatasource {
  final Map<String, dynamic> subscriptionPayload;

  _FakeSellerRemoteDatasource(this.subscriptionPayload)
    : super(apiClient: _RecordingApiClient());

  @override
  Future<Map<String, dynamic>> getSubscription(String sellerId) async {
    return subscriptionPayload;
  }
}

void main() {
  group('Seller endpoint contract', () {
    test('dashboard endpoint uses self route (no sellerId segment)', () async {
      final client = _RecordingApiClient()
        ..getPayload = {
          'data': {
            'total_orders': 0,
            'pending_orders': 0,
            'processing_orders': 0,
            'completed_orders': 0,
            'cancelled_orders': 0,
            'refunded_orders': 0,
            'problematic_orders': 0,
            'total_revenue': 0,
            'pending_revenue': 0,
            'refunded_revenue': 0,
            'total_collections': 0,
            'active_collections': 0,
            'sold_collections': 0,
            'total_auctions': 0,
            'active_auctions': 0,
          },
        };
      final ds = SellerRemoteDatasource(apiClient: client);

      await ds.getDashboardStats('seller-123');
      expect(client.lastGetPath, '/seller/dashboard');
    });

    test(
      'subscription/performance endpoints use self route (no sellerId segment)',
      () async {
        final client = _RecordingApiClient()
          ..getPayload = {'data': <String, dynamic>{}};
        final ds = SellerRemoteDatasource(apiClient: client);

        await ds.getSubscription('seller-123');
        expect(client.lastGetPath, '/seller/subscription');

        await ds.getPerformance('seller-123');
        expect(client.lastGetPath, '/seller/performance');
      },
    );

    test('onboarding/initiate keep canonical seller routes', () async {
      final client = _RecordingApiClient();
      final ds = SellerRemoteDatasource(apiClient: client);

      await ds.performOnboarding('Store Name');
      expect(client.lastPostPath, '/seller/onboarding');

      client.postPayload = {
        'data': {
          'payment_id': 'p1',
          'payment_url': 'https://pay',
          'gross_amount': 1000,
          'expired_at': '2026-01-01T00:00:00Z',
        },
      };
      await ds.initiateSubscriptionPayment();
      expect(client.lastPostPath, '/seller/subscription/initiate');
    });

    test(
      'seller onboarding 400 becomes typed exception and blocks payment',
      () async {
        final client = _RecordingApiClient()
          ..responseStatusCode = 400
          ..postPayload = {
            'success': false,
            'error': {
              'code': 'MISSING_REQUIREMENTS',
              'message': 'Missing verification requirements',
              'details': {
                'requires_verification': ['phone_number', 'username'],
              },
            },
          };
        final ds = SellerRemoteDatasource(apiClient: client);

        await expectLater(
          () => ds.performOnboarding('Store Name'),
          throwsA(
            isA<BadRequestException>().having(
              (e) => e.code,
              'code',
              'MISSING_REQUIREMENTS',
            ),
          ),
        );
      },
    );

    test(
      'unsupported legacy seller endpoints throw explicit UnsupportedError',
      () async {
        final client = _RecordingApiClient();
        final ds = SellerRemoteDatasource(apiClient: client);

        expect(
          () => ds.getSalesTrendData(sellerId: 's1'),
          throwsA(isA<UnsupportedError>()),
        );
        expect(
          () => ds.getRecentActivity('s1'),
          throwsA(isA<UnsupportedError>()),
        );
        expect(
          () => ds.getActivityHistory('s1'),
          throwsA(isA<UnsupportedError>()),
        );
      },
    );
  });

  group('Seller DTO/status mapping', () {
    test('subscription parses snake_case backend payload', () async {
      final repo = SellerRepositoryImpl(
        remoteDatasource: _FakeSellerRemoteDatasource({
          'is_active': true,
          'yearly_fee': 1250000,
          'start_date': '2026-01-01T00:00:00Z',
          'expiry_date': '2027-01-01T00:00:00Z',
          'status': 'expired',
          'payment_id': 'pay_123',
          'created_at': '2026-01-01T00:00:00Z',
        }),
      );

      final result = await repo.getSubscription('seller-123');
      expect(result.isSuccess, isTrue);
      final sub = result.data!;
      expect(sub.isActive, isTrue);
      expect(sub.paymentId, 'pay_123');
      expect(sub.status, SubscriptionStatus.expired);
    });
  });
}
