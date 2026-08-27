import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';

/// Address Repository Interface
/// Handles CRUD operations for user addresses with purpose-based separation
abstract class IAddressRepository {
  /// Get all addresses for a user
  Future<Result<List<AddressEntity>>> getAddressesByUserId(String userId);

  /// Get addresses by user ID and purpose (shipping or sender)
  Future<Result<List<AddressEntity>>> getAddressesByPurpose(
    String userId,
    AddressPurpose purpose,
  );

  /// Get address by ID
  Future<Result<AddressEntity>> getAddressById(String addressId);

  /// Get primary address for user (optionally filter by purpose)
  Future<Result<AddressEntity?>> getPrimaryAddress(
    String userId, {
    AddressPurpose? purpose,
  });

  /// Add new address
  Future<Result<void>> addAddress(AddressEntity address);

  /// Update address
  Future<Result<void>> updateAddress(AddressEntity address);

  /// Delete address
  Future<Result<void>> deleteAddress(String addressId);

  /// Set address as primary (unset others atomically within the same purpose)
  Future<Result<void>> setPrimaryAddress(String addressId, String userId);

  /// Stream of addresses for real-time updates
  Stream<Result<List<AddressEntity>>> watchAddresses(String userId);

  /// Stream of addresses by purpose for real-time updates
  Stream<Result<List<AddressEntity>>> watchAddressesByPurpose(
    String userId,
    AddressPurpose purpose,
  );

  /// Count addresses for a user (optionally filter by purpose)
  Future<Result<int>> countAddresses(String userId, {AddressPurpose? purpose});
}
