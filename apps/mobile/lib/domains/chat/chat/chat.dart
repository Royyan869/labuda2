// Chat Refactor Module
//
// Clean Architecture implementation for Chat feature.
// Uses API-based backend with WebSocket for real-time updates.

// Domain Layer
export 'domain/entities/chat_entities.dart';
export 'domain/repositories/chat_repository.dart';
export 'domain/usecases/chat_usecase_providers.dart';

// Data Layer - NOT EXPORTED (Implementation Detail)
// ❌ NOT exported: data/dto/, data/mappers/, data/remote/, data/repositories/
// These are internal to the module and accessed via repository interface only.

// Presentation Layer - Providers (State, Notifier)
export 'presentation/providers/chat_state.dart';
export 'presentation/providers/chat_notifier.dart';
export 'presentation/providers/chat_providers.dart';

// Presentation Layer - Screens
export 'presentation/screens/chat_list_screen.dart';
export 'presentation/screens/chat_detail_screen.dart';
export 'presentation/screens/new_chat_screen.dart';

// Presentation Layer - Widgets
export 'presentation/widgets/chat_card.dart';
export 'presentation/widgets/message_bubble.dart';
export 'presentation/widgets/chat_input_area.dart';
export 'presentation/widgets/typing_indicator.dart';
export 'presentation/widgets/new_chat_user_list_widget.dart';
