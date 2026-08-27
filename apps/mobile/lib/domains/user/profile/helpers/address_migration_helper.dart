import 'package:labuda/domains/user/profile/models/user_address.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';

/// Helper untuk migrasi dari UserAddress (single) ke AddressEntity (multiple)
class AddressMigrationHelper {
  /// Convert UserAddress to AddressEntity
  /// Auto-set label = "Home", isPrimary = true
  /// For old data, recipientName and phone will be empty strings (to be updated by user)
  static AddressEntity? convertToAddressEntity({
    required UserAddress userAddress,
    required String userId,
    String? addressId,
    String recipientName = '', // Will be updated by user
    String phone = '', // Will be updated by user
  }) {
    // Skip if essential fields are missing
    if (userAddress.province == null ||
        userAddress.city == null ||
        userAddress.district == null ||
        userAddress.village == null ||
        userAddress.streetAddress == null ||
        userAddress.streetAddress!.trim().isEmpty ||
        userAddress.postalCode == null ||
        userAddress.postalCode!.trim().isEmpty) {
      return null;
    }

    final now = DateTime.now();

    return AddressEntity(
      id: addressId ?? '', // Will be set by Firestore
      userId: userId,
      purpose: AddressPurpose
          .shipping, // Default: shipping address for migrated data
      nickname: null, // No nickname for migrated addresses
      recipientName: recipientName, // Empty by default, user should update
      phone: phone, // Empty by default, user should update
      province: userAddress.province!,
      city: userAddress.city!,
      district: userAddress.district!,
      village: userAddress.village!,
      streetAddress: userAddress.streetAddress!.trim(),
      postalCode: userAddress.postalCode!.trim(),
      notes: null, // UserAddress doesn't have notes
      isPrimary: true, // First address is always primary
      createdAt: now,
      updatedAt: now,
    );
  }

  /// Check if UserAddress has complete data for migration
  static bool isValidForMigration(UserAddress userAddress) {
    return userAddress.province != null &&
        userAddress.city != null &&
        userAddress.district != null &&
        userAddress.village != null &&
        userAddress.streetAddress != null &&
        userAddress.streetAddress!.trim().isNotEmpty &&
        userAddress.postalCode != null &&
        userAddress.postalCode!.trim().isNotEmpty;
  }
}
