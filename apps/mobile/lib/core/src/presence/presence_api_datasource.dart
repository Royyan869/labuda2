import 'package:labuda/core/api/base_api_repository.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/presence/presence.dart';

/// Canonical datasource for initial Presence snapshot.
/// Uses GET /api/v1/users/presence?user_ids=id1,id2
class PresenceApiDatasource extends BaseApiRepository {
  PresenceApiDatasource(super.apiClient, {super.logger});

  /// Batch fetch presences for target userIds.
  /// Returns list of Presence with canonical fields.
  Future<Result<List<Presence>>> getPresences(List<String> userIds) async {
    if (userIds.isEmpty) {
      return Result.success([]);
    }
    return executeRequest(
      () => apiClient.get(
        '/users/presence',
        queryParameters: {'user_ids': userIds.join(',')},
      ),
      parser: (data) {
        final map = data as Map<String, dynamic>;
        final list = map['presences'] as List;
        return list
            .map((e) => Presence.fromJson(e as Map<String, dynamic>))
            .toList();
      },
    );
  }

  /// Single fetch convenience.
  Future<Result<Presence?>> getPresence(String userId) async {
    final result = await getPresences([userId]);
    return result.map((list) => list.isEmpty ? null : list.first);
  }
}
