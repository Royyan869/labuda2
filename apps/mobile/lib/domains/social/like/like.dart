/// Like Feature (Backend-First Architecture)
///
/// This module follows the backend-first pattern:
/// - Domain: Entities and Repository interfaces
/// - Data: DTOs, Mappers, Datasources, Repository Implementation
/// - Presentation: Notifiers and State
///
/// Usage:
/// ```dart
/// import 'package:labuda/domains/social/like/like.dart';
///
/// // In widget
/// ref.watch(likeStatsProvider(params))
/// ref.read(likeNotifierProvider.notifier).toggleLike(...)
/// ```

library;

// Domain
export 'domain/entities/like.dart';
export 'domain/repositories/like_repository.dart';

// Presentation (Notifiers + State)
export 'presentation/providers/like_state.dart';
export 'presentation/providers/like_notifier.dart';

