/// Withdraw Mapper
///
/// Converts between withdrawal DTOs and domain entities.
library;

import '../../domain/entities/withdrawal.dart';
import '../dto/withdraw_dto.dart';

/// WithdrawalMapper converts between DTOs and entities
class WithdrawalMapper {
  /// Convert WithdrawalItemDto to Withdrawal entity
  static Withdrawal fromDto(WithdrawalItemDto dto) {
    return Withdrawal(
      id: dto.id,
      sellerId: dto.sellerId,
      amount: dto.amount.toDouble(),
      feeAmount: dto.feeAmount.toDouble(),
      status: WithdrawalStatus.fromString(dto.status),
      bankNameSnapshot: dto.bankNameSnapshot,
      bankCodeSnapshot: dto.bankCodeSnapshot,
      accountNumberSnapshot: dto.accountNumberSnapshot,
      accountHolderSnapshot: dto.accountHolderSnapshot,
      createdAt: DateTime.parse(dto.createdAt),
      updatedAt: DateTime.parse(dto.updatedAt),
    );
  }

  /// Convert list of WithdrawalItemDto to list of Withdrawal entities
  static List<Withdrawal> fromDtoList(List<WithdrawalItemDto> dtos) {
    return dtos.map((dto) => fromDto(dto)).toList();
  }

  /// Convert WithdrawRequest entity to WithdrawRequestDto
  static WithdrawRequestDto requestToDto(WithdrawRequest request) {
    return WithdrawRequestDto(amount: request.amount.toInt());
  }

  /// Convert WithdrawResponseDto to WithdrawResult entity
  static WithdrawResult responseToResult(WithdrawResponseDto dto) {
    return WithdrawResult(
      withdrawalId: dto.withdrawalId,
      status: WithdrawalStatus.fromString(dto.status),
      feeAmount: dto.feeAmount.toDouble(),
      totalDebitAmount: dto.totalDebitAmount.toDouble(),
    );
  }
}
