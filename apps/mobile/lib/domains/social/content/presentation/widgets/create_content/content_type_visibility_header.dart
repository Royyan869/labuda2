import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';

/// Widget for user header with visibility dropdown
///
/// Displays:
/// - User avatar and name
/// - Visibility dropdown (Public, Followers, Private)
class ContentVisibilityHeader extends StatelessWidget {
  final AuthUser? authenticatedUser;
  final String postVisibility;
  final ValueChanged<String> onVisibilityChanged;

  const ContentVisibilityHeader({
    super.key,
    required this.authenticatedUser,
    required this.postVisibility,
    required this.onVisibilityChanged,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final user = authenticatedUser;

    return Container(
      padding: const EdgeInsets.fromLTRB(16, 6, 16, 6),
      child: Row(
        children: [
          if (user != null)
            ProfileAvatar(
              userId: user.id,
              size: 40,
              imageUrl: user.avatarUrl,
            )
          else
            Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(
                color: isDark
                    ? AppColors.darkGray700
                    : AppColors.neutralGray200,
                shape: BoxShape.circle,
              ),
              child: Icon(
                Icons.person_outline,
                size: 20,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray500,
              ),
            ),
          const SizedBox(width: 12),
          Expanded(
            child: user != null && user.username.isNotEmpty
                ? Text(
                    '@${user.username}',
                    style: const TextStyle(
                      fontWeight: FontWeight.w600,
                      fontSize: 16,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  )
                : Container(
                    height: 14,
                    decoration: BoxDecoration(
                      color: isDark
                          ? AppColors.darkGray700
                          : AppColors.neutralGray200,
                      borderRadius: BorderRadius.circular(999),
                    ),
                  ),
          ),
          const SizedBox(width: 8),
          _buildVisibilityDropdown(isDark),
        ],
      ),
    );
  }

  Widget _buildVisibilityDropdown(bool isDark) {
    return Container(
      width: 116,
      height: 32,
      padding: const EdgeInsets.symmetric(horizontal: 8),
      decoration: BoxDecoration(
        border: Border.all(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray300,
        ),
        borderRadius: BorderRadius.circular(6),
      ),
      child: DropdownButtonHideUnderline(
        child: DropdownButton<String>(
          value: postVisibility,
          isDense: true,
          isExpanded: true,
          icon: Icon(
            Icons.arrow_drop_down,
            size: 18,
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
          ),
          items: ['Public', 'Followers', 'Private'].map((String value) {
            IconData icon;
            switch (value) {
              case 'Public':
                icon = Icons.public;
                break;
              case 'Followers':
                icon = Icons.people;
                break;
              case 'Private':
                icon = Icons.lock;
                break;
              default:
                icon = Icons.public;
            }
            return DropdownMenuItem<String>(
              value: value,
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(icon, size: 14),
                  const SizedBox(width: 6),
                  Flexible(
                    child: Text(
                      value,
                      style: const TextStyle(fontSize: 13),
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                ],
              ),
            );
          }).toList(),
          onChanged: (String? newValue) {
            if (newValue != null) {
              onVisibilityChanged(newValue);
            }
          },
        ),
      ),
    );
  }
}
