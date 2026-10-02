import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';

/// Address Repository Interface
/// Handles CRUD operations for the account's single address book.
///
/// One address book per account; `tag` narrows a read to addresses carrying
/// that role tag (shipping / sender). There is no per-tag primary: the
/// account has exactly one primary address.
abstract class IAddressRepository {
  /// Get all addresses for a user
  Future<Result<List<AddressEntity>>> getAddressesByUserId(String userId);

  /// Get addresses for a user that carry [tag] (shipping or sender)
  Future<Result<List<AddressEntity>>> getAddressesByTag(
    String userId,
    AddressTag tag,
  );

  /// Get address by ID
  Future<Result<AddressEntity>> getAddressById(String addressId);

  /// Get the account's primary address, optionally narrowed to [tag]
  Future<Result<AddressEntity?>> getPrimaryAddress(
    String userId, {
    AddressTag? tag,
  });

  /// Add new address
  Future<Result<void>> addAddress(AddressEntity address);

  /// Update address
  Future<Result<void>> updateAddress(AddressEntity address);

  /// Delete address
  Future<Result<void>> deleteAddress(String addressId);

  /// Set address as primary (unsets the account's previous primary atomically)
  Future<Result<void>> setPrimaryAddress(String addressId, String userId);

  /// Stream of addresses for real-time updates
  Stream<Result<List<AddressEntity>>> watchAddresses(String userId);

  /// Stream of addresses carrying [tag] for real-time updates
  Stream<Result<List<AddressEntity>>> watchAddressesByTag(
    String userId,
    AddressTag tag,
  );

  /// Count addresses for a user (optionally narrowed to [tag])
  Future<Result<int>> countAddresses(String userId, {AddressTag? tag});
}
