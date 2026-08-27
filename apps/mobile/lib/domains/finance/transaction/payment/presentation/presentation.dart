/// Payment Presentation Layer
///
/// Dumb UI widgets and screens.
/// Only render state and delegate actions to Notifiers.
///
/// IMPORTANT: Coin purchase functionality removed - Coins are loyalty points
/// that can only be earned, NOT purchased.
///
/// ⚠️ COINS OWNERSHIP:
/// - Coin providers and widgets have been REMOVED from this file
/// - Coins are now owned by domains/finance/wallet/coins module
/// - Use import 'package:labuda/domains/finance/wallet/coins/coins.dart';
library;

// Widgets
export 'widgets/payment_method_tile.dart';
export 'widgets/payment_method_selector_sheet.dart';
export 'widgets/payment_method_picker_sheet.dart';

// Payment initiation (PAYMENT INITIATION FLOW)
export 'providers/payment_initiation_notifier.dart'
    show
        InitiatePaymentRequest,
        PaymentInitiationNotifier,
        paymentInitiationProvider;
export 'providers/payment_initiation_state.dart';
export 'providers/payment_providers.dart' show paymentRepositoryProvider;

// PASS_18V: canonical payment method + backend-calculated fee (checkout flow)
export '../domain/entities/payment.dart' show PaymentMethodOption;

// Payment result (PAYMENT RESULT / STATUS RECONCILIATION FLOW)
export 'providers/payment_result_notifier.dart'
    show PaymentResultNotifier, paymentResultProvider;
export 'providers/payment_result_state.dart';
