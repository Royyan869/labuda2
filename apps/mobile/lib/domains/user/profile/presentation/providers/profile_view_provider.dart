import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/user/profile/data/mappers/user_api_mapper.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show userApiDatasourceProvider;
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/auth_user.dart';

/// Combined user/profile snapshot fetched from a single backend request.
///
/// This is the single owner for profile-page hydration so the profile screen
/// can render both identity and cover/profile metadata from one GET.
class ProfileViewData {
  final AuthUser user;
  final ProfileEntity profile;

  const ProfileViewData({required this.user, required this.profile});
}

final profileViewDataProvider =
    FutureProvider.family<ProfileViewData?, String>((ref, userId) async {
      final datasource = ref.read(userApiDatasourceProvider);
      final result = await datasource.getUserById(userId);

      return result.fold((error) {
        if (error.contains('not found') || error.contains('404')) {
          return null;
        }
        throw Exception(error);
      }, (response) {
        return ProfileViewData(
          user: UserApiMapper.toAuthUser(response),
          profile: UserApiMapper.toProfileEntity(response),
        );
      });
    });
