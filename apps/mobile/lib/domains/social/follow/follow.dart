// Barrel file for follow module

// Domain layer exports
export 'domain/entities/follow_entity.dart';
export 'domain/repositories/i_follow_repository.dart';
export 'domain/use_cases/follow_user_use_case.dart';
export 'domain/use_cases/unfollow_user_use_case.dart';
export 'domain/use_cases/get_follow_stats_use_case.dart';
export 'domain/use_cases/get_followers_use_case.dart';
export 'domain/use_cases/get_following_use_case.dart';
export 'domain/use_cases/search_users_use_case.dart';

// Domain layer - Use Case Providers (Riverpod)
export 'domain/use_cases/providers/use_case_providers.dart';

// Data layer - Providers (Riverpod)
export 'data/follow_providers.dart';

// Presentation layer exports
export 'presentation/providers/follow_actions_provider.dart';
export 'presentation/providers/follow_status_provider.dart';
export 'presentation/providers/follow_stats_provider.dart';
export 'presentation/providers/follow_lists_provider.dart';
export 'presentation/providers/follow_search_provider.dart';
export 'presentation/providers/follow_stream_provider.dart';

export 'presentation/widgets/follow_stats_widget.dart';
export 'presentation/widgets/user_card.dart';
export 'presentation/widgets/user_search_bar.dart';

export 'presentation/screens/follow_list_screen.dart';
