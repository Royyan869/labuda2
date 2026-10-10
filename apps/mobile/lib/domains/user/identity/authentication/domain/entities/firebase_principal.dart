import 'package:firebase_auth/firebase_auth.dart';

/// Firebase principal.
///
/// This is the identity-only shape produced by Firebase Auth.
/// It intentionally carries no backend account authority.
class FirebasePrincipal {
  final String uid;
  final String? email;
  final bool emailVerified;
  final List<String> providerIds;

  const FirebasePrincipal({
    required this.uid,
    required this.emailVerified,
    this.email,
    this.providerIds = const <String>[],
  });

  factory FirebasePrincipal.fromFirebaseUser(User user) {
    return FirebasePrincipal(
      uid: user.uid,
      email: user.email,
      emailVerified: user.emailVerified,
      providerIds: user.providerData.map((info) => info.providerId).toList(),
    );
  }
}
