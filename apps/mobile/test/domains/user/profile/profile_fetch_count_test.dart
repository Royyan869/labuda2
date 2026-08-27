import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/models/api/user_api_models.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show userApiDatasourceProvider;
import 'package:labuda/domains/user/profile/presentation/providers/profile_view_provider.dart';
import 'package:labuda/shared/services/logger_service.dart';

class _CountingUserApiDatasource extends UserApiDatasource {
  _CountingUserApiDatasource(this._response)
    : super(ApiClient.testing(logger: LoggerService.instance));

  final Map<String, dynamic> _response;
  int getUserByIdCalls = 0;

  @override
  Future<Result<UserApiResponse>> getUserById(String userId) async {
    getUserByIdCalls += 1;
    return Result.success(UserApiResponse.fromJson(_response));
  }
}

Map<String, dynamic> _profileViewResponse(String userId) => <String, dynamic>{
  'id': userId,
  'email': 'seller@example.com',
  'username': 'seller',
  'account_status': 'active',
  'roles': <String>['seller'],
  'store_name': 'Seller Farm',
  'store_image_url': 'https://example.com/store.jpg',
  'created_at': '2026-07-27T00:00:00.000Z',
  'updated_at': '2026-07-27T00:00:00.000Z',
  'profile': <String, dynamic>{
    'id': 'profile-$userId',
    'username': 'seller',
    'avatar_url': 'https://example.com/avatar.jpg',
    'cover_photo_url': 'https://example.com/cover.jpg',
    'bio': 'Bio',
    'location': 'Bandung',
    'followers_count': 2,
    'following_count': 1,
    'last_active_at': '2026-07-27T00:00:00.000Z',
  },
};

void main() {
  testWidgets(
    'profile view fetches once on open, once on reconciliation, and never polls periodically',
    (tester) async {
      const userId = 'user-fetch-count';
      final datasource = _CountingUserApiDatasource(
        _profileViewResponse(userId),
      );
      final container = ProviderContainer(
        overrides: [userApiDatasourceProvider.overrideWithValue(datasource)],
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            home: Consumer(
              builder: (context, ref, _) {
                ref.watch(profileViewDataProvider(userId));
                return const SizedBox.shrink();
              },
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(datasource.getUserByIdCalls, 1);

      container.invalidate(profileViewDataProvider(userId));
      await tester.pumpAndSettle();
      expect(datasource.getUserByIdCalls, 2);

      await tester.pump(const Duration(seconds: 46));
      await tester.pumpAndSettle();
      expect(datasource.getUserByIdCalls, 2);
    },
  );
}
