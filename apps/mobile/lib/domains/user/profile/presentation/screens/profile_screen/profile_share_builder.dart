import 'package:labuda/domains/social/share/domain/entities/share_target.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/helpers/user_identity_formatter.dart';

bool canShareProfileIdentity({
  required String? username,
  required ContentLifecycle lifecycle,
}) {
  return lifecycle.isActive &&
      UserIdentityFormatter.formatHandle(username) != null;
}

ShareTarget? buildProfileShareTarget({
  required String userId,
  required String? username,
  required String? bio,
  required String? avatarUrl,
  required ContentLifecycle lifecycle,
}) {
  if (!canShareProfileIdentity(username: username, lifecycle: lifecycle)) {
    return null;
  }

  final handle = UserIdentityFormatter.formatHandle(username)!;
  final description = bio != null && bio.trim().isNotEmpty
      ? bio.trim()
      : 'Lihat profil $handle di LABUDA';
  final resolvedAvatarUrl = avatarUrl != null && avatarUrl.trim().isNotEmpty
      ? avatarUrl
      : null;

  return ShareTarget(
    id: userId,
    type: ExternalShareType.profile,
    title: handle,
    description: description,
    imageUrl: resolvedAvatarUrl,
  );
}
