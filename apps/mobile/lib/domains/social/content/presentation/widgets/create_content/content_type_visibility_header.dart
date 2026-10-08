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
    final scheme = Theme.of(context).colorScheme;
    final user = authenticatedUser;

    return Container(
      padding: const EdgeInsets.fromLTRB(
        AppMetrics.p16,
        AppMetrics.p8,
        AppMetrics.p16,
        AppMetrics.p8,
      ),
      child: Row(
        children: [
          if (user != null)
            ProfileAvatar(userId: user.id, size: 40, imageUrl: user.avatarUrl)
          else
            Container(
              width: AppIconSize.action * 2,
              height: AppIconSize.action * 2,
              decoration: BoxDecoration(
                color: scheme.surfaceContainerHighest,
                shape: BoxShape.circle,
              ),
              child: Icon(
                Icons.person_outline,
                size: AppIconSize.action,
                color: scheme.onSurfaceVariant,
              ),
            ),
          const SizedBox(width: 12),
          Expanded(
            child: user != null && user.username.isNotEmpty
                ? Text(
                    '@${user.username}',
                    style: context.typeRoles.titleCompact.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  )
                : Container(
                    height: 14,
                    decoration: BoxDecoration(
                      color: scheme.surfaceContainerHighest,
                      borderRadius: BorderRadius.circular(AppShape.pill),
                    ),
                  ),
          ),
          const SizedBox(width: 8),
          _buildVisibilityDropdown(context),
        ],
      ),
    );
  }

  Widget _buildVisibilityDropdown(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      width: AppContentSize.panel,
      height: AppContentSize.controlCompact,
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p8),
      decoration: BoxDecoration(
        border: Border.all(color: scheme.outlineVariant),
        borderRadius: BorderRadius.circular(AppShape.r6),
      ),
      child: DropdownButtonHideUnderline(
        child: DropdownButton<String>(
          value: postVisibility,
          isDense: true,
          isExpanded: true,
          icon: Icon(
            Icons.arrow_drop_down,
            size: AppIconSize.action,
            color: scheme.onSurfaceVariant,
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
                  Icon(icon, size: AppIconSize.inlineGlyph),
                  const SizedBox(width: 6),
                  Flexible(
                    child: Text(
                      value,
                      style: Theme.of(context).textTheme.labelLarge,
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
