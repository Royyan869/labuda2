import 'package:flutter/material.dart';
import 'package:labuda/shared/shared.dart';

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
            padding: const EdgeInsets.all(24),
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
              fontSize: 20,
              fontWeight: FontWeight.bold,
              color: scheme.onSurface,
            ),
          ),
          const SizedBox(height: 8),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 48),
            child: Text(
              'Add your shipping address to start shopping',
              textAlign: TextAlign.center,
              style: TextStyle(
                fontSize: 14,
                color: scheme.onSurfaceVariant,
              ),
            ),
          ),
          const SizedBox(height: 32),
          SizedBox(
            width: 200,
            child: AppButton.primary(
              text: 'Add Address',
              onPressed: onAddAddress,
            ),
          ),
        ],
      ),
    );
  }
}
