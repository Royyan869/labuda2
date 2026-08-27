import 'package:labuda/domains/commerce/transaction/order/data/dto/order_confirmation_dto.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/domain.dart';

/// Mapper for OrderConfirmation
///
/// Converts between DTO and domain entity
class OrderConfirmationMapper {
  /// Convert DTO to domain entity
  static OrderConfirmation toEntity(OrderConfirmationDto dto) {
    return OrderConfirmation(
      id: dto.id,
      orderId: dto.orderId,
      buyerId: dto.buyerId,
      sellerId: dto.sellerId,
      startDate: dto.startDate,
      originalEndDate: dto.originalEndDate,
      extendedEndDate: dto.extendedEndDate,
      extensionUsed: dto.extensionUsed,
      status: ConfirmationStatusExtension.fromString(dto.status),
      createdAt: dto.createdAt,
      completedAt: dto.completedAt,
      completionReason: dto.completionReason,
      day5NotificationSent: dto.day5NotificationSent,
    );
  }

  /// Convert domain entity to DTO
  static OrderConfirmationDto toDto(OrderConfirmation entity) {
    return OrderConfirmationDto(
      id: entity.id,
      orderId: entity.orderId,
      buyerId: entity.buyerId,
      sellerId: entity.sellerId,
      startDate: entity.startDate,
      originalEndDate: entity.originalEndDate,
      extendedEndDate: entity.extendedEndDate,
      extensionUsed: entity.extensionUsed,
      status: entity.status.toStorageString(),
      createdAt: entity.createdAt,
      completedAt: entity.completedAt,
      completionReason: entity.completionReason,
      day5NotificationSent: entity.day5NotificationSent,
    );
  }

  /// Convert list of DTOs to entities
  static List<OrderConfirmation> toEntityList(List<OrderConfirmationDto> dtos) {
    return dtos.map((dto) => toEntity(dto)).toList();
  }
}
