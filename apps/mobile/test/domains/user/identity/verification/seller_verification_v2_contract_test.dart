import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/domains/user/identity/verification/data/remote/verification_v2_datasource.dart';
import 'package:labuda/domains/user/identity/verification/data/repositories/seller_verification_repository_v2.dart';
import 'package:labuda/domains/user/identity/verification/presentation/providers/seller_verification_v2_provider.dart';
import 'package:labuda/domains/user/identity/verification/domain/entities/seller_verification_status.dart';
import 'package:labuda/domains/user/preference/seller/presentation/widgets/withdraw_dialog.dart';

class _NoopLogger implements ILoggerService {
  @override
  dynamic noSuchMethod(Invocation invocation) =>
      Future.value(Result.success(null));
}

class _FakeVerificationDatasource extends VerificationV2Datasource {
  _FakeVerificationDatasource(this._dto) : super(_DummyApiClient());
  final VerificationStatusDto? _dto;

  @override
  Future<VerificationStatusDto?> getVerificationStatus() async => _dto;
}

class _DummyApiClient implements ApiClient {
  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

void main() {
  group('Seller verification V2 contract', () {
    test('status DTO parses documents and rejection_note snake_case', () {
      final dto = VerificationStatusDto.fromJson({
        'status': 'needs_resubmission',
        'reason': 'please resubmit',
        'submitted_at': '2026-05-30T10:00:00Z',
        'reviewed_at': '2026-05-31T10:00:00Z',
        'documents': [
          {
            'id': 'doc-1',
            'document_type': 'identity_ktp',
            'document_url': 'https://example.com/doc.jpg',
            'status': 'rejected',
            'rejection_note': 'blurred',
            'submitted_at': '2026-05-30T10:00:00Z',
            'reviewed_at': '2026-05-31T10:00:00Z',
          },
        ],
      });

      expect(dto.documents, hasLength(1));
      expect(dto.documents.first['document_type'], 'identity_ktp');
      expect(dto.documents.first['rejection_note'], 'blurred');
      expect(dto.documents.first['submitted_at'], '2026-05-30T10:00:00Z');
      expect(dto.documents.first['reviewed_at'], '2026-05-31T10:00:00Z');
    });

    test(
      'repository uses status endpoint as single source for documents',
      () async {
        final repo = SellerVerificationRepositoryV2(
          datasource: _FakeVerificationDatasource(
            const VerificationStatusDto(
              status: 'pending_review',
              documents: [
                {
                  'id': 'doc-1',
                  'document_type': 'identity_ktp',
                  'status': 'pending',
                },
              ],
            ),
          ),
          logger: _NoopLogger(),
        );

        final docsResult = await repo.getDocuments();
        expect(docsResult.isSuccess, isTrue);
        expect(docsResult.data, hasLength(1));
        expect(docsResult.data!.first['id'], 'doc-1');
      },
    );

    test(
      'notifier loadStatus reads repository directly without setup',
      () async {
        final repo = SellerVerificationRepositoryV2(
          datasource: _FakeVerificationDatasource(
            const VerificationStatusDto(status: 'approved', documents: []),
          ),
          logger: _NoopLogger(),
        );

        final container = ProviderContainer(
          overrides: [
            sellerVerificationV2RepositoryProvider.overrideWithValue(repo),
          ],
        );
        addTearDown(container.dispose);

        await container
            .read(sellerVerificationV2NotifierProvider.notifier)
            .loadStatus();

        final state = container.read(sellerVerificationV2NotifierProvider);
        expect(state.status, SellerVerificationStatus.approved);
        expect(state.isVerified, isTrue);
        expect(state.isLoading, isFalse);
      },
    );

    test('delete document action throws UnsupportedError', () async {
      final repo = SellerVerificationRepositoryV2(
        datasource: _FakeVerificationDatasource(null),
        logger: _NoopLogger(),
      );

      expect(
        () => repo.deleteDocument('doc-1'),
        throwsA(isA<UnsupportedError>()),
      );
    });

    test('pending_review does not grant payout authority', () {
      const state = SellerVerificationV2State(
        isVerified: false,
        status: SellerVerificationStatus.pendingReview,
      );
      expect(hasPayoutAuthority(state), isFalse);
    });
  });
}
