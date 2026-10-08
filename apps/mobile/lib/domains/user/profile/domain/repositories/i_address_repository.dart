import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';

/// Address Repository Interface
/// Handles CRUD operations for the account's single address book.
///
/// The account owns one address book with exactly one primary address (when it
/// has any active address). The primary address is the account's default.
abstract class IAddressRepository {
  /// Get all addresses for a user
  Future<Result<List<AddressEntity>>> getAddressesByUserId(String userId);

  /// Get address by ID
  Future<Result<AddressEntity>> getAddressById(String addressId);

  /// Get the account's primary address (null when the account has none)
  Future<Result<AddressEntity?>> getPrimaryAddress(String userId);

  /// Add new address
  Future<Result<void>> addAddress(AddressEntity address);

  /// Update address
  Future<Result<void>> updateAddress(AddressEntity address);

  /// Delete address
  Future<Result<void>> deleteAddress(String addressId);

  /// Set address as primary (unsets the account's previous primary atomically)
  Future<Result<void>> setPrimaryAddress(String addressId, String userId);

  /// Count addresses for a user
  Future<Result<int>> countAddresses(String userId);
}
