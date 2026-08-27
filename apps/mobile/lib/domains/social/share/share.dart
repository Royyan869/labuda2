// Share Module - Clean Architecture Implementation
//
// This module follows the Flutter Clean-ish Architecture pattern:
// - Domain: Entities, Repository Interfaces (pure Dart)
// - Data: DTOs, Mappers, Datasources, Repository Implementations (Firebase, etc.)
// - Application: Notifiers, State (Riverpod)
// - Presentation: Providers, Widgets (Flutter UI)
//
// Usage:
// ```dart
// import 'package:labuda/domains/social/share/share.dart';
//
// // In UI
// final shareState = ref.watch(shareNotifierProvider);
// final notifier = ref.read(shareNotifierProvider.notifier);
//
// // Share via external
// await notifier.shareViaExternal(
//   target: shareTarget,
//   destination: ShareDestinationType.whatsapp,
// );
//
// // Share as post
// final postId = await notifier.shareAsPost(
//   target: shareTarget,
//   authorId: userId,
//   caption: 'Check this out!',
// );
// ```

library;

// Domain Layer - Entities + Repository Interface
export 'domain/domain.dart';

// Data Layer - Internal use only, NOT exported
// (datasources and repository impl are implementation details)

// Presentation Layer - Notifiers + State + Widgets
export 'presentation/providers/share_state.dart';
export 'presentation/providers/share_notifier.dart';
export 'presentation/presentation.dart';
