library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/instance_status.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/promotion_instance.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/promotion_ownership.dart';
import 'package:labuda/domains/commerce/pricing/promotion/presentation/mappers/promotion_ui_mapper.dart';
import 'package:labuda/domains/commerce/pricing/promotion/presentation/providers/promotion_providers.dart';

class PromotionDetailScreen extends ConsumerStatefulWidget {
  final String instanceId;

  const PromotionDetailScreen({super.key, required this.instanceId});

  @override
  ConsumerState<PromotionDetailScreen> createState() =>
      _PromotionDetailScreenState();
}

class _PromotionDetailScreenState extends ConsumerState<PromotionDetailScreen> {
  bool _isSubmitting = false;

  @override
  Widget build(BuildContext context) {
    final instancesAsync = ref.watch(myInstancesProvider);
    final ownershipsAsync = ref.watch(myOwnershipsProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('Promotion Detail')),
      body: instancesAsync.when(
        data: (instancesResult) {
          return ownershipsAsync.when(
            data: (ownershipsResult) {
              if (!instancesResult.isSuccess || !ownershipsResult.isSuccess) {
                return _buildError('Failed to load promotion detail');
              }

              final instances = instancesResult.data ?? <PromotionInstance>[];
              final ownerships =
                  ownershipsResult.data ?? <PromotionOwnership>[];
              final ownershipMap = {for (final o in ownerships) o.id: o};
              PromotionInstance? instance;
              for (final item in instances) {
                if (item.id == widget.instanceId) {
                  instance = item;
                  break;
                }
              }
              if (instance == null) {
                return _buildError('Promotion not found');
              }

              final ownership = ownershipMap[instance.ownershipId];
              return _buildContent(context, instance, ownership, instances);
            },
            loading: () => const Center(child: CircularProgressIndicator()),
            error: (error, _) => _buildError(error.toString()),
          );
        },
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => _buildError(error.toString()),
      ),
    );
  }

  Widget _buildError(String message) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Text(message, textAlign: TextAlign.center),
      ),
    );
  }

  Widget _buildContent(
    BuildContext context,
    PromotionInstance instance,
    PromotionOwnership? ownership,
    List<PromotionInstance> allInstances,
  ) {
    final status = instance.status;
    final isActive = status == InstanceStatus.active;
    final isPaused = status == InstanceStatus.paused;
    final isCompleted =
        status == InstanceStatus.cancelled || status == InstanceStatus.expired;
    final hasAnyActiveOnOwnership = allInstances.any(
      (i) =>
          i.ownershipId == instance.ownershipId &&
          i.status == InstanceStatus.active,
    );
    final canUseNow =
        ownership != null &&
        ownership.canActivate &&
        ownership.remainingDurationHours > 0 &&
        !hasAnyActiveOnOwnership;

    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        _SectionCard(
          title: 'Ownership',
          children: [
            _kv('Package Name', ownership?.packageId ?? '-'),
            _kv('Total Duration', _hours(ownership?.totalDurationHours)),
            _kv('Consumed Duration', _hours(ownership?.consumedDurationHours)),
            _kv(
              'Remaining Duration',
              _hours(ownership?.remainingDurationHours),
            ),
            _kv('Validity Expiry', _date(ownership?.expiresAt)),
          ],
        ),
        const SizedBox(height: 12),
        _SectionCard(
          title: 'Instance',
          children: [
            _kv('Target Name', PromotionUiMapper.mapTargetName(instance)),
            _kv('Status', PromotionUiMapper.mapStatus(status)),
            _kv('Activated At', _dateTime(instance.activatedAt)),
            _kv(
              'Stop Reason',
              PromotionUiMapper.mapReason(instance.stopReason),
            ),
          ],
        ),
        const SizedBox(height: 18),
        if (!isCompleted) ...[
          if (isActive)
            _actionButton(
              context,
              label: 'Pause',
              color: Colors.orange,
              onPressed: () => _pause(instance.id),
            ),
          if (isPaused)
            _actionButton(
              context,
              label: 'Resume',
              color: AppColors.successGreen,
              onPressed: () => _resume(instance.id),
            ),
          const SizedBox(height: 10),
          _actionButton(
            context,
            label: 'Cancel',
            color: AppColors.primaryRed,
            onPressed: () => _cancel(instance.id),
          ),
          const SizedBox(height: 10),
          OutlinedButton(
            onPressed: _isSubmitting
                ? null
                : () {
                    context.push(
                      RoutePaths.sellerPromotionActivate,
                      extra: {
                        'preselectedOwnershipId': instance.ownershipId,
                        'reassignInstanceId': instance.id,
                      },
                    );
                  },
            child: const Text('Reassign'),
          ),
        ],
        if (canUseNow) ...[
          const SizedBox(height: 10),
          ElevatedButton(
            onPressed: _isSubmitting
                ? null
                : () {
                    context.push(
                      RoutePaths.sellerPromotionActivate,
                      extra: {'preselectedOwnershipId': ownership.id},
                    );
                  },
            child: const Text('Use Now'),
          ),
        ],
      ],
    );
  }

  Widget _actionButton(
    BuildContext context, {
    required String label,
    required Color color,
    required VoidCallback onPressed,
  }) {
    return SizedBox(
      width: double.infinity,
      child: ElevatedButton(
        onPressed: _isSubmitting ? null : onPressed,
        style: ElevatedButton.styleFrom(
          backgroundColor: color,
          foregroundColor: Colors.white,
        ),
        child: _isSubmitting ? const CircularProgressIndicator() : Text(label),
      ),
    );
  }

  Future<void> _pause(String instanceId) async {
    setState(() => _isSubmitting = true);
    final result = await ref
        .read(promotionControllerProvider)
        .pausePromotion(instanceId);
    _finishMutation(
      result.isSuccess,
      result.error ?? 'Gagal dijeda. Coba lagi.',
    );
  }

  Future<void> _resume(String instanceId) async {
    setState(() => _isSubmitting = true);
    final result = await ref
        .read(promotionControllerProvider)
        .resumePromotion(instanceId);
    _finishMutation(
      result.isSuccess,
      result.error ?? 'Gagal dilanjutkan. Coba lagi.',
    );
  }

  Future<void> _cancel(String instanceId) async {
    setState(() => _isSubmitting = true);
    final result = await ref
        .read(promotionControllerProvider)
        .cancelPromotion(instanceId);
    _finishMutation(
      result.isSuccess,
      result.error ?? 'Gagal dibatalkan. Coba lagi.',
    );
  }

  void _finishMutation(bool success, String errorText) {
    if (!mounted) return;
    setState(() => _isSubmitting = false);
    if (success) {
      ref.invalidate(myInstancesProvider);
      ref.invalidate(myOwnershipsProvider);
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('Success')));
      return;
    }
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(errorText), backgroundColor: AppColors.primaryRed),
    );
  }

  static Widget _kv(String key, String value) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        children: [
          Expanded(flex: 2, child: Text(key)),
          Expanded(
            flex: 3,
            child: Text(
              value,
              style: const TextStyle(fontWeight: FontWeight.w600),
            ),
          ),
        ],
      ),
    );
  }

  static String _hours(int? h) => h == null ? '-' : '${h}h';

  static String _date(DateTime? d) =>
      d == null ? '-' : DateFormat('dd MMM yyyy').format(d);

  static String _dateTime(DateTime? d) =>
      d == null ? '-' : DateFormat('dd MMM yyyy, HH:mm').format(d);
}

class _SectionCard extends StatelessWidget {
  final String title;
  final List<Widget> children;

  const _SectionCard({required this.title, required this.children});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: AppColors.neutralGray200),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w700),
          ),
          const SizedBox(height: 10),
          ...children,
        ],
      ),
    );
  }
}
