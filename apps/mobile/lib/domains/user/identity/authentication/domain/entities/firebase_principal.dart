import 'package:firebase_auth/firebase_auth.dart';

/// Firebase principal.
///
/// This is the identity-only shape produced by Firebase Auth.
/// It intentionally carries no backend account authority.
class FirebasePrincipal {
  final String uid;
  final String? email;
  final bool emailVerified;
  final String? phoneNumber;
  final List<String> providerIds;
  final DateTime? creationTime;
  final DateTime? lastSignInTime;

  const FirebasePrincipal({
    required this.uid,
    required this.emailVerified,
    this.email,
    this.phoneNumber,
    this.providerIds = const <String>[],
    this.creationTime,
    this.lastSignInTime,
  });

  String get id => uid;

  factory FirebasePrincipal.fromFirebaseUser(User user) {
    return FirebasePrincipal(
      uid: user.uid,
      email: user.email,
      emailVerified: user.emailVerified,
      phoneNumber: user.phoneNumber,
      providerIds: user.providerData.map((info) => info.providerId).toList(),
      creationTime: user.metadata.creationTime,
      lastSignInTime: user.metadata.lastSignInTime,
    );
  }
}
