library;

/// Promotion Contract DTO — canonical authority promotion_contracts
/// Maps GET /api/v1/promotions/contracts and POST /api/v1/promotions/contracts
/// Uses budget, CPM, duration, geography, queue — NOT package/ownership.

class PromotionContractDto {
  final String id;
  final String sellerId;
  final String kind; // internal|external
  // Canonical lifecycle vocabulary (promotion_contract_status_enum):
  // prepared | active | paused | finalizing | finalized.
  final String status;
  final int budgetRupiah;
  final int cpmRupiah;
  final String plannedStart;
  final String plannedFinish;
  final String allocationAccountId;
  final String? pausedAt;
  final String? finalizedAt;
  final String createdAt;
  final String updatedAt;
  final List<String>
  cityIds; // promotion_contract_geographies city_id set, empty = nationwide

  PromotionContractDto({
    required this.id,
    required this.sellerId,
    required this.kind,
    required this.status,
    required this.budgetRupiah,
    required this.cpmRupiah,
    required this.plannedStart,
    required this.plannedFinish,
    required this.allocationAccountId,
    this.pausedAt,
    this.finalizedAt,
    required this.createdAt,
    required this.updatedAt,
    required this.cityIds,
  });

  factory PromotionContractDto.fromJson(Map<String, dynamic> json) {
    return PromotionContractDto(
      id: json['id'] as String,
      sellerId: json['seller_id'] as String,
      kind: json['kind'] as String,
      status: json['status'] as String,
      budgetRupiah: json['budget_rupiah'] as int,
      cpmRupiah: json['cpm_rupiah'] as int,
      plannedStart: json['planned_start'] as String,
      plannedFinish: json['planned_finish'] as String,
      allocationAccountId: json['allocation_account_id'] as String,
      pausedAt: json['paused_at'] as String?,
      finalizedAt: json['finalized_at'] as String?,
      createdAt: json['created_at'] as String,
      updatedAt: json['updated_at'] as String,
      cityIds: (json['city_ids'] as List<dynamic>? ?? [])
          .map((e) => e as String)
          .toList(),
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'seller_id': sellerId,
    'kind': kind,
    'status': status,
    'budget_rupiah': budgetRupiah,
    'cpm_rupiah': cpmRupiah,
    'planned_start': plannedStart,
    'planned_finish': plannedFinish,
    'allocation_account_id': allocationAccountId,
    'paused_at': pausedAt,
    'finalized_at': finalizedAt,
    'created_at': createdAt,
    'updated_at': updatedAt,
    'city_ids': cityIds,
  };
}

class PromotionContractListDto {
  final List<PromotionContractDto> contracts;
  final int count;

  PromotionContractListDto({required this.contracts, required this.count});

  factory PromotionContractListDto.fromJson(Map<String, dynamic> json) {
    final list = (json['contracts'] as List<dynamic>? ?? [])
        .map((e) => PromotionContractDto.fromJson(e as Map<String, dynamic>))
        .toList();
    return PromotionContractListDto(
      contracts: list,
      count: json['count'] as int? ?? list.length,
    );
  }
}

class CreatePromotionContractRequestDto {
  final String kind;
  final int budgetRupiah;
  final int durationDays;
  final List<String> cityIds;

  CreatePromotionContractRequestDto({
    required this.kind,
    required this.budgetRupiah,
    required this.durationDays,
    required this.cityIds,
  });

  Map<String, dynamic> toJson() => {
    'kind': kind,
    'budget_rupiah': budgetRupiah,
    'duration_days': durationDays,
    'city_ids': cityIds,
  };
}

/// Canonical reusable promotion funding balance.
///
/// WIRE AUTHORITY: GET /api/v1/promote-balance → { data: { balance } }.
///
/// This is the seller-owned PROMOTE_BALANCE projection (read-only; the ledger
/// is the money authority). It is NOT a wallet: there is no deposit, no
/// withdrawal and no arbitrary top-up — the only way this balance grows is the
/// canonical exact-shortage FundingIntent payment or a promotion finalization
/// releasing unused PROMOTION_ALLOCATION back here.
class PromoteBalanceDto {
  /// Whole Rupiah reusable funding available to fund a promotion.
  final int balance;

  const PromoteBalanceDto({required this.balance});

  factory PromoteBalanceDto.fromJson(Map<String, dynamic> json) {
    return PromoteBalanceDto(balance: (json['balance'] as num?)?.toInt() ?? 0);
  }

  @override
  String toString() => 'PromoteBalanceDto(balance: $balance)';
}

/// Canonical exact-shortage funding obligation for a proposed promotion.
///
/// WIRE AUTHORITY: POST /api/v1/promotions/contracts/payment-intent →
/// { data: { intent: { payment_required, shortage, intent_id, billing_id,
/// required_cost, available_funding } } }.
///
/// AUTHORITY: the backend is the sole authority for whether a payment is
/// required and for the exact amount. The client renders these numbers and
/// never recomputes a shortage, a balance, or a fee.
///
/// This is a snapshot funding calculation and payment obligation — NOT a
/// reservation, NOT bound to a specific promotion, and NOT a wallet top-up.
/// When [paymentRequired] is false the backend created nothing (no intent, no
/// billing): Create draws straight from reusable PROMOTE_BALANCE.
class PromotionFundingIntentDto {
  /// True when the seller must pay exactly [shortage] before allocation.
  final bool paymentRequired;

