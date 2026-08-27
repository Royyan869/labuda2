import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';

/// User Management Controller
///
/// Handles user lookup, search, and role management operations.
/// Separated from AuthController for better separation of concerns.
///
/// R4.1B DI MIGRATION: Now uses authRepositoryProvider instead of direct instantiation
///
/// GUIDELINES compliant:
/// - File size < 150 lines ✅
/// - Single responsibility (user management) ✅
/// - Uses repository pattern ✅
/// - Uses canonical DI path (providers) ✅
class UserController extends Notifier<UserControllerState> {
  late final IAuthRepository _authRepository;

  @override
  UserControllerState build() {
    // R4.1B: Use authRepositoryProvider instead of AuthRepositoryImpl()
    // Previous implementation created AuthRepositoryImpl() directly which used ApiDI fallback
    // Now uses the canonical provider from auth_controller.dart
    _authRepository = ref.watch(authRepositoryProvider);

    return const UserControllerState.initial();
  }

  /// Deactivate user account
  Future<bool> deactivateAccount({
    required String userId,
    required String reason,
  }) async {
    state = const UserControllerState.loading();

    final result = await _authRepository.deactivateAccount(
      userId: userId,
      reason: reason,
    );

    if (result.isSuccess) {
      state = const UserControllerState.success();
      return true;
    } else {
      state = UserControllerState.error(result.error!);
      return false;
    }
  }

  /// Get user by ID
  Future<AuthUser?> getUserById(String userId) async {
    state = const UserControllerState.loading();

    final result = await _authRepository.getUserById(userId);

    if (result.isSuccess) {
      state = const UserControllerState.success();
      return result.data;
    } else {
      state = UserControllerState.error(result.error!);
      return null;
    }
  }

  /// Search users by name or username
  Future<List<AuthUser>> searchUsers({
    required String query,
    int limit = 20,
  }) async {
    state = const UserControllerState.loading();

    final result = await _authRepository.searchUsers(
      query: query,
      limit: limit,
    );

    if (result.isSuccess) {
      state = const UserControllerState.success();
      return result.data!;
    } else {
      state = UserControllerState.error(result.error!);
      return [];
    }
  }

  /// Update user role (for seller upgrade, admin operations)
  Future<bool> updateUserRole({
    required String userId,
    required UserRole newRole,
  }) async {
    state = const UserControllerState.loading();

    final result = await _authRepository.updateUserRole(
      userId: userId,
      newRole: newRole,
    );

    if (result.isSuccess) {
      state = const UserControllerState.success();
      return true;
    } else {
      state = UserControllerState.error(result.error!);
      return false;
    }
  }

  /// Clear error state
  void clearError() {
    if (state is UserControllerStateError) {
      state = const UserControllerState.initial();
    }
  }

  /// Reset state to initial
  void reset() {
    state = const UserControllerState.initial();
  }
}

/// User Controller State
sealed class UserControllerState {
  const UserControllerState();

  const factory UserControllerState.initial() = UserControllerStateInitial;
  const factory UserControllerState.loading() = UserControllerStateLoading;
  const factory UserControllerState.success() = UserControllerStateSuccess;
  const factory UserControllerState.error(String message) =
      UserControllerStateError;
}

class UserControllerStateInitial extends UserControllerState {
  const UserControllerStateInitial();
}

class UserControllerStateLoading extends UserControllerState {
  const UserControllerStateLoading();
}

class UserControllerStateSuccess extends UserControllerState {
  const UserControllerStateSuccess();
}

class UserControllerStateError extends UserControllerState {
  final String message;
  const UserControllerStateError(this.message);
}

/// Provider untuk UserController
final userControllerProvider =
    NotifierProvider<UserController, UserControllerState>(UserController.new);
