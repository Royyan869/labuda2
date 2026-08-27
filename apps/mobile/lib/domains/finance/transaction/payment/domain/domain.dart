/// Payment Domain Layer
///
/// Pure Dart entities and repository interfaces.
/// No dependencies on Flutter, Firebase, or external services.
///
/// IMPORTANT: Coins are LOYALTY POINTS, NOT purchasable packages.
///
/// ⚠️ COINS OWNERSHIP:
/// - Coin entities and repositories have been REMOVED from this file
/// - Coins are now owned by domains/finance/wallet/coins module
/// - Use import 'package:labuda/domains/finance/wallet/coins/coins.dart';
library;

// Entities
export 'entities/payment.dart';
export 'entities/payment_intent.dart';
export 'entities/payment_result.dart';
// PHASE 1F: payment_status.dart removed - using unified PaymentStatus from core
// export 'entities/payment_status.dart'; // REMOVED - use package:labuda/core/common/types/payment_types.dart
export 'entities/payment_method.dart';
// export 'entities/fee_config.dart'; // P11 Phase 2: Removed - fee calculations now done by backend
export 'entities/payment_request.dart';
export 'entities/seller_subscription_payment_request.dart';

// Repository Interfaces
export 'repositories/payment_repository.dart';

// Failures
export 'failures/payment_failure.dart';
