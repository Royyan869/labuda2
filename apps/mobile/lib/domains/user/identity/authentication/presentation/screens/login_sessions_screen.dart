import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/auth_session.dart';
import 'package:labuda/domains/user/profile/data/models/api/user_api_models.dart';
import 'package:labuda/generated/app_localizations.dart';

/// Login Sessions Screen
///
/// Displays active device sessions retrieved from GET /auth/sessions.
/// Allows revoking individual session families and signing out all devices.
///
/// ## State:
/// - loading/error/empty/list driven by explicit StatefulWidget state
/// - revoke and logout-all each show a confirm dialog
/// - after each mutation the list is refreshed
class LoginSessionsScreen extends ConsumerStatefulWidget {
  const LoginSessionsScreen({super.key});

  @override
  ConsumerState<LoginSessionsScreen> createState() =>
      _LoginSessionsScreenState();
}

class _LoginSessionsScreenState extends ConsumerState<LoginSessionsScreen> {
  bool _isLoading = true;
  String? _error;
  List<AuthSessionDto> _sessions = [];
  bool _isMutating = false;

  @override
  void initState() {
    super.initState();
    _loadSessions();
  }

  Future<void> _loadSessions() async {
    setState(() {
      _isLoading = true;
      _error = null;
    });

    final repo = ref.read(authRepositoryProvider);
    final result = await repo.getActiveSessions();

    if (!mounted) return;

    result.fold(
      (msg) => setState(() {
        _error = msg;
        _isLoading = false;
      }),
      (sessions) => setState(() {
        _sessions = sessions;
        _isLoading = false;
      }),
    );
  }

  Future<void> _confirmRevokeSession(
    BuildContext context,
    AuthSessionDto session,
  ) async {
    final l10n = AppLocalizations.of(context)!;
    final confirmed = await _showConfirmDialog(
      context: context,
      title: l10n.revokeSessionTitle,
      message: l10n.revokeSessionMessage,
      confirmLabel: l10n.revokeSession,
      isDestructive: true,
    );
    if (!confirmed) return;
    if (!mounted) return;
    await _revokeSession(session.familyId);
  }

  Future<void> _revokeSession(String familyId) async {
    setState(() => _isMutating = true);

    final repo = ref.read(authRepositoryProvider);
    final result = await repo.revokeSession(familyId);

    if (!mounted) return;

    setState(() => _isMutating = false);

    final l10n = AppLocalizations.of(context)!;
    result.fold((msg) => AppSnackBar.showError(context, msg), (_) {
      AppSnackBar.showSuccess(context, l10n.sessionRevokedSuccess);
      _loadSessions();
    });
  }

  Future<void> _confirmLogoutAll(BuildContext context) async {
    final l10n = AppLocalizations.of(context)!;
    final confirmed = await _showConfirmDialog(
      context: context,
      title: l10n.signOutAllDevicesTitle,
      message: l10n.signOutAllDevicesMessage,
      confirmLabel: l10n.signOutAllDevices,
      isDestructive: true,
    );
    if (!confirmed) return;
    if (!mounted) return;
    await _logoutAll();
  }

  Future<void> _logoutAll() async {
    setState(() => _isMutating = true);
    final controller = ref.read(authControllerProvider.notifier);
    await controller.signOutAll();
    if (!mounted) return;
    setState(() => _isMutating = false);
    AppSnackBar.showSuccess(context, AppLocalizations.of(context)!.allSessionsRevokedSuccess);
    _loadSessions();
  }

