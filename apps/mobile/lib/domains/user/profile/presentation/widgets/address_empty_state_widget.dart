import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Empty State Widget untuk Address List
class AddressEmptyStateWidget extends StatelessWidget {
  final VoidCallback onAddAddress;

  const AddressEmptyStateWidget({
    super.key,
    required this.onAddAddress,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Container(
            padding: const EdgeInsets.all(AppMetrics.p24),
            decoration: BoxDecoration(
              color: scheme.primary.withValues(alpha: 0.1),
              shape: BoxShape.circle,
            ),
            child: Icon(
              Icons.location_on_outlined,
              size: 64,
              color: scheme.primary,
            ),
          ),
          const SizedBox(height: 24),
          Text(
            'No Address Yet',
            style: TextStyle(
              fontSize: AppType.s20,
              fontWeight: FontWeight.bold,
              color: scheme.onSurface,
            ),
          ),
          const SizedBox(height: 8),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p48),
            child: Text(
              'Add your shipping address to start shopping',
              textAlign: TextAlign.center,
              style: TextStyle(
                fontSize: AppType.s14,
                color: scheme.onSurfaceVariant,
              ),
            ),
          ),
          const SizedBox(height: 32),
          SizedBox(
            width: 200,
            child: ElevatedButton(
              onPressed: onAddAddress,
              child: const Text('Add Address'),
            ),
          ),
        ],
      ),
    );
  }
}
