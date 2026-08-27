import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../domain/entities/account_status.dart';
import '../providers/auth_controller.dart';
import '../providers/auth_state.dart';

/// Screen shown when user's account is suspended or banned.
///
/// ID1D: Prevents normal app navigation. Only actions available:
/// - View restriction reason
/// - Logout
/// - Contact support info
class AccountRestrictedScreen extends ConsumerWidget {
  const AccountRestrictedScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final authState = ref.watch(authControllerProvider);

    // Extract restriction type from auth state
    final AccountStatus restrictionType;
    if (authState is AuthStateAccountRestricted) {
      restrictionType = authState.restrictionType;
    } else {
      // Fallback — should not happen since router gates this screen
      restrictionType = AccountStatus.suspended;
    }

    final isBanned = restrictionType == AccountStatus.banned;
    final theme = Theme.of(context);

    return Scaffold(
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 32),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              const Spacer(flex: 2),

              // Icon
              Icon(
                isBanned ? Icons.block : Icons.pause_circle_outline,
                size: 80,
                color: isBanned ? Colors.red.shade700 : Colors.orange.shade700,
              ),
              const SizedBox(height: 24),

              // Title
              Text(
                isBanned ? 'Akun Diblokir' : 'Akun Ditangguhkan',
                style: theme.textTheme.headlineSmall?.copyWith(
                  fontWeight: FontWeight.bold,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 16),

              // Body message
              Text(
                isBanned
                    ? 'Akun Anda telah diblokir secara permanen karena pelanggaran '
                          'ketentuan layanan. Anda tidak dapat mengakses fitur aplikasi.'
                    : 'Akun Anda sedang ditangguhkan sementara. '
                          'Selama masa penangguhan, Anda tidak dapat mengakses fitur aplikasi.',
                style: theme.textTheme.bodyLarge?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                  height: 1.5,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 24),

              // Support info
              Container(
                padding: const EdgeInsets.all(16),
                decoration: BoxDecoration(
                  color: theme.colorScheme.surfaceContainerHighest,
                  borderRadius: BorderRadius.circular(12),
                ),
                child: Row(
                  children: [
                    Icon(Icons.support_agent, color: theme.colorScheme.primary),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Text(
                        'Jika Anda merasa ini adalah kesalahan, hubungi tim dukungan '
                        'melalui email support.',
                        style: theme.textTheme.bodyMedium,
                      ),
                    ),
                  ],
                ),
              ),

              const Spacer(flex: 3),

              // Logout button
              SizedBox(
                width: double.infinity,
                child: OutlinedButton.icon(
                  onPressed: () {
                    ref.read(authControllerProvider.notifier).signOut();
                  },
                  icon: const Icon(Icons.logout),
                  label: const Text('Keluar'),
                  style: OutlinedButton.styleFrom(
                    padding: const EdgeInsets.symmetric(vertical: 14),
                  ),
                ),
              ),
              const SizedBox(height: 32),
            ],
          ),
        ),
      ),
    );
  }
}
