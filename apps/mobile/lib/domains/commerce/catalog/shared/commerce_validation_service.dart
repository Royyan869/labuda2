/// Commerce-specific validation service
///
/// Contains business logic validation for commerce domain entities
/// including pricing, product specifications, and auction parameters.
library;

import 'package:labuda/core/common/result.dart';

/// Commerce-specific validators
class CommerceValidationService {
  /// Validate price for commerce products
  ///
  /// Business rules:
  /// - Price must be positive number
  /// - Maximum price limit enforced
  /// - Currency format validation (IDR)
  static Result<double> validatePrice(String price) {
    if (price.trim().isEmpty) {
      return Result.error('Harga tidak boleh kosong');
    }

    final parsedPrice = double.tryParse(price);
    if (parsedPrice == null) {
      return Result.error('Format harga tidak valid');
    }

    if (parsedPrice <= 0) {
      return Result.error('Harga harus lebih dari 0');
    }

    // Maximum 1 billion IDR
    if (parsedPrice > 1000000000) {
      return Result.error('Harga terlalu tinggi (maksimal Rp 1.000.000.000)');
    }

    return Result.success(parsedPrice);
  }

  /// Validate Koi fish size
  ///
  /// Business rules:
  /// - Size must be positive number in centimeters
  /// - Maximum size limit for realistic Koi fish
  /// - Decimal precision to 1 decimal place
  static Result<double> validateKoiSize(String size) {
    if (size.trim().isEmpty) {
      return Result.error('Ukuran koi tidak boleh kosong');
    }

    final parsedSize = double.tryParse(size);
    if (parsedSize == null) {
      return Result.error('Format ukuran tidak valid');
    }

    if (parsedSize <= 0) {
      return Result.error('Ukuran harus lebih dari 0 cm');
    }

    // Maximum 200cm (realistic maximum for Koi fish)
    if (parsedSize > 200) {
      return Result.error('Ukuran terlalu besar (maksimal 200 cm)');
    }

    return Result.success(parsedSize);
  }

  /// Validate Koi fish age
  ///
  /// Business rules:
  /// - Age must be positive number in months
  /// - Maximum age limit for realistic Koi fish lifespan
  static Result<int> validateKoiAge(String age) {
    if (age.trim().isEmpty) {
      return Result.error('Umur koi tidak boleh kosong');
    }

    final parsedAge = int.tryParse(age);
    if (parsedAge == null) {
      return Result.error('Format umur tidak valid');
    }

    if (parsedAge <= 0) {
      return Result.error('Umur harus lebih dari 0 bulan');
    }

    // Maximum 600 months (50 years, realistic maximum)
    if (parsedAge > 600) {
      return Result.error('Umur terlalu tua (maksimal 600 bulan / 50 tahun)');
    }

    return Result.success(parsedAge);
  }

  /// Validate auction starting price
  ///
  /// Business rules:
  /// - Starting price must meet minimum auction requirements
  /// - Maximum starting price enforced
  static Result<double> validateAuctionStartingPrice(String price) {
    final priceResult = validatePrice(price);
    if (priceResult.isError) {
      return priceResult;
    }

    final parsedPrice = priceResult.data!;

    // Minimum auction starting price (e.g., 100,000 IDR)
    if (parsedPrice < 100000) {
      return Result.error('Harga awal lelang minimal Rp 100.000');
    }

    return Result.success(parsedPrice);
  }

  /// Validate auction bid increment
  ///
  /// Business rules:
  /// - Bid increment must be positive
  /// - Minimum and maximum increment limits
  static Result<int> validateBidIncrement(String increment) {
    if (increment.trim().isEmpty) {
      return Result.error('Increment bid tidak boleh kosong');
    }

    final parsedIncrement = int.tryParse(increment);
    if (parsedIncrement == null) {
      return Result.error('Format increment tidak valid');
    }

    if (parsedIncrement <= 0) {
      return Result.error('Increment harus lebih dari 0');
    }

    // Minimum increment 10,000 IDR
    if (parsedIncrement < 10000) {
      return Result.error('Increment minimal Rp 10.000');
    }

    // Maximum increment 100,000,000 IDR
    if (parsedIncrement > 100000000) {
      return Result.error('Increment terlalu besar (maksimal Rp 100.000.000)');
    }

    return Result.success(parsedIncrement);
  }

  /// Validate stock quantity
  ///
  /// Business rules:
  /// - Stock must be non-negative integer
  /// - Maximum stock limit for inventory management
  static Result<int> validateStock(String stock) {
    if (stock.trim().isEmpty) {
      return Result.error('Stok tidak boleh kosong');
    }

    final parsedStock = int.tryParse(stock);
    if (parsedStock == null) {
      return Result.error('Format stok tidak valid');
    }

    if (parsedStock < 0) {
      return Result.error('Stok tidak boleh negatif');
    }

    // Maximum 10,000 items per listing
    if (parsedStock > 10000) {
      return Result.error('Stok terlalu banyak (maksimal 10.000 item)');
    }

    return Result.success(parsedStock);
  }

  /// Validate discount percentage
  ///
  /// Business rules:
  /// - Discount must be between 0-100
  static Result<int> validateDiscountPercentage(String discount) {
    if (discount.trim().isEmpty) {
      return Result.error('Diskon tidak boleh kosong');
    }

    final parsedDiscount = int.tryParse(discount);
    if (parsedDiscount == null) {
      return Result.error('Format diskon tidak valid');
    }

    if (parsedDiscount < 0) {
      return Result.error('Diskon tidak boleh negatif');
    }

    if (parsedDiscount > 100) {
      return Result.error('Diskon maksimal 100%');
    }

    return Result.success(parsedDiscount);
  }
}
