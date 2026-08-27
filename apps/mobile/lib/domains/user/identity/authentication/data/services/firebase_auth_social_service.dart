import 'package:firebase_auth/firebase_auth.dart';
import 'package:google_sign_in/google_sign_in.dart';
import 'package:labuda/core/core.dart';
import '../../domain/entities/firebase_principal.dart';

/// Firebase Auth Social Service - Handles social login operations
class FirebaseAuthSocialService {
  final FirebaseAuth _firebaseAuth;
  final GoogleSignIn? _googleSignIn;

  FirebaseAuthSocialService({
    FirebaseAuth? firebaseAuth,
    GoogleSignIn? googleSignIn,
  }) : _firebaseAuth = firebaseAuth ?? FirebaseAuth.instance,
       _googleSignIn = googleSignIn;

  Future<Result<FirebasePrincipal>> signInWithGoogle() async {
    try {
      // Initialize GoogleSignIn if not already done
      final googleSignIn = _googleSignIn ?? GoogleSignIn(scopes: ['email']);

      // Trigger the Google Sign-In flow
      final GoogleSignInAccount? googleUser = await googleSignIn.signIn();

      // If user cancels the sign-in
      if (googleUser == null) {
        return Result.error('Sign in dibatalkan oleh pengguna');
      }

      // Obtain the auth details from the request
      final GoogleSignInAuthentication googleAuth =
          await googleUser.authentication;

      // Create a new credential
      final credential = GoogleAuthProvider.credential(
        accessToken: googleAuth.accessToken,
        idToken: googleAuth.idToken,
      );

      // Sign in to Firebase with the Google credential
      final UserCredential userCredential = await _firebaseAuth
          .signInWithCredential(credential);

      if (userCredential.user != null) {
        return Result.success(
          FirebasePrincipal.fromFirebaseUser(userCredential.user!),
        );
      } else {
        return Result.error('Google Sign-In failed: No user data received');
      }
    } on FirebaseAuthException catch (e) {
      return Result.error(_getFirebaseErrorMessage(e));
    } catch (e) {
      return Result.error('Google Sign-In failed: ${e.toString()}');
    }
  }

  Future<Result<FirebasePrincipal>> signInWithApple() async {
    // Apple Sign In implementation would go here
    // For now, return not implemented
    return Result.error('Apple Sign In not implemented yet');
  }

  Future<Result<void>> signOutGoogle() async {
    try {
      if (_googleSignIn != null) {
        await _googleSignIn.signOut();
      }
      return Result.success(null);
    } catch (e) {
      return Result.error('Google sign out failed: ${e.toString()}');
    }
  }

  String _getFirebaseErrorMessage(FirebaseAuthException e) {
    switch (e.code) {
      case 'account-exists-with-different-credential':
        return 'Account already registered with a different login method';
      case 'invalid-credential':
        return 'Invalid credentials';
      case 'operation-not-allowed':
        return 'This login method is not allowed';
      case 'user-disabled':
        return 'This account has been disabled';
      default:
        return 'Error: ${e.message ?? e.code}';
    }
  }
}
