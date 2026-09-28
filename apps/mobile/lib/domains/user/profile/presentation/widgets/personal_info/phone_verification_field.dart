import 'package:labuda/core/core.dart';
import 'package:flutter/material.dart';
import 'package:labuda/shared/shared.dart';

/// Phone verification field widget
class PhoneVerificationField extends StatelessWidget {
  final TextEditingController phoneController;
  final bool phoneVerified;
  final DateTime? phoneVerifiedAt;
  final VoidCallback onVerifyPhone;

  const PhoneVerificationField({
    super.key,
    required this.phoneController,
    required this.phoneVerified,
    this.phoneVerifiedAt,
    required this.onVerifyPhone,
    
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Column(
      children: [
        _buildPhoneInput(context, scheme),
        if (!phoneVerified) ...[
          const SizedBox(height: 12),
          _buildVerifyPrompt(context, scheme),
        ],
      ],
    );
  }

  Widget _buildPhoneInput(BuildContext context, ColorScheme scheme) {
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: scheme.onSurfaceVariant,
        border: Border.all(
          color: scheme.onSurfaceVariant,
        ),
        borderRadius: BorderRadius.circular(AppShape.r8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                Icons.phone_outlined,
                color: scheme.onSurfaceVariant,
                size: 20,
              ),
              const SizedBox(width: 8),
              Text(
                'Phone Number',
                style: TextStyle(
                  fontSize: AppType.s12,
                  color: scheme.onSurfaceVariant,
                ),
              ),
              const Spacer(),
              _buildVerificationBadge(context, scheme),
            ],
          ),
          const SizedBox(height: 12),
          TextField(
            controller: phoneController,
            keyboardType: TextInputType.phone,
            style: TextStyle(
              fontSize: AppType.s14,
              fontWeight: FontWeight.w500,
              color: scheme.onSurface,
            ),
            decoration: InputDecoration(
              hintText: '081234567890',
              hintStyle: TextStyle(
                color: scheme.onSurfaceVariant,
              ),
              contentPadding: const EdgeInsets.symmetric(
                horizontal: AppMetrics.p12,
                vertical: AppMetrics.p10,
              ),
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(AppShape.r8),
                borderSide: BorderSide(
                  color: scheme.onSurfaceVariant,
                ),
              ),
              enabledBorder: OutlineInputBorder(
                borderRadius: BorderRadius.circular(AppShape.r8),
                borderSide: BorderSide(
                  color: scheme.onSurfaceVariant,
                ),
              ),
              focusedBorder: OutlineInputBorder(
                borderRadius: BorderRadius.circular(AppShape.r8),
borderSide: BorderSide(
                   color: scheme.primary,
                   width: 1.5,
                 ),
              ),
            ),
          ),
          if (phoneVerified && phoneVerifiedAt != null) ...[
            const SizedBox(height: 8),
            Text(
              'Verified on ${phoneVerifiedAt!.day}/${phoneVerifiedAt!.month}/${phoneVerifiedAt!.year}',
              style: TextStyle(
                fontSize: AppType.s11,
                color: scheme.onSurfaceVariant,
              ),
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildVerificationBadge(BuildContext context, ColorScheme scheme) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p8, vertical: AppMetrics.p4),
      decoration: BoxDecoration(
        color: phoneVerified
            ? context.statusColors.success.withValues(alpha: 0.1)
            : context.statusColors.warning.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(AppShape.r6),
        border: Border.all(
          color: phoneVerified
              ? context.statusColors.success.withValues(alpha: 0.3)
              : context.statusColors.warning.withValues(alpha: 0.3),
        ),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            phoneVerified ? Icons.verified : Icons.warning,
            color: phoneVerified ? context.statusColors.success : context.statusColors.warning,
            size: 12,
          ),
          const SizedBox(width: 4),
          Text(
            phoneVerified ? 'Verified' : 'Unverified',
            style: TextStyle(
              color: phoneVerified ? context.statusColors.success : context.statusColors.warning,
              fontSize: AppType.s10,
              fontWeight: FontWeight.w600,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildVerifyPrompt(BuildContext context, ColorScheme scheme) {
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: context.statusColors.warning.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(AppShape.r8),
        border: Border.all(color: context.statusColors.warning.withValues(alpha: 0.2)),
      ),
      child: Row(
        children: [
          Icon(Icons.info_outline, color: context.statusColors.warning, size: 16),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              'Please verify your phone number',
              style: TextStyle(
                fontSize: AppType.s11,
                color: scheme.onSurfaceVariant,
              ),
            ),
          ),
          AppButton.text(text: 'Verify', onPressed: onVerifyPhone),
        ],
      ),
    );
  }
}
