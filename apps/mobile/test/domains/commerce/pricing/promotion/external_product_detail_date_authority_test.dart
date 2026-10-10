// EXTERNAL PRODUCT DETAIL — DATE/TIME AUTHORITY CONVERGENCE GATE.
//
// Owner decision (authority purge):
//   Product Info timestamps (Created / Updated / Submitted / Approved) render
//   through canonical AppFormatters.formatDateTime (id_ID). The private
//   unlocalized `DateFormat('dd MMM yyyy, HH:mm')` helper is purged.
//
// Proven canonical month abbreviations (id_ID) that expose the locale
// difference vs English defaults:
//   May → Mei, August → Agu, October → Okt, December → Des
//
// This gate proves the REAL ExternalProductDetailScreen Product Info section,
// source residue (no private formatter), and that AppFormatters remains the
// sole authority on this surface.
//
// It does NOT modify AppFormatters, bootstrap, TimeFormatService, DTO parsing,
// order timezone policy, or any other timestamp consumer.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:intl/date_symbol_data_local.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/commerce/pricing/promotion/data/repositories/external_product_repository.dart';
import 'package:hishumi/domains/commerce/pricing/promotion/domain/entities/external_product.dart';
import 'package:hishumi/domains/commerce/pricing/promotion/domain/entities/external_product_media.dart';
import 'package:hishumi/domains/commerce/pricing/promotion/domain/entities/external_product_review_status.dart';
import 'package:hishumi/domains/commerce/pricing/promotion/presentation/screens/external_product_detail_screen.dart';
import 'package:hishumi/domains/commerce/pricing/promotion/presentation/providers/canonical_external_product_providers.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/shared/utils/app_formatters.dart';

const String _productId = 'ep-date-authority-1';
const String _screenPath =
    'lib/domains/commerce/pricing/promotion/presentation/screens/'
    'external_product_detail_screen.dart';

/// Deterministic dates that expose id_ID vs English month abbreviations.
final DateTime _created = DateTime(2024, 5, 15, 10, 30); // May → Mei
final DateTime _updated = DateTime(2024, 8, 12, 9, 15); // August → Agu
final DateTime _submitted = DateTime(2024, 10, 20, 14, 45); // October → Okt
final DateTime _approved = DateTime(2024, 12, 31, 23, 5); // December → Des

final String _createdText = AppFormatters.formatDateTime(_created);
final String _updatedText = AppFormatters.formatDateTime(_updated);
final String _submittedText = AppFormatters.formatDateTime(_submitted);
final String _approvedText = AppFormatters.formatDateTime(_approved);

class _FakeExternalProductRepository implements ExternalProductRepository {
  _FakeExternalProductRepository(this.product);

  final ExternalProduct product;

  @override
  Future<Result<ExternalProduct>> getExternalProduct(String id) async =>
      Result.success(product);

  @override
  Future<Result<ExternalProduct>> createExternalProductDraft({
    required String title,
    required String externalUrl,
    String? description,
  }) => throw UnimplementedError();

  @override
  Future<Result<ExternalProduct>> updateExternalProduct({
    required String id,
    String? title,
    String? description,
    String? externalUrl,
  }) => throw UnimplementedError();

  @override
  Future<Result<ExternalProduct>> submitExternalProduct({
    required String id,
    String? note,
  }) => throw UnimplementedError();

  @override
  Future<Result<ExternalProduct>> resubmitExternalProduct({
    required String id,
    String? note,
  }) => throw UnimplementedError();

  @override
  Future<Result<List<ExternalProduct>>> listMyExternalProducts() =>
      throw UnimplementedError();

  @override
  Future<Result<ExternalProductMedia>> attachExternalProductMedia({
    required String externalProductId,
    required String mediaType,
    required String storageKey,
    required String url,
    String? thumbnailUrl,
    int? sortOrder,
  }) => throw UnimplementedError();

  @override
  Future<Result<List<ExternalProductMedia>>> listExternalProductMedia(
    String externalProductId,
  ) => throw UnimplementedError();

  @override
  Future<Result<void>> deleteExternalProductMedia({
    required String externalProductId,
    required String mediaId,
  }) => throw UnimplementedError();
}

