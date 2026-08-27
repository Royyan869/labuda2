import 'dart:io';

import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Avatar Section Widget for Edit Profile
/// Shows single avatar for buyers, dual avatars (personal + farm) for sellers
class EditProfileAvatarSection extends StatelessWidget {
  final bool isSeller;
  final String? avatarUrl;
  final String? selectedAvatarPath;
  final bool isAvatarMarkedForRemoval;
  final VoidCallback onChangeAvatar;
  final VoidCallback onRemoveAvatar;
  // Seller-only fields
  final String? farmPhotoUrl;
  final String? selectedStorePhotoPath;
  final bool isStorePhotoMarkedForRemoval;
  final VoidCallback? onChangeStorePhoto;
  final VoidCallback? onRemoveStorePhoto;

  const EditProfileAvatarSection({
    super.key,
    required this.isSeller,
    this.avatarUrl,
    this.selectedAvatarPath,
    required this.isAvatarMarkedForRemoval,
    required this.onChangeAvatar,
    required this.onRemoveAvatar,
    this.farmPhotoUrl,
    this.selectedStorePhotoPath,
    this.isStorePhotoMarkedForRemoval = false,
    this.onChangeStorePhoto,
    this.onRemoveStorePhoto,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    if (isSeller) {
      return Row(
        mainAxisAlignment: MainAxisAlignment.spaceEvenly,
        children: [
          _AvatarItem(
            isDark: isDark,
            label: 'Personal Avatar',
            url: avatarUrl,
            selectedPath: selectedAvatarPath,
            isMarkedForRemoval: isAvatarMarkedForRemoval,
            onTap: onChangeAvatar,
            onRemove: onRemoveAvatar,
          ),
          _AvatarItem(
            isDark: isDark,
            label: 'Farm Photo',
            url: farmPhotoUrl,
            selectedPath: selectedStorePhotoPath,
            isMarkedForRemoval: isStorePhotoMarkedForRemoval,
            onTap: onChangeStorePhoto ?? () {},
            onRemove: onRemoveStorePhoto ?? () {},
          ),
        ],
      );
    }

    return Center(
      child: _AvatarItem(
        isDark: isDark,
        label: 'Profile Photo',
        url: avatarUrl,
        selectedPath: selectedAvatarPath,
        isMarkedForRemoval: isAvatarMarkedForRemoval,
        onTap: onChangeAvatar,
        onRemove: onRemoveAvatar,
      ),
    );
  }
}

class _AvatarItem extends StatelessWidget {
  final bool isDark;
  final String label;
  final String? url;
  final String? selectedPath;
  final bool isMarkedForRemoval;
  final VoidCallback onTap;
  final VoidCallback onRemove;

  const _AvatarItem({
    required this.isDark,
    required this.label,
    this.url,
    this.selectedPath,
    required this.isMarkedForRemoval,
    required this.onTap,
    required this.onRemove,
  });

  bool get _hasImage =>
      (url != null || selectedPath != null) && !isMarkedForRemoval;

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        GestureDetector(
          onTap: onTap,
          child: Stack(
            children: [
              CircleAvatar(
                radius: 60,
                backgroundColor: isDark
                    ? AppColors.darkGray700
                    : AppColors.neutralGray200,
                backgroundImage: _getBackgroundImage(),
                child: !_hasImage && selectedPath == null
                    ? Icon(
                        Icons.person,
                        size: 60,
                        color: AppColors.neutralGray500,
                      )
                    : null,
              ),
              Positioned(
                bottom: 0,
                right: 0,
                child: CircleAvatar(
                  radius: 18,
                  backgroundColor: AppColors.primaryRed,
                  child: const Icon(
                    Icons.camera_alt,
                    size: 18,
                    color: Colors.white,
                  ),
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 8),
        Text(
          label,
          style: TextStyle(
            fontSize: 12,
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
          ),
        ),
        if (_hasImage)
          TextButton(
            onPressed: onRemove,
            child: Text(
              'Remove',
              style: TextStyle(color: AppColors.primaryRed, fontSize: 12),
            ),
          ),
      ],
    );
  }

  ImageProvider? _getBackgroundImage() {
    if (selectedPath != null) {
      return FileImage(File(selectedPath!));
    }
    if (url != null && !isMarkedForRemoval) {
      return NetworkImage(url!);
    }
    return null;
  }
}
