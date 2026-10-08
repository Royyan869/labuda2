library;

import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Read-only notification settings placeholder.
///
/// This screen is intentionally limited to device-level guidance and a clear
/// under-development message. Backend preference APIs are not called here.
class NotificationSettingsScreen extends StatelessWidget {
  const NotificationSettingsScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    return Scaffold(
      appBar: AppBar(title: const Text('Notification Settings')),
      // SAFE-AREA-35: the body content owns the LIVE system bottom inset —
      // /settings/notifications is a FLAT top-level GoRoute (ProfileModule),
      // so no shell bar owns it; the ListView's EXPLICIT p16 padding also
      // bypasses the framework's automatic list-padding consumption, so
      // nothing else could claim the bottom. Top stays with Scaffold.appBar
      // (the body slot already drops the top padding — no phantom top).
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(AppMetrics.p16),
          children: [
            const _StatusBanner(),
            const SizedBox(height: 16),
            _SectionCard(
              title: 'Current scope',
              children: const [
                _ReadOnlyRow(
                  icon: Icons.phone_android_outlined,
                  title: 'Device notifications',
                  subtitle:
                      'Manage push permissions from your phone or tablet settings.',
                ),
                _ReadOnlyRow(
                  icon: Icons.lock_outline,
                  title: 'Backend preferences',
                  subtitle:
                      'Not available yet in this build, so no save action is shown.',
                ),
                _ReadOnlyRow(
                  icon: Icons.notifications_none,
                  title: 'In-app delivery',
                  subtitle:
                      'Notification list and read-state APIs remain available elsewhere in the app.',
                ),
              ],
            ),
            const SizedBox(height: 16),
            _SectionCard(
              title: 'What changed',
              children: [
                Text(
                  'This screen is intentionally read-only for now. '
                  'It is safe to open from Settings, but it does not submit '
                  'any unsupported preference updates.',
                  style: TextStyle(
                    color: colorScheme.onSurfaceVariant,
                    height: 1.45,
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _StatusBanner extends StatelessWidget {
  const _StatusBanner();

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: colorScheme.surfaceContainerLow,
        borderRadius: BorderRadius.circular(AppShape.r16),
        border: Border.all(
          color: context.statusColors.warning.withValues(alpha: 0.25),
        ),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(Icons.info_outline, color: context.statusColors.warning),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Notification settings are under development',
                  style: context.typeRoles.titleCompact.copyWith(
                    color: colorScheme.onSurface,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const SizedBox(height: 6),
                Text(
                  'You can open this page safely, but preferences are not saved '
                  'from this screen yet.',
                  style: TextStyle(
                    color: colorScheme.onSurfaceVariant,
                    height: 1.45,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _SectionCard extends StatelessWidget {
  final String title;
  final List<Widget> children;

  const _SectionCard({required this.title, required this.children});

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    return Container(
      decoration: BoxDecoration(
        color: colorScheme.surfaceContainerLow,
        borderRadius: BorderRadius.circular(AppShape.r16),
      ),
      child: Padding(
        padding: const EdgeInsets.all(AppMetrics.p16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              title,
              style: context.typeRoles.titleSection.copyWith(
                fontWeight: FontWeight.w600,
                color: colorScheme.onSurface,
              ),
            ),
            const SizedBox(height: 12),
            ...children,
          ],
        ),
      ),
    );
  }
}

class _ReadOnlyRow extends StatelessWidget {
  final IconData icon;
  final String title;
  final String subtitle;

  const _ReadOnlyRow({
    required this.icon,
    required this.title,
    required this.subtitle,
  });

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.only(bottom: AppMetrics.p12),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: AppIconSize.action, color: colorScheme.primary),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: context.typeRoles.titleCompact.copyWith(
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  subtitle,
                  style: context.typeRoles.bodyDense.copyWith(
                    color: colorScheme.onSurfaceVariant,
                    height: 1.4,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