ExternalProduct _product() => ExternalProduct(
  id: _productId,
  ownerUserId: 'seller-1',
  title: 'Koi Kohaku Premium',
  description: 'External listing for promotion targeting',
  externalUrl: 'https://example.com/koi-kohaku',
  normalizedExternalUrl: 'https://example.com/koi-kohaku',
  reviewStatus: ExternalProductReviewStatus.approved,
  rejectionReason: null,
  unsafeUrlFlag: false,
  submittedAt: _submitted,
  approvedAt: _approved,
  createdAt: _created,
  updatedAt: _updated,
  media: const <ExternalProductMedia>[],
  canEdit: true,
  canSubmit: false,
  canResubmit: false,
  publicVisible: true,
);

Future<void> _pumpScreen(WidgetTester tester) async {
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        externalProductRepositoryProvider.overrideWithValue(
          _FakeExternalProductRepository(_product()),
        ),
      ],
      child: MaterialApp(
        theme: AppTheme.lightTheme,
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('id'),
        home: const ExternalProductDetailScreen(productId: _productId),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(() async {
    await initializeDateFormatting('id_ID');
  });

  group('External Product Detail — AppFormatters date authority', () {
    test('canonical AppFormatters output uses Indonesian month abbreviations', () {
      expect(_createdText, '15 Mei 2024, 10:30');
      expect(_updatedText, '12 Agu 2024, 09:15');
      expect(_submittedText, '20 Okt 2024, 14:45');
      expect(_approvedText, '31 Des 2024, 23:05');

      // Cross-check the production authority itself.
      expect(AppFormatters.formatDateTime(_created), _createdText);
      expect(AppFormatters.formatDateTime(_updated), _updatedText);
      expect(AppFormatters.formatDateTime(_submitted), _submittedText);
      expect(AppFormatters.formatDateTime(_approved), _approvedText);
    });

    testWidgets(
      'Product Info renders all four timestamps through AppFormatters',
      (tester) async {
        await _pumpScreen(tester);

        expect(tester.takeException(), isNull);

        // Product Info structure remains intact.
        expect(find.text('Product Info'), findsOneWidget);
        expect(find.text('Created'), findsOneWidget);
        expect(find.text('Updated'), findsOneWidget);
        expect(find.text('Submitted'), findsOneWidget);
        expect(find.text('Approved'), findsOneWidget);

        // Canonical Indonesian month output appears for every field.
        expect(find.text(_createdText), findsOneWidget);
        expect(find.text(_updatedText), findsOneWidget);
        expect(find.text(_submittedText), findsOneWidget);
        expect(find.text(_approvedText), findsOneWidget);

        // English month alternatives must not appear.
        expect(find.textContaining('May 2024'), findsNothing);
        expect(find.textContaining('Aug 2024'), findsNothing);
        expect(find.textContaining('Oct 2024'), findsNothing);
        expect(find.textContaining('Dec 2024'), findsNothing);
      },
    );

    testWidgets(
      'Product Info values stay inside the expanded label/value cells',
      (tester) async {
        await _pumpScreen(tester);
        expect(tester.takeException(), isNull);

        for (final String expected in <String>[
          _createdText,
          _updatedText,
          _submittedText,
          _approvedText,
        ]) {
          final Finder finder = find.text(expected);
          expect(finder, findsOneWidget);
          final Rect rect = tester.getRect(finder);
          expect(rect.left, greaterThanOrEqualTo(-0.5));
          expect(rect.right, lessThanOrEqualTo(tester.getSize(find.byType(Scaffold)).width + 0.5));
        }
      },
    );

    test('production source has no private local date formatter residue', () {
      final String source = File(_screenPath)
          .readAsStringSync()
          .replaceAll('\r\n', '\n');

      expect(source.contains('_dateTime'), isFalse);
      expect(source.contains('DateFormat('), isFalse);
      expect(source.contains("import 'package:intl/intl.dart'"), isFalse);
      expect(
        source.contains('AppFormatters.formatDateTime(product.createdAt)'),
        isTrue,
      );
      expect(
        source.contains('AppFormatters.formatDateTime(product.updatedAt)'),
        isTrue,
      );
      expect(
        source.contains('AppFormatters.formatDateTime(product.submittedAt!)'),
        isTrue,
      );
      expect(
        source.contains('AppFormatters.formatDateTime(product.approvedAt!)'),
        isTrue,
      );
      expect(source.contains('.toLocal()'), isFalse);
    });
  });
}
