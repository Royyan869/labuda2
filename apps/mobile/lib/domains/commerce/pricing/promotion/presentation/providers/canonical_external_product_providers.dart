/// Canonical External Product providers.
///
/// External products are a canonical promotion asset (admin-reviewed,
/// seller-owned). The legacy package/ownership/instance/discovery
/// providers are purged — this file is the only promotion provider
/// surface besides the contract providers.
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/repositories/external_product_repository.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/external_product.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/external_product_media.dart';

/// Canonical external product repository provider.
final externalProductRepositoryProvider = Provider<ExternalProductRepository>(
  (ref) {
    final apiClient = ref.watch(apiClientProvider);
    return ExternalProductRepositoryImpl(apiClient);
  },
);

/// My external products provider (GET /promotions/my/external-products).
final myExternalProductsProvider =
    FutureProvider.autoDispose<Result<List<ExternalProduct>>>((ref) async {
      final repository = ref.watch(externalProductRepositoryProvider);
      return repository.listMyExternalProducts();
    });

/// External product detail provider (GET /promotions/external-products/:id).
final externalProductDetailProvider = FutureProvider.autoDispose
    .family<Result<ExternalProduct>, String>((ref, productId) async {
      final repository = ref.watch(externalProductRepositoryProvider);
      return repository.getExternalProduct(productId);
    });

/// External product media list provider.
final externalProductMediaProvider = FutureProvider.autoDispose
    .family<Result<List<ExternalProductMedia>>, String>((ref, productId) async {
      final repository = ref.watch(externalProductRepositoryProvider);
      return repository.listExternalProductMedia(productId);
    });

/// External product write controller (create/update/submit/media).
final externalProductControllerProvider =
    Provider<ExternalProductController>((ref) {
      final repository = ref.watch(externalProductRepositoryProvider);
      return ExternalProductController(repository);
    });

/// Controller for external product write operations.
class ExternalProductController {
  final ExternalProductRepository _repository;

  ExternalProductController(this._repository);

  Future<Result<ExternalProduct>> createExternalProductDraft({
    required String title,
    required String externalUrl,
    String? description,
  }) {
    return _repository.createExternalProductDraft(
      title: title,
      externalUrl: externalUrl,
      description: description,
    );
  }

  Future<Result<ExternalProduct>> updateExternalProduct({
    required String id,
    String? title,
    String? description,
    String? externalUrl,
  }) {
    return _repository.updateExternalProduct(
      id: id,
      title: title,
      description: description,
      externalUrl: externalUrl,
    );
  }

  Future<Result<ExternalProduct>> submitExternalProduct({
    required String id,
    String? note,
  }) {
    return _repository.submitExternalProduct(id: id, note: note);
  }

  Future<Result<ExternalProduct>> resubmitExternalProduct({
    required String id,
    String? note,
  }) {
    return _repository.resubmitExternalProduct(id: id, note: note);
  }

  Future<Result<ExternalProductMedia>> attachExternalProductMedia({
    required String externalProductId,
    required String mediaType,
    required String storageKey,
    required String url,
    String? thumbnailUrl,
    int? sortOrder,
  }) {
    return _repository.attachExternalProductMedia(
      externalProductId: externalProductId,
      mediaType: mediaType,
      storageKey: storageKey,
      url: url,
      thumbnailUrl: thumbnailUrl,
      sortOrder: sortOrder,
    );
  }

  Future<Result<void>> deleteExternalProductMedia({
    required String externalProductId,
    required String mediaId,
  }) {
    return _repository.deleteExternalProductMedia(
      externalProductId: externalProductId,
      mediaId: mediaId,
    );
  }
}