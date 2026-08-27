import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';

/// Profile Application State
///
/// Immutable state container for profile feature.
/// Phase 5: Application Layer
///
/// Rules:
/// - Immutable (all fields final)
/// - No BuildContext stored
/// - No UI state (scroll, focus, etc.)
class ProfileState {
  final AsyncValue<ProfileEntity?> profile;
  final bool isUpdating;
  final String? errorMessage;

  const ProfileState({
    this.profile = const AsyncValue.data(null),
    this.isUpdating = false,
    this.errorMessage,
  });

  /// Initial state
  static const initial = ProfileState();

  /// Loading state helper
  static ProfileState loading() =>
      const ProfileState(profile: AsyncValue.loading(), isUpdating: false);

  /// Error state helper
  static ProfileState error(Object error, StackTrace stackTrace) =>
      ProfileState(
        profile: AsyncValue.error(error, stackTrace),
        isUpdating: false,
      );

  /// Data state helper
  static ProfileState data(ProfileEntity data) =>
      ProfileState(profile: AsyncValue.data(data), isUpdating: false);

  ProfileState copyWith({
    AsyncValue<ProfileEntity?>? profile,
    bool? isUpdating,
    String? errorMessage,
  }) {
    return ProfileState(
      profile: profile ?? this.profile,
      isUpdating: isUpdating ?? this.isUpdating,
      errorMessage: errorMessage,
    );
  }

  /// Convenience getters
  bool get isLoading => profile.isLoading || isUpdating;
  bool get hasError => profile.hasError || errorMessage != null;
  bool get hasData => profile.hasValue && profile.value != null;

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is ProfileState &&
          profile == other.profile &&
          isUpdating == other.isUpdating &&
          errorMessage == other.errorMessage;

  @override
  int get hashCode => Object.hash(profile, isUpdating, errorMessage);
}
