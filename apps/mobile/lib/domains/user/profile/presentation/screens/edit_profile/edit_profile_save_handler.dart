import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/shared.dart';
// R4.2: Import StorePhotoUploadService directly from owner (seller domain)
import 'package:hishumi/domains/user/preference/seller/data/data.dart'
    show StorePhotoUploadService;
import 'package:hishumi/domains/user/profile/domain/entities/profile_entity.dart';
import 'package:hishumi/domains/user/profile/data/services/cover_photo_upload_service.dart';
import 'package:hishumi/domains/user/preference/seller/data/seller_providers.dart'
    show sellerRemoteDatasourceProvider;
import 'package:hishumi/domains/user/profile/data/services/avatar_upload_service.dart';

/// Mixin for handling save operations in edit profile screen.
///
/// CANONICAL SAVE AUTHORITY: this mixin is the *only* client save
/// orchestrator for Edit Profile. It performs exactly one profile update
/// (via [AuthController.updateProfile] → `PATCH /users/me/profile`) and, for
/// sellers, one seller update (`PATCH /seller/profile`). No other writer
/// touches the profile endpoint from this surface.
mixin EditProfileSaveHandler<T extends ConsumerStatefulWidget>
    on ConsumerState<T> {
  // Required getters - must be implemented by mixing class
  GlobalKey<FormState> get formKey;
  @override
  WidgetRef get ref;
  String get actualUserId;
  bool get isSeller;

  // Controllers
  TextEditingController get usernameController;
  TextEditingController get bioController;
  TextEditingController get farmNameController;
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

  // Services
  CoverPhotoUploadService get coverPhotoUploadService;
  AvatarUploadService get avatarUploadService;
  StorePhotoUploadService get storePhotoUploadService;

  // Setters for loading state
  void setLoading(bool loading);

  /// Reads the direct profile response after a write and returns the avatar,
  /// cover, and cover-updated-at as persisted by the backend.
  ProfileEntity? get cachedProfile;

  Future<void> save() async {
    // Validate all fields at once. Field-level errors render next to the
    // offending field; do not blanket the whole form.
    if (!formKey.currentState!.validate()) {
      return;
    }

    setLoading(true);

    // SUBMISSION SNAPSHOT: capture the text/social/store intent BEFORE the
    // upload sequence begins. The uploads await, and the PATCH payload is built
    // afterwards; reading the live controllers there would let an edit during
    // upload silently change the in-flight save.
    final bioSnapshot = bioController.text.trim();
    final socials = (
      instagram: _socialValue(instagramController),
      facebook: _socialValue(facebookController),
      tiktok: _socialValue(tiktokController),
      twitter: _socialValue(twitterController),
    );
    final storeNameSnapshot = farmNameController.text.trim();

    try {
      // 1. Avatar upload (new selection or removal) — produce the reference.
      final avatarResolution = await _resolveAvatarReference();
      if (avatarResolution.isError) {
        if (mounted) {
          AppSnackBar.showError(context, avatarResolution.error!);
          setLoading(false);
        }
        return;
      }

      // 2. Cover upload (new selection or removal) — produce the reference.
      final coverResolution = await _resolveCoverReference();
      if (coverResolution.isError) {
        if (mounted) {
          AppSnackBar.showError(context, coverResolution.error!);
          setLoading(false);
        }
        return;
      }
      final coverChanged = coverResolution.hasUpdate;
      final coverReference = coverResolution.data;

      // 3. Seller store photo upload (new selection or removal).
      String? storeImageReference;
      var storeImageChanged = false;
      if (isSeller) {
        final storeResolution = await _resolveStoreImageReference(
          storeNameSnapshot,
        );
        if (storeResolution.isError) {
          if (mounted) {
            AppSnackBar.showError(context, storeResolution.error!);
            setLoading(false);
          }
          return;
        }
        storeImageReference = storeResolution.data;
        storeImageChanged = storeResolution.hasUpdate;
      }

      // 4. ONE profile update — the single writer for /users/me/profile.
      final profileSuccess = await _saveProfile(
        bio: bioSnapshot,
        socials: socials,
        storeName: storeNameSnapshot,
        avatarReference: avatarResolution.data,
        coverReference: coverChanged ? coverReference : null,
        includeCover: coverChanged,
      );
      if (!profileSuccess) {
        if (mounted) setLoading(false);
        return;
      }

      // 5. Seller store identity update (only when the store field changed).
      if (isSeller && storeImageChanged) {
        try {
          await ref.read(sellerRemoteDatasourceProvider).updateSellerProfile(
            storeName: storeNameSnapshot,
            storeImageUrl: storeImageReference,
          );
        } catch (_) {
          if (mounted) {
            AppSnackBar.showError(
              context,
              'Nama atau foto toko belum bisa disimpan. Coba lagi.',
            );
            setLoading(false);
          }
          return;
        }
      }

      // 6. Success — clear dirty state and return to the previous screen.
      if (mounted) {
        AppSnackBar.showSuccess(context, 'Profile berhasil diperbarui');
        Navigator.of(context).pop(true);
      }
    } finally {
      if (mounted) setLoading(false);
    }
  }

  /// Resolves the avatar reference to persist. Returns an error when an
  /// upload fails; data is the storage reference (null = removed).
  Future<({bool isError, String? data, String? error})> _resolveAvatarReference() async {
    if (isAvatarMarkedForRemoval) {
      await avatarUploadService.deleteAvatar(actualUserId);
      return (isError: false, data: null, error: null);
    }
    if (selectedAvatarPath != null) {
      final result = await avatarUploadService.uploadAvatar(
        userId: actualUserId,
        imagePath: selectedAvatarPath!,
      );
      if (!result.isSuccess) {
        _logUploadFailure('avatar', result.error);
        return (
          isError: true,
          data: null,
          error: 'Foto profil gagal diunggah. Coba lagi.',
        );
      }
      return (isError: false, data: result.data, error: null);
    }
    return (isError: false, data: avatarUrl, error: null);
  }

  /// Resolves the cover reference to persist, using the canonical storage key
  /// contract: a new upload persists the STORAGE KEY; removal persists the
  /// empty-string clear signal (backend → NULL). [hasUpdate] is false when the
  /// cover was untouched, so the field is omitted from the request entirely.
  Future<({bool isError, bool hasUpdate, String? data, String? error})>
  _resolveCoverReference() async {
    if (isCoverMarkedForRemoval) {
      return (isError: false, hasUpdate: true, data: '', error: null);
    }
    if (selectedCoverPath != null) {
      final result = await coverPhotoUploadService.uploadCoverPhoto(
        userId: actualUserId,
        imagePath: selectedCoverPath!,
      );
      if (!result.isSuccess) {
        _logUploadFailure('cover', result.error);
        return (
          isError: true,
          hasUpdate: false,
          data: null,
          error: 'Foto sampul gagal diunggah. Coba lagi.',
        );
      }
      return (
        isError: false,
        hasUpdate: true,
        data: result.data!.storageKey,
        error: null,
      );
    }
    return (isError: false, hasUpdate: false, data: null, error: null);
  }

  /// Resolves the seller store-image reference. hasUpdate=false when the store
  /// identity did not change; otherwise `data` is the storage key (new image)
  /// or empty string (removal).
  Future<({bool isError, bool hasUpdate, String? data, String? error})>
  _resolveStoreImageReference(String storeName) async {
    final storeNameChanged =
        cachedProfile?.farmInfo?.farmName.trim() != storeName;

    if (isStorePhotoMarkedForRemoval) {
      return (isError: false, hasUpdate: true, data: '', error: null);
    }
    if (selectedStorePhotoPath != null) {
      final result = await storePhotoUploadService.uploadStorePhoto(
        userId: actualUserId,
        imagePath: selectedStorePhotoPath!,
      );
      if (!result.isSuccess) {
        _logUploadFailure('store photo', result.error);
        return (
          isError: true,
          hasUpdate: false,
          data: null,
          error: 'Foto toko gagal diunggah. Coba lagi.',
        );
      }
      return (isError: false, hasUpdate: true, data: result.data!.storageKey, error: null);
    }
    // No new image → only send the store_name change (if any).
    if (storeNameChanged) {
      return (isError: false, hasUpdate: true, data: null, error: null);
    }
    return (isError: false, hasUpdate: false, data: null, error: null);
  }

  /// The single profile writer. Sends bio, avatar, cover (only when changed)
  /// and social handles in one `PATCH /users/me/profile`.
  Future<bool> _saveProfile({
    required String bio,
    required ({String? instagram, String? facebook, String? tiktok, String? twitter}) socials,
    required String storeName,
    required String? avatarReference,
    required String? coverReference,
    required bool includeCover,
  }) async {
    if (isSeller && storeName.isEmpty) {
      if (mounted) {
        AppSnackBar.showError(context, 'Nama toko/farm wajib diisi');
      }
      return false;
    }

    final authController = ref.read(authControllerProvider.notifier);

    // Username is IMMUTABLE after registration (canonical identity) and is
    // never sent. Social handles: empty clears the handle; presence = visible.
    final success = await authController.updateProfile(
      photoUrl: avatarReference,
      bio: bio.isEmpty ? null : bio,
      coverPhotoUrl: includeCover ? coverReference : null,
      instagramHandle: socials.instagram,
      facebookHandle: socials.facebook,
      tiktokHandle: socials.tiktok,
      twitterHandle: socials.twitter,
    );

    if (!success && mounted) {
      AppSnackBar.showError(
        context,
        'Perubahan profile belum bisa disimpan. Periksa isian lalu coba lagi.',
      );
    }
    return success;
  }

  /// Empty text becomes an explicit empty string so the backend clears the
  /// handle; non-empty is sent as-is (trimmed).
  String _socialValue(TextEditingController controller) =>
      controller.text.trim();

  /// Logs the technical upload failure for diagnostics while the UI shows a
  /// fixed user-facing message. Technical detail must never reach the user.
  void _logUploadFailure(String asset, Object? technicalError) {
    ref
        .read(loggerServiceProvider)
        .error('Edit Profile upload failed ($asset)', extra: {
          'error': technicalError?.toString(),
        });
  }
}
