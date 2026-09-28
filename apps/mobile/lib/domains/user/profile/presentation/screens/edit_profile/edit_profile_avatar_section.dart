import 'dart:io';

import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

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
    if (isSeller) {
      return Row(
        mainAxisAlignment: MainAxisAlignment.spaceEvenly,
        children: [
          _AvatarItem(
            label: 'Personal Avatar',
            url: avatarUrl,
            selectedPath: selectedAvatarPath,
            isMarkedForRemoval: isAvatarMarkedForRemoval,
            onTap: onChangeAvatar,
            onRemove: onRemoveAvatar,
          ),
          _AvatarItem(
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
  final String label;
  final String? url;
  final String? selectedPath;
  final bool isMarkedForRemoval;
  final VoidCallback onTap;
  final VoidCallback onRemove;

  const _AvatarItem({
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
    final scheme = Theme.of(context).colorScheme;
    return Column(
      children: [
        GestureDetector(
          onTap: onTap,
          child: Stack(
            children: [
              CircleAvatar(
                radius: 60,
                backgroundColor: scheme.surfaceContainerHighest,
                backgroundImage: _getBackgroundImage(),
                child: !_hasImage && selectedPath == null
                    ? Icon(
                        Icons.person,
                        size: 60,
                        color: scheme.onSurfaceVariant,
                      )
                    : null,
              ),
              Positioned(
                bottom: 0,
                right: 0,
                child: CircleAvatar(
                  radius: 18,
                  backgroundColor: scheme.primary,
                  child: Icon(
                    Icons.camera_alt,
                    size: 18,
                    color: scheme.onPrimary,
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
            fontSize: AppType.s12,
            color: scheme.onSurfaceVariant,
          ),
        ),
        if (_hasImage)
          TextButton(
            onPressed: onRemove,
            child: Text(
              'Remove',
              style: TextStyle(color: scheme.primary, fontSize: AppType.s12),
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
