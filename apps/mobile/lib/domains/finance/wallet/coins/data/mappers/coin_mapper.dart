import 'package:labuda/domains/finance/wallet/coins/domain/entities/coin_balance.dart';
import 'package:labuda/domains/finance/wallet/coins/domain/entities/coin_transaction.dart';
import 'package:labuda/domains/finance/wallet/coins/data/dto/coin_dto.dart';

/// Mapper for converting between DTOs and entities.
///
/// This is a simple pass-through mapper since the DTO
/// extends the entity pattern. In a more complex scenario,
/// this would handle data transformation logic.
class CoinMapper {
  /// Converts [CoinBalanceDto] to [CoinBalance] entity.
  static CoinBalance balanceToEntity(CoinBalanceDto dto) {
    return dto.toEntity();
  }

  /// Converts [CoinBalance] entity to [CoinBalanceDto].
  static CoinBalanceDto balanceToDto(CoinBalance entity) {
    return CoinBalanceDto.fromEntity(entity);
  }

  /// Converts [CoinTransactionDto] to [CoinTransaction] entity.
  static CoinTransaction transactionToEntity(CoinTransactionDto dto) {
    return dto.toEntity();
  }

  /// Converts [CoinTransaction] entity to [CoinTransactionDto].
  static CoinTransactionDto transactionToDto(CoinTransaction entity) {
    return CoinTransactionDto.fromEntity(entity);
  }

  /// Converts list of DTOs to list of entities.
  static List<CoinTransaction> transactionsToEntities(
    List<CoinTransactionDto> dtos,
  ) {
    return dtos.map((dto) => dto.toEntity()).toList();
  }

  /// Converts list of entities to list of DTOs.
  static List<CoinTransactionDto> transactionsToDtos(
    List<CoinTransaction> entities,
  ) {
    return entities
        .map((entity) => CoinTransactionDto.fromEntity(entity))
        .toList();
  }
}