  Future<bool> _showConfirmDialog({
    required BuildContext context,
    required String title,
    required String message,
    required String confirmLabel,
    required bool isDestructive,
  }) async {
    final l10n = AppLocalizations.of(context)!;
    final scheme = Theme.of(context).colorScheme;
    final result = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(title),
        content: Text(message),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(l10n.cancel),
          ),
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(
              confirmLabel,
              style: TextStyle(
                color: isDestructive ? scheme.error : scheme.primary,
              ),
            ),
          ),
        ],
      ),
    );
    return result ?? false;
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;

    return Scaffold(
      appBar: AppBarCustom(title: l10n.loginSessions),
      body: Stack(
        children: [
          _buildBody(context, l10n),
          if (_isMutating)
            Positioned.fill(
              child: ColoredBox(
                color: Theme.of(
                  context,
                ).colorScheme.scrim.withValues(alpha: 0.35),
                child: const Center(child: CircularProgressIndicator()),
              ),
            ),
        ],
      ),
    );
  }

  Widget _buildBody(BuildContext context, AppLocalizations l10n) {
    if (_isLoading) {
      return const Center(child: CircularProgressIndicator());
    }

    if (_error != null) {
      return _buildErrorState(context, l10n);
    }

    if (_sessions.isEmpty) {
      return _buildEmptyState(context, l10n);
    }

    return _buildSessionList(context, l10n);
  }

  Widget _buildErrorState(
    BuildContext context,
    AppLocalizations l10n,
  ) {
    final scheme = Theme.of(context).colorScheme;
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.error_outline, size: 48, color: scheme.error),
            const SizedBox(height: 16),
            Text(
              l10n.failedToLoadSessions,
              textAlign: TextAlign.center,
              style: TextStyle(
                color: scheme.onSurface,
              ),
            ),
            const SizedBox(height: 24),
            ElevatedButton.icon(
              onPressed: _loadSessions,
              icon: const Icon(Icons.refresh),
              label: const Text('Coba Lagi'),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildEmptyState(BuildContext context, AppLocalizations l10n) {
    final scheme = Theme.of(context).colorScheme;
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.devices_outlined,
              size: 64,
              color: scheme.outline,
            ),
            const SizedBox(height: 16),
            Text(
              l10n.noActiveSessions,
              textAlign: TextAlign.center,
              style: TextStyle(
                fontSize: 16,
                color: scheme.onSurface,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildSessionList(
    BuildContext context,
    AppLocalizations l10n,
  ) {
    final scheme = Theme.of(context).colorScheme;
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        // Info header
        Text(
          l10n.manageActiveSessions,
          style: TextStyle(
            color: scheme.onSurfaceVariant,
            fontSize: 13,
          ),
        ),
        const SizedBox(height: 12),

        // Session cards
        ..._sessions.map(
          (session) => _SessionCard(
            session: session,
            l10n: l10n,
            onRevoke: () => _confirmRevokeSession(context, session),
          ),
        ),

        const SizedBox(height: 24),

        // Sign out all devices button
        OutlinedButton.icon(
          onPressed: _isMutating ? null : () => _confirmLogoutAll(context),
          icon: const Icon(Icons.logout, size: 18),
          label: Text(l10n.signOutAllDevices),
          style: OutlinedButton.styleFrom(
            foregroundColor: scheme.error,
            side: BorderSide(color: scheme.error.withValues(alpha: 0.5)),
            padding: const EdgeInsets.symmetric(vertical: 12, horizontal: 16),
          ),
        ),
      ],
    );
  }
}

class _SessionCard extends StatelessWidget {
  final AuthSessionDto session;
  final AppLocalizations l10n;
  final VoidCallback onRevoke;

  const _SessionCard({
    required this.session,
    required this.l10n,
    required this.onRevoke,
  });

  AuthSession get _entity => AuthSession(
    familyId: session.familyId,
    deviceId: session.deviceId,
    deviceName: session.deviceName,
    platform: session.platform,
    appVersion: session.appVersion,
    issuedAt: session.issuedAt,
    expiresAt: session.expiresAt,
    lastUsedAt: session.lastUsedAt,
    fcmTokenActive: session.fcmTokenActive,
  );

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final entity = _entity;
    return Container(
      margin: const EdgeInsets.only(bottom: 12),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerLow,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: scheme.outlineVariant,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                _platformIcon(entity.platform),
                size: 20,
                color: scheme.onSurfaceVariant,
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  entity.deviceLabel,
                  style: TextStyle(
                    fontWeight: FontWeight.w600,
                    color: scheme.onSurface,
                  ),
                ),
              ),
              TextButton(
                onPressed: onRevoke,
                style: TextButton.styleFrom(
                  foregroundColor: scheme.error,
                  padding: const EdgeInsets.symmetric(
                    horizontal: 8,
                    vertical: 4,
                  ),
                ),
                child: Text(
                  l10n.revokeSession,
                  style: const TextStyle(fontSize: 12),
                ),
              ),
            ],
          ),
          if (entity.appVersion != null) ...[
            const SizedBox(height: 4),
            Text(
              'v${entity.appVersion}',
              style: TextStyle(
                fontSize: 12,
                color: scheme.onSurfaceVariant,
              ),
            ),
          ],
          const SizedBox(height: 8),
          _buildDateRow(
            context,
            icon: Icons.access_time,
            label: l10n.lastActive,
            date: entity.lastActivity,
          ),
        ],
      ),
    );
  }

  Widget _buildDateRow(
    BuildContext context, {
    required IconData icon,
    required String label,
    required DateTime date,
  }) {
    final formatted = _formatDateTime(date);
    final textColor = Theme.of(context).colorScheme.onSurfaceVariant;
    return Row(
      children: [
        Icon(icon, size: 14, color: textColor),
        const SizedBox(width: 4),
        Text(
          '$label: $formatted',
          style: TextStyle(fontSize: 12, color: textColor),
        ),
      ],
    );
  }

  IconData _platformIcon(String? platform) {
    switch (platform?.toLowerCase()) {
      case 'android':
        return Icons.android;
      case 'ios':
        return Icons.phone_iphone;
      case 'web':
        return Icons.computer;
      default:
        return Icons.devices;
    }
  }

  String _formatDateTime(DateTime dt) {
    final local = dt.toLocal();
    return '${local.day.toString().padLeft(2, '0')}/'
        '${local.month.toString().padLeft(2, '0')}/'
        '${local.year} '
        '${local.hour.toString().padLeft(2, '0')}:'
        '${local.minute.toString().padLeft(2, '0')}';
  }
}
