/// Like Feature (Backend-First Architecture)
///
/// This module follows the backend-first pattern:
/// - Domain: Entities and Repository interfaces
/// - Data: DTOs, Mappers, Datasources, Repository Implementation
/// - Presentation: Notifiers, State, and Widgets
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

// Presentation (Widgets)
export 'presentation/widgets/like_count_widget.dart';
export 'presentation/widgets/like_count_style_utils.dart';
export 'presentation/widgets/like_count_states.dart';
