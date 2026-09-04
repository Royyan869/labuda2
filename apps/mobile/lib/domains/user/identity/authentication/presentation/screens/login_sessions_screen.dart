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
                color: isDestructive ? AppColors.error : AppColors.primaryRed,
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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Scaffold(
      appBar: AppBarCustom(title: l10n.loginSessions),
      body: Stack(
        children: [
          _buildBody(context, l10n, isDark),
          if (_isMutating)
            const Positioned.fill(
              child: ColoredBox(
                color: Color(0x55000000),
                child: Center(child: CircularProgressIndicator()),
              ),
            ),
        ],
      ),
    );
  }

  Widget _buildBody(BuildContext context, AppLocalizations l10n, bool isDark) {
    if (_isLoading) {
      return const Center(child: CircularProgressIndicator());
    }

    if (_error != null) {
      return _buildErrorState(context, l10n, isDark);
    }

    if (_sessions.isEmpty) {
      return _buildEmptyState(l10n, isDark);
    }

    return _buildSessionList(context, l10n, isDark);
  }

  Widget _buildErrorState(
    BuildContext context,
    AppLocalizations l10n,
    bool isDark,
  ) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.error_outline, size: 48, color: AppColors.error),
            const SizedBox(height: 16),
            Text(
              l10n.failedToLoadSessions,
              textAlign: TextAlign.center,
              style: TextStyle(
                color: isDark
                    ? AppColors.neutralGray300
                    : AppColors.neutralGray700,
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

  Widget _buildEmptyState(AppLocalizations l10n, bool isDark) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.devices_outlined,
              size: 64,
              color: isDark
                  ? AppColors.neutralGray500
                  : AppColors.neutralGray400,
            ),
            const SizedBox(height: 16),
            Text(
              l10n.noActiveSessions,
              textAlign: TextAlign.center,
              style: TextStyle(
                fontSize: 16,
                color: isDark
                    ? AppColors.neutralGray300
                    : AppColors.neutralGray700,
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
    bool isDark,
  ) {
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        // Info header
        Text(
          l10n.manageActiveSessions,
          style: TextStyle(
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
            fontSize: 13,
          ),
        ),
        const SizedBox(height: 12),

        // Session cards
        ..._sessions.map(
          (session) => _SessionCard(
            session: session,
            isDark: isDark,
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
            foregroundColor: AppColors.error,
            side: BorderSide(color: AppColors.error.withValues(alpha: 0.5)),
            padding: const EdgeInsets.symmetric(vertical: 12, horizontal: 16),
          ),
        ),
      ],
    );
  }
}

class _SessionCard extends StatelessWidget {
  final AuthSessionDto session;
  final bool isDark;
  final AppLocalizations l10n;
  final VoidCallback onRevoke;

  const _SessionCard({
    required this.session,
    required this.isDark,
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
    final entity = _entity;
    return Container(
      margin: const EdgeInsets.only(bottom: 12),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
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
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray600,
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  entity.deviceLabel,
                  style: TextStyle(
                    fontWeight: FontWeight.w600,
                    color: isDark
                        ? AppColors.neutralGray100
                        : AppColors.neutralGray900,
                  ),
                ),
              ),
              TextButton(
                onPressed: onRevoke,
                style: TextButton.styleFrom(
                  foregroundColor: AppColors.error,
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
                color: isDark
                    ? AppColors.neutralGray500
                    : AppColors.neutralGray600,
              ),
            ),
          ],
          const SizedBox(height: 8),
          _buildDateRow(
            icon: Icons.access_time,
            label: l10n.lastActive,
            date: entity.lastActivity,
            isDark: isDark,
          ),
        ],
      ),
    );
  }

  Widget _buildDateRow({
    required IconData icon,
    required String label,
    required DateTime date,
    required bool isDark,
  }) {
    final formatted = _formatDateTime(date);
    final textColor = isDark
        ? AppColors.neutralGray400
        : AppColors.neutralGray600;
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
