/// User profile mutation result.
///
/// This is intentionally not a full authenticated account. It only carries
/// fields that were actually mutated by the profile update flow.
class UserProfilePatch {
  final String? photoUrl;
  final String? phoneNumber;
  final DateTime? phoneVerifiedAt;
  final String? username;
  final String? bio;
  final String? location;
  final DateTime? dateOfBirth;

  const UserProfilePatch({
    this.photoUrl,
    this.phoneNumber,
    this.phoneVerifiedAt,
    this.username,
    this.bio,
    this.location,
    this.dateOfBirth,
  });

  bool get isEmpty =>
      photoUrl == null &&
      phoneNumber == null &&
      phoneVerifiedAt == null &&
      username == null &&
      bio == null &&
      location == null &&
      dateOfBirth == null;
}
