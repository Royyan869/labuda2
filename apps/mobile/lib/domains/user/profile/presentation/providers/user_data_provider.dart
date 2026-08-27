import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';

/// Provider untuk fetch user data berdasarkan userId
///
/// Menggunakan FutureProvider untuk auto-cache dan loading states
/// Now uses Riverpod dependency graph (no direct GetIt access)
final userDataProvider = FutureProvider.family<AuthUser?, String>((
  ref,
  userId,
) async {
  // Use auth module's provider directly
  final authRepository = ref.read(authRepositoryProvider);

  final result = await authRepository.getUserById(userId);

  if (result.isSuccess) {
    return result.data;
  } else {
    // Throw error agar FutureProvider bisa handle sebagai error state
    throw Exception(result.error);
  }
});

/// Provider untuk check apakah sedang loading user data
final userDataLoadingProvider = Provider.family<bool, String>((ref, userId) {
  final userDataAsync = ref.watch(userDataProvider(userId));
  return userDataAsync.isLoading;
});

/// Provider untuk get user data dengan error handling
final userDataResultProvider = Provider.family<AsyncValue<AuthUser?>, String>((
  ref,
  userId,
) {
  return ref.watch(userDataProvider(userId));
});
