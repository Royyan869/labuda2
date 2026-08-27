/// Finance Gateway Interface
///
/// **STRICT BOUNDARY ENFORCEMENT:**
/// Commerce domain CANNOT directly import or call finance repositories.
/// ALL finance operations MUST go through this interface.
///
/// **DOMAIN ISOLATION:**
/// - Finance is completely separate from commerce
/// - Commerce can ONLY call finance via this gateway
/// - No direct finance repository imports allowed in commerce
/// - Finance does NOT import commerce (unidirectional dependency)
///
/// **RULES:**
/// ❌ FORBIDDEN: `import '../../finance/repositories/finance_repository.dart'` in commerce
/// ❌ FORBIDDEN: Direct finance mutations from commerce
/// ✅ REQUIRED: Use FinanceGateway interface for all finance operations
///
/// **IMPLEMENTATION:**
/// The actual implementation lives in finance domain.
/// Commerce domain depends on this interface (abstraction) only.
/// This is the Bridge Pattern for strict domain separation.
abstract class FinanceGateway {
  /// Charge payment for commerce transaction
  ///
  /// Called by commerce domain when:
  /// - Order is confirmed
  /// - Auction payment is processed
  /// - Checkout is completed
  ///
  /// **STRICT RULE:** Commerce cannot directly mutate wallet balance
  /// Finance domain owns all wallet/ledger state
  Future<FinanceResult> charge({
    required String userId,
    required double amount,
    required String currency,
    required String description,
    required String orderId,
  });

  /// Release escrow for completed transaction
  ///
  /// Called by commerce domain when:
  /// - Order is delivered successfully
  /// - Auction winner claims item
  /// - Seller confirmation received
  Future<FinanceResult> releaseEscrow({
    required String transactionId,
    required String reason,
  });

  /// Refund payment to buyer
  ///
  /// Called by commerce domain when:
  /// - Order is cancelled
  /// - Item returned
  /// - Dispute resolved in buyer's favor
  Future<FinanceResult> refund({
    required String transactionId,
    required double amount,
    required String reason,
  });

  /// Check if user has sufficient balance
  ///
  /// Called by commerce domain for:
  /// - Pre-validation before order placement
  /// - Balance checks for bidding
  /// - Wallet availability for purchases
  Future<FinanceResult<bool>> checkBalance({
    required String userId,
    required double amount,
  });

  /// Get user's available balance
  ///
  /// Read-only operation for display purposes
  /// Does NOT mutate finance state
  Future<FinanceResult<double>> getBalance(String userId);

  /// Hold funds for pending transaction
  ///
  /// Called by commerce domain when:
  /// - Order is placed (awaiting confirmation)
  /// - Auction bid is placed (awaiting acceptance)
  Future<FinanceResult> holdFunds({
    required String userId,
    required double amount,
    required String orderId,
  });

  /// Verify payment status
  ///
  /// Called by commerce domain to confirm:
  /// - Payment was successful
  /// - Funds are secured
  /// - Transaction is valid
  Future<FinanceResult<bool>> verifyPayment(String transactionId);
}

/// Finance Result Type
///
/// Standardized result type for all finance operations
/// Enforces that finance operations return structured results
class FinanceResult<T> {
  final T? data;
  final String? error;
  final bool isSuccess;
  final String? transactionId;

  const FinanceResult({
    this.data,
    this.error,
    required this.isSuccess,
    this.transactionId,
  });

  /// Create success result
  factory FinanceResult.success({T? data, String? transactionId}) =>
      FinanceResult(data: data, isSuccess: true, transactionId: transactionId);

  /// Create error result
  factory FinanceResult.error(String error) =>
      FinanceResult(isSuccess: false, error: error);

  /// Check if result is successful
  bool get isSuccessful => isSuccess;

  /// Get data or throw if error
  T get dataOrThrow {
    if (!isSuccess) {
      throw FinanceException(error ?? 'Finance operation failed');
    }
    return data as T;
  }
}

/// Finance Exception
///
/// Thrown when finance operation fails
/// Used by data getter to enforce error handling
class FinanceException implements Exception {
  final String message;
  FinanceException(this.message);

  @override
  String toString() => 'FinanceException: $message';
}
