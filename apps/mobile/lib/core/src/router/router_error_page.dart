import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

class RouterErrorPage extends StatelessWidget {
  final GoRouterState state;

  const RouterErrorPage({super.key, required this.state});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    return Scaffold(
      backgroundColor: scheme.surface,
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(AppMetrics.p24),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              // Error icon
              Container(
                padding: const EdgeInsets.all(AppMetrics.p20),
                decoration: BoxDecoration(
                  color: context.statusColors.error.withValues(alpha: 0.1),
                  shape: BoxShape.circle,
                ),
                child: Icon(
                  Icons.error_outline,
                  size: 64,
                  color: context.statusColors.error,
                ),
              ),
              const SizedBox(height: 24),

              // Title
              Text(
                'Page Not Found',
                style: TextStyle(
                  fontSize: AppType.s24,
                  fontWeight: FontWeight.bold,
                  color: scheme.onSurface,
                ),
              ),
              const SizedBox(height: 12),

              // Error details
              Text(
                'The page "${state.uri.path}" could not be found.',
                textAlign: TextAlign.center,
                style: TextStyle(
                  fontSize: AppType.s16,
                  color: scheme.onSurfaceVariant,
                ),
              ),
              const SizedBox(height: 8),
              // Debug: Show full URI for troubleshooting
              Text(
                'Full URI: ${state.uri}',
                textAlign: TextAlign.center,
                style: TextStyle(
                  fontSize: AppType.s12,
                  color: scheme.onSurfaceVariant,
                ),
              ),
              const SizedBox(height: 32),

              // Actions
              Column(
                children: [
                  SizedBox(
                    width: double.infinity,
                    child: ElevatedButton(
                      onPressed: () => context.go('/splash'),
                      style: ElevatedButton.styleFrom(
                        backgroundColor: scheme.primary,
                        foregroundColor: scheme.onPrimary,
                        padding: const EdgeInsets.symmetric(vertical: AppMetrics.p16),
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(AppShape.r12),
                        ),
                      ),
                      child: const Text(
                        'Go to Home',
                        style: TextStyle(
                          fontSize: AppType.s16,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ),
                  ),
                  const SizedBox(height: 12),
                  SizedBox(
                    width: double.infinity,
                    child: OutlinedButton(
                      onPressed: () {
                        if (context.canPop()) {
                          context.pop();
                        } else {
                          context.go('/splash');
                        }
                      },
                      style: OutlinedButton.styleFrom(
                        side: BorderSide(color: scheme.outline),
                        padding: const EdgeInsets.symmetric(vertical: AppMetrics.p16),
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(AppShape.r12),
                        ),
                      ),
                      child: Text(
                        'Go Back',
                        style: TextStyle(
                          fontSize: AppType.s16,
                          fontWeight: FontWeight.w600,
                          color: scheme.onSurface,
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}
