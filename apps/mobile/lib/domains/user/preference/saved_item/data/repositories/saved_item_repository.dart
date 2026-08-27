import 'package:dio/dio.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/domains/user/preference/saved_item/models/saved_item_model.dart';

class SavedItemRepository {
  final Dio _dio;

  SavedItemRepository({Dio? dio, ApiClient? apiClient})
    : _dio = dio ?? apiClient?.dio ?? ApiClient().dio;

  Future<List<SavedItemModel>> getSavedItems({String? type}) async {
    try {
      final response = await _dio.get(
        '/saved-items',
        queryParameters: {if (type != null) 'type': type},
      );

      final data = response.data['data'];
      final items = data['items'] as List<dynamic>? ?? [];
      final auctions = data['auctions'] as List<dynamic>? ?? [];

      final result = <SavedItemModel>[];

      for (var item in items) {
        result.add(SavedItemModel.fromJson(item as Map<String, dynamic>));
      }

      for (var auction in auctions) {
        result.add(SavedItemModel.fromJson(auction as Map<String, dynamic>));
      }

      return result;
    } catch (e) {
      rethrow;
    }
  }

  Future<SavedItemModel> addSavedItem({
    required String targetType,
    required String targetId,
  }) async {
    try {
      final response = await _dio.post(
        '/saved-items',
        data: {'target_type': targetType, 'target_id': targetId},
      );

      return SavedItemModel.fromJson(
        response.data['data'] as Map<String, dynamic>,
      );
    } catch (e) {
      rethrow;
    }
  }

  Future<void> removeSavedItem({
    required String targetType,
    required String targetId,
  }) async {
    try {
      await _dio.delete(
        '/saved-items/$targetId',
        queryParameters: {'type': targetType},
      );
    } catch (e) {
      rethrow;
    }
  }

  Future<void> clearSavedItems({String? type}) async {
    try {
      await _dio.delete(
        '/saved-items',
        queryParameters: {if (type != null) 'type': type},
      );
    } catch (e) {
      rethrow;
    }
  }

  Future<int> getSavedItemsCount({String? type}) async {
    try {
      final response = await _dio.get(
        '/saved-items/count',
        queryParameters: {if (type != null) 'type': type},
      );

      return response.data['data']['count'] as int;
    } catch (e) {
      rethrow;
    }
  }

  Future<bool> isSaved({
    required String targetType,
    required String targetId,
  }) async {
    try {
      final response = await _dio.get(
        '/saved-items/check',
        queryParameters: {'type': targetType, 'id': targetId},
      );

      return response.data['data']['is_saved'] as bool;
    } catch (e) {
      return false;
    }
  }
}
