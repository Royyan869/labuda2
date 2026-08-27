/// Authentication Data Providers - Riverpod providers for auth data layer
///
/// R4.3 PLACEMENT CONSISTENCY: Data layer providers for auth feature.
/// Replaces inline datasource construction in auth_controller.dart.
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/data/datasources/auth_api_datasource.dart';
import 'package:labuda/domains/user/identity/authentication/data/repositories/auth_repository_impl.dart';

// =============================================================================
// DATASOURCE PROVIDERS
// =============================================================================

/// Auth API Datasource Provider
///
/// Provides the API datasource for authentication operations.
/// R4.3: Moved from inline construction in auth_controller.dart to data layer.
final authApiDatasourceProvider = Provider<AuthApiDatasource>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  final logger = ref.watch(loggerServiceProvider);
  return AuthApiDatasource(apiClient, logger: logger);
});

// =============================================================================
// REPOSITORY PROVIDERS
// =============================================================================

/// Auth Repository Provider
///
/// Provides the implementation of IAuthRepository.
/// This replaces the inline construction in auth_controller.dart.
///
/// R4.3: Construction moved from auth_repositoryProvider (presentation layer)
/// to data layer provider file.
final authRepositoryProvider = Provider<IAuthRepository>((ref) {
  final apiDatasource = ref.watch(authApiDatasourceProvider);
  final localStorage = ref.watch(localStorageServiceProvider);
  return AuthRepositoryImpl(
    firebaseAuth: null, // Will use default instance
    googleSignIn: null, // Will create default instance
    localStorage: localStorage,
    apiDatasource: apiDatasource,
  );
});
