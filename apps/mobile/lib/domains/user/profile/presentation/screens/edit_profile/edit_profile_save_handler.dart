import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
// R4.2: Import StorePhotoUploadService directly from owner (seller domain)
import 'package:labuda/domains/user/preference/seller/data/data.dart'
    show StorePhotoUploadService;
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';
import 'package:labuda/domains/user/profile/presentation/providers/profile_core_provider.dart';
import 'package:labuda/domains/user/profile/data/services/cover_photo_upload_service.dart';
import 'package:labuda/domains/user/profile/data/services/avatar_upload_service.dart';
import 'edit_profile_validators.dart';

/// Mixin for handling save operations in edit profile screen
mixin EditProfileSaveHandler<T extends ConsumerStatefulWidget>
    on ConsumerState<T> {
  // Required getters - must be implemented by mixing class
  GlobalKey<FormState> get formKey;
  @override
  WidgetRef get ref;
  String get actualUserId;
  bool get isSeller;
  ProfileEntity? get cachedProfile;

  // Controllers
  TextEditingController get usernameController;
  TextEditingController get bioController;
  TextEditingController get farmNameController;
  TextEditingController get websiteController;
  TextEditingController get instagramController;
  TextEditingController get facebookController;
  TextEditingController get tiktokController;
  TextEditingController get twitterController;

  // State
  String? get avatarUrl;
  String? get selectedAvatarPath;
  bool get isAvatarMarkedForRemoval;
  String? get coverPhotoUrl;
  String? get selectedCoverPath;
  bool get isCoverMarkedForRemoval;
  String? get farmPhotoUrl;
  String? get selectedStorePhotoPath;
  bool get isStorePhotoMarkedForRemoval;
  DateTime? get establishedDate;
  bool get isEmailPublic;
  bool get isPhonePublic;
  bool get isSocialMediaPublic;

  // Services
  CoverPhotoUploadService get coverPhotoUploadService;
  AvatarUploadService get avatarUploadService;
  StorePhotoUploadService get storePhotoUploadService;

  // Setters for loading state
  void setLoading(bool loading);

  Future<void> save() async {
    // Validate all fields at once
    if (!formKey.currentState!.validate()) {
      if (mounted) {
        AppSnackBar.showError(context, 'Please fill all required fields');
      }
      return;
    }

    setLoading(true);

    try {
      // Save personal data first (auth user)
      final personalSuccess = await savePersonal();
      if (!personalSuccess) {
        if (mounted) setLoading(false);
        return;
      }

      // Save all profile fields in a single call to avoid race conditions
      final profileSuccess = await saveProfileFields();
      if (!profileSuccess) {
        if (mounted) setLoading(false);
        return;
      }

      // All data saved successfully - show success message
      if (mounted) {
        AppSnackBar.showSuccess(context, 'Profile updated successfully');

        // Navigate back to previous screen (pop instead of go)
        Navigator.of(context).pop();
      }
    } finally {
      if (mounted) setLoading(false);
    }
  }

  /// Save all profile fields in a single update call
  Future<bool> saveProfileFields() async {
    // Use cached profile instead of re-reading from stream
    // This avoids race conditions when stream is still loading
    if (cachedProfile == null) {
      if (mounted) {
        AppSnackBar.showError(
          context,
          'Profile data not loaded yet. Please wait.',
        );
      }
      return false;
    }

    final profile = cachedProfile!;
    final fields = <String, dynamic>{};

    // 1. Handle cover photo upload
    final coverResult = await prepareCoverPhoto();
    if (coverResult == null) return false; // Error occurred
    if (coverResult.hasUpdate) {
      fields['coverPhotoUrl'] = coverResult.url;
    }

    // 2. Handle farm info (seller only)
    if (isSeller) {
      final farmResult = await prepareFarmInfo(profile);
      if (farmResult == null) return false; // Error occurred
      fields['farmInfo'] = farmResult;
    }

    // 3. Prepare contact info
    final contactInfo = prepareContactInfo();
    fields['contactInfo'] = contactInfo;

    // 4. Update all fields in a single call
    if (fields.isNotEmpty) {
      try {
        await ref.read(profileActionsProvider).updateFields(profile, fields);
      } catch (e) {
        if (mounted) {
          AppSnackBar.showError(
            context,
            'Perubahan belum bisa disimpan. Coba lagi.',
          );
        }
        return false;
      }
    }

    return true;
  }

  /// Prepare cover photo (upload if needed; clear via empty-string PATCH).
  /// Returns null on error, or result with hasUpdate=false if no changes.
  ///
  /// Canonical semantics (STAGE 4F-1/4F-2):
  /// - New/updated cover → upload to the canonical fixed key and persist the
  ///   STORAGE KEY (never the resolved read URL).
  /// - Removal → PATCH cover_photo_url = "" (backend converts to NULL).
  ///   There is no delete-media endpoint by design.
  Future<({bool hasUpdate, String? url})?> prepareCoverPhoto() async {
    // Handle cover removal — clear the DB reference, no S3 delete.
    if (isCoverMarkedForRemoval) {
      return (hasUpdate: true, url: '');
    }

    // Handle new cover upload
    if (selectedCoverPath != null) {
      final result = await coverPhotoUploadService.uploadCoverPhoto(
        userId: actualUserId,
        imagePath: selectedCoverPath!,
      );

      if (!result.isSuccess) {
        if (mounted) {
          AppSnackBar.showError(context, result.error!);
        }
        return null;
      }
      // Persist the storage key — the backend authority stores the key and
      // resolves it to a read URL on hydration.
      return (hasUpdate: true, url: result.data!.storageKey);
    }

    // No changes
    return (hasUpdate: false, url: coverPhotoUrl);
  }

  /// Prepare farm info (upload store photo if needed)
  Future<FarmInfo?> prepareFarmInfo(ProfileEntity profile) async {
    String? photoUrl = farmPhotoUrl;

    // Handle store photo removal
    if (isStorePhotoMarkedForRemoval) {
      photoUrl = null;
    }
    // Handle new store photo upload
    else if (selectedStorePhotoPath != null) {
      final result = await storePhotoUploadService.uploadStorePhoto(
        userId: actualUserId,
        imagePath: selectedStorePhotoPath!,
      );
      if (!result.isSuccess) {
        if (mounted) {
          AppSnackBar.showError(context, result.error!);
        }
        return null;
      }
      photoUrl = result.data;
    }

    return FarmInfo(
      farmName: farmNameController.text.trim(),
      farmPhotoUrl: photoUrl,
      farmWebsite: websiteController.text.trim().isEmpty
          ? null
          : websiteController.text.trim(),
      specialties: profile.farmInfo?.specialties,
      establishedDate: establishedDate,
    );
  }

  /// Prepare contact info (no async operation needed)
  ContactInfo prepareContactInfo() {
    final authState = ref.read(authControllerProvider);
    final user = authState is AuthStateAuthenticated ? authState.user : null;

    final maskedEmail = user?.email.isNotEmpty == true
        ? EditProfileValidators.maskEmail(user!.email)
        : null;
    final maskedPhone = user?.phoneNumber?.isNotEmpty == true
        ? EditProfileValidators.maskPhone(user!.phoneNumber!)
        : null;

    return ContactInfo(
      maskedEmail: maskedEmail,
      maskedPhone: maskedPhone,
      isEmailPublic: isEmailPublic,
      isPhonePublic: isPhonePublic,
      instagramHandle: instagramController.text.trim().isEmpty
          ? null
          : instagramController.text.trim(),
      facebookHandle: facebookController.text.trim().isEmpty
          ? null
          : facebookController.text.trim(),
      tiktokHandle: tiktokController.text.trim().isEmpty
          ? null
          : tiktokController.text.trim(),
      twitterHandle: twitterController.text.trim().isEmpty
          ? null
          : twitterController.text.trim(),
      isSocialMediaPublic: isSocialMediaPublic,
    );
  }

  Future<bool> savePersonal() async {
    final authController = ref.read(authControllerProvider.notifier);
    String? photoUrl = avatarUrl;

    if (isAvatarMarkedForRemoval) {
      photoUrl = null;
      // Delete from AWS S3
      await avatarUploadService.deleteAvatar(actualUserId);
    } else if (selectedAvatarPath != null) {
      // Upload avatar to AWS S3
      final result = await avatarUploadService.uploadAvatar(
        userId: actualUserId,
        imagePath: selectedAvatarPath!,
      );

      if (!result.isSuccess) {
        if (mounted) {
          AppSnackBar.showError(context, result.error!);
        }
        return false;
      }
      photoUrl = result.data;
    }

    // Username is IMMUTABLE after registration (canonical identity). It is NOT
    // included in the profile-update payload — the backend would reject any
    // change for an already-set username anyway (USERNAME_ALREADY_SET), but we
    // do not attempt a mutation from this surface at all.
    final success = await authController.updateProfile(
      photoUrl: photoUrl,
      bio: bioController.text.trim().isEmpty ? null : bioController.text.trim(),
      phoneNumber: null,
    );

    if (!success && mounted) {
      AppSnackBar.showError(context, 'Failed to update personal information');
    }

    return success;
  }
}