  /// Exact amount the seller must pay. Zero when [paymentRequired] is false.
  final int shortage;

  /// Canonical funding intent id. Present only when [paymentRequired] is true.
  final String? intentId;

  /// Billing obligation id behind [intentId]. Present only when required.
  final String? billingId;

  /// Total promotion cost (informational snapshot).
  final int requiredCost;

  /// Seller's reusable PROMOTE_BALANCE at intent time (informational).
  final int availableFunding;

  const PromotionFundingIntentDto({
    required this.paymentRequired,
    required this.shortage,
    required this.requiredCost,
    required this.availableFunding,
    this.intentId,
    this.billingId,
  });

  factory PromotionFundingIntentDto.fromJson(Map<String, dynamic> json) {
    final intentId = json['intent_id'] as String?;
    final billingId = json['billing_id'] as String?;
    return PromotionFundingIntentDto(
      paymentRequired: json['payment_required'] as bool? ?? false,
      shortage: (json['shortage'] as num?)?.toInt() ?? 0,
      requiredCost: (json['required_cost'] as num?)?.toInt() ?? 0,
      availableFunding: (json['available_funding'] as num?)?.toInt() ?? 0,
      intentId: (intentId == null || intentId.isEmpty) ? null : intentId,
      billingId: (billingId == null || billingId.isEmpty) ? null : billingId,
    );
  }

  @override
  String toString() =>
      'PromotionFundingIntentDto(required: $requiredCost, '
      'available: $availableFunding, shortage: $shortage, '
      'paymentRequired: $paymentRequired)';
}

/// One selectable payment method for the current promotion funding obligation.
///
/// WIRE AUTHORITY: GET /api/v1/promotions/contracts/payment-intent/:id/
/// payment-methods → { data: { shortage_amount, currency, methods } }.
///
/// [serviceFeeAmount] (F) and [grossAmount] are computed server-side from the
/// exact shortage obligation — the client must never recompute, adjust, or
/// submit either value.
class PromotionFundingPaymentMethodDto {
  /// Canonical payment_method_code sent back to the pay endpoint.
  final String methodCode;

  /// Human-readable method name for the picker.
  final String displayName;

  /// Payment-method fee F the backend will snapshot for this method.
  final int serviceFeeAmount;

  /// Total the gateway will charge for this method: shortage + F.
  final int grossAmount;

  const PromotionFundingPaymentMethodDto({
    required this.methodCode,
    required this.displayName,
    required this.serviceFeeAmount,
    required this.grossAmount,
  });

  factory PromotionFundingPaymentMethodDto.fromJson(Map<String, dynamic> json) {
    return PromotionFundingPaymentMethodDto(
      methodCode: json['method_code'] as String,
      displayName: json['display_name'] as String,
      serviceFeeAmount: (json['service_fee_amount'] as num?)?.toInt() ?? 0,
      grossAmount: (json['gross_amount'] as num?)?.toInt() ?? 0,
    );
  }
}

/// Read-only payment-method disclosure for one promotion funding obligation.
///
/// [shortageAmount] is the exact obligation the seller must pay — never the
/// promotion budget. Every method's gross is shortage + its fee.
class PromotionFundingPaymentMethodsDto {
  final int shortageAmount;
  final String currency;
  final List<PromotionFundingPaymentMethodDto> methods;

  const PromotionFundingPaymentMethodsDto({
    required this.shortageAmount,
    required this.currency,
    required this.methods,
  });

  factory PromotionFundingPaymentMethodsDto.fromJson(
    Map<String, dynamic> json,
  ) {
    return PromotionFundingPaymentMethodsDto(
      shortageAmount: (json['shortage_amount'] as num?)?.toInt() ?? 0,
      currency: json['currency'] as String? ?? 'IDR',
      methods: (json['methods'] as List<dynamic>? ?? const [])
          .map(
            (e) => PromotionFundingPaymentMethodDto.fromJson(
              e as Map<String, dynamic>,
            ),
          )
          .toList(),
    );
  }
}

/// Initiated exact-shortage payment for a promotion funding obligation.
///
/// WIRE AUTHORITY: POST /api/v1/promotions/contracts/payment-intent/:id/pay →
/// { data: { payment_id, payment_url, gross_amount, shortage, intent_id } }.
///
/// [grossAmount] is the amount the gateway will charge (shortage + fee) as
/// snapshotted by the backend payment engine — the client only redirects.
class PromotionFundingPaymentDto {
  final String paymentId;
  final String paymentUrl;
  final int grossAmount;
  final int shortage;

  const PromotionFundingPaymentDto({
    required this.paymentId,
    required this.paymentUrl,
    required this.grossAmount,
    required this.shortage,
  });

  factory PromotionFundingPaymentDto.fromJson(Map<String, dynamic> json) {
    return PromotionFundingPaymentDto(
      paymentId: json['payment_id'] as String? ?? '',
      paymentUrl: json['payment_url'] as String? ?? '',
      grossAmount: (json['gross_amount'] as num?)?.toInt() ?? 0,
      shortage: (json['shortage'] as num?)?.toInt() ?? 0,
    );
  }
}
