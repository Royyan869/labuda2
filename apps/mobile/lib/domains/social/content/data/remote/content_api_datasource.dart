// Content API Datasource
// HTTP operations for Content domain - isolasi ApiClient

import 'package:dio/dio.dart';
import 'package:labuda/core/api/api.dart';
import 'package:labuda/domains/social/content/data/dto/content_dto.dart';
import 'package:uuid/uuid.dart';

/// Content API Datasource
///
/// Menangani semua HTTP operations ke backend API.
/// Isolasi dari domain logic - hanya handle request/response.
class ContentApiDatasource {
  final ApiClient _apiClient;

  ContentApiDatasource(this._apiClient);

  // ==========================================================================
  // Content CRUD Operations
  // ==========================================================================

  /// Create new content
  Future<ContentDto> createContent(CreateContentDto request) async {
    final idempotencyKey = const Uuid().v4();
    final response = await _apiClient.post(
      '/contents',
      data: request.toJson(),
      options: Options(headers: {'Idempotency-Key': idempotencyKey}),
    );
    return ContentDto.fromJson(response.data['data']);
  }

  /// Get single content by ID
  Future<ContentDto> getContentById(String contentId) async {
    final response = await _apiClient.get('/contents/$contentId');
    return ContentDto.fromJson(response.data['data']);
  }

  /// Get list of contents with filters
  Future<List<ContentDto>> getContents({
    String? status,
    String? visibility,
    String? city,
    String? province,
    int? limit,
    int? offset,
  }) async {
    final params = <String, dynamic>{};
    if (status != null) params['status'] = status;
    if (visibility != null) params['visibility'] = visibility;
    if (city != null) params['city'] = city;
    if (province != null) params['province'] = province;
    if (limit != null) params['limit'] = limit;
    if (offset != null) params['offset'] = offset;

    final response = await _apiClient.get(
      '/content/contents',
      queryParameters: params,
    );

    final List<dynamic> data = response.data['data'] ?? [];
    return data.map((json) => ContentDto.fromJson(json)).toList();
  }

  /// Get user's contents — cursor-paginated (C3A canonical route).
  ///
  /// Calls GET /users/:id/contents with cursor-based pagination.
  /// [cursor] is opaque; pass the [UserContentPageDto.nextCursor] from the
  /// prior response verbatim. Omit on the first fetch.
  Future<UserContentPageDto> getUserContents(
    String userId, {
    int limit = 20,
    String? cursor,
  }) async {
    final params = <String, dynamic>{'limit': limit};
    if (cursor != null) params['cursor'] = cursor;

    final response = await _apiClient.get(
      '/users/$userId/contents',
      queryParameters: params,
    );

    return UserContentPageDto.fromJson(
      response.data['data'] as Map<String, dynamic>,
    );
  }

  /// Update content
  Future<ContentDto> updateContent(
    String contentId,
    UpdateContentDto request,
  ) async {
    final response = await _apiClient.put(
      '/contents/$contentId',
      data: request.toJson(),
    );
    return ContentDto.fromJson(response.data['data']);
  }

  /// Delete content
  Future<void> deleteContent(String contentId) async {
    await _apiClient.delete('/contents/$contentId');
  }

  // ==========================================================================
  // Feed & Discovery
  // ==========================================================================

  /// Get trending contents
  Future<List<ContentDto>> getTrendingContents({
    int? limit,
    int? offset,
  }) async {
    final params = <String, dynamic>{};
    if (limit != null) params['limit'] = limit;
    if (offset != null) params['offset'] = offset;

    final response = await _apiClient.get(
      '/content/trending',
      queryParameters: params,
    );

    final List<dynamic> data = response.data['data'] ?? [];
    return data.map((json) => ContentDto.fromJson(json)).toList();
  }

  // ==========================================================================
  // Search Operations
  // ==========================================================================

  /// Search contents
  Future<ContentSearchResultDto> searchContents(
    String query, {
    String? city,
    String? province,
    int? limit,
    int? offset,
  }) async {
    final params = <String, dynamic>{'q': query};
    if (city != null) params['city'] = city;
    if (province != null) params['province'] = province;
    if (limit != null) params['limit'] = limit;
    if (offset != null) params['offset'] = offset;

    final response = await _apiClient.get(
      '/content/search',
      queryParameters: params,
    );
    return ContentSearchResultDto.fromJson(response.data['data']);
  }

  // ==========================================================================
  // Engagement Operations
  // ==========================================================================

  // Note: View tracking is handled by backend automatically via GET /contents/:id
  // No explicit incrementViewCount needed
}
