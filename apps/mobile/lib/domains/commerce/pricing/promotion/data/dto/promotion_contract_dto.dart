library;

/// Promotion Contract DTO — canonical authority promotion_contracts
/// Maps GET /api/v1/promotions/contracts and POST /api/v1/promotions/contracts
/// Uses budget, CPM, duration, geography, queue — NOT package/ownership.

class PromotionContractDto {
  final String id;
  final String sellerId;
  final String kind; // internal|external
  final String status; // active|paused|finalized
  final int budgetRupiah;
  final int cpmRupiah;
  final String plannedStart;
  final String plannedFinish;
  final String allocationAccountId;
  final String? pausedAt;
  final String? finalizedAt;
  final String createdAt;
  final String updatedAt;
  final List<String> cityIds; // promotion_contract_geographies city_id set, empty = nationwide

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
      cityIds: (json['city_ids'] as List<dynamic>? ?? []).map((e) => e as String).toList(),
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
