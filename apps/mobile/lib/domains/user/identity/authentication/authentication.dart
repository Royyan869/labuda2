// Authentication feature module
//
// PUBLIC API - Sesuai GUIDELINES barrel file rules
library;

// Domain layer (entities & repository interfaces)
export 'domain/entities/auth_user.dart';
export 'domain/repositories/i_auth_repository.dart';

// Domain types currently in data/ (TODO: move to domain/)
export 'data/username_validation_service.dart'
    show UsernameCheckStatus, UsernameCheckResult, UsernameValidationService;

// Presentation layer (providers, screens, widgets)
export 'presentation/providers/auth_controller.dart';
export 'presentation/providers/auth_state.dart';
export 'presentation/providers/user_controller.dart';
export 'presentation/screens/sign_in_screen.dart';
export 'presentation/screens/sign_up_screen.dart';
export 'presentation/screens/forgot_password_screen.dart';
export 'presentation/screens/complete_profile_screen.dart';
export 'presentation/screens/account_restricted_screen.dart';

// NOTE: Data layer is PRIVATE
// - Repository implementations
// - Data models
// - Services (Firebase isolated in data/services/)
// Access via DI or use cases only
