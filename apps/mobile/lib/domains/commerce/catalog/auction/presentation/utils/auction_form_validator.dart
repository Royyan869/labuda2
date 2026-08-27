/// Auction Form Validator
///
/// Pure validation utilities for auction form input.
/// Extracted from admin_stubs.dart - SAFe cleanup pass.
library;

import 'package:flutter/material.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';

/// Validator for auction creation/editing forms
class AuctionFormValidator {
  static String? validateTitle(String? value) {
    if (value == null || value.isEmpty) return 'Title is required';
    if (value.length < 10) return 'Title must be at least 10 characters';
    return null;
  }

  static String? validateDescription(String? value) {
    if (value == null || value.isEmpty) return 'Description is required';
    if (value.length < 20) return 'Description must be at least 20 characters';
    return null;
  }

  static String? validateStartingBid(String? value) {
    if (value == null || value.isEmpty) return 'Starting bid is required';
    final amount = double.tryParse(value);
    if (amount == null || amount <= 0) return 'Invalid amount';
    return null;
  }

  static String? validateEndDate(DateTime? value) {
    if (value == null) return 'End date is required';
    if (value.isBefore(DateTime.now())) return 'End date must be in the future';
    return null;
  }

  // Step 1 validation for create auction screen
  bool validateStep1({
    required String title,
    required List<dynamic> mediaFiles,
    required List<String> existingMediaUrls,
    required String? variety,
    required double? size,
    required int? age,
    required String? gender,
  }) {
    if (title.isEmpty) return false;
    if (mediaFiles.isEmpty && existingMediaUrls.isEmpty) return false;
    if (variety == null || variety.isEmpty) return false;
    if (size == null || size <= 0) return false;
    if (age == null || age < 0) return false;
    if (gender == null || gender.isEmpty) return false;
    return true;
  }

  // Step 2 validation for create auction screen
  bool validateStep2({
    required double? openingBid,
    required double? bidIncrement,
  }) {
    if (openingBid == null || openingBid <= 0) return false;
    if (bidIncrement == null || bidIncrement <= 0) return false;
    return true;
  }

  // Show validation warning dialog
  void showValidationWarning(BuildContext context) {
    AppSnackBar.showWarning(context, 'Please complete all required fields');
  }
}
