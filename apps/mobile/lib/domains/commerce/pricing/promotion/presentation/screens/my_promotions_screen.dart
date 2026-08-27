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

class MyPromotionsScreen extends ConsumerWidget {
  const MyPromotionsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final instancesAsync = ref.watch(myInstancesProvider);
    final ownershipsAsync = ref.watch(myOwnershipsProvider);

    return DefaultTabController(
      length: 3,
      child: Scaffold(
        appBar: AppBar(
          title: const Text('My Promotions'),
          bottom: const TabBar(
            tabs: [
              Tab(text: 'Active'),
              Tab(text: 'Paused'),
              Tab(text: 'Completed'),
            ],
          ),
        ),
        body: instancesAsync.when(
          data: (instancesResult) {
            return ownershipsAsync.when(
              data: (ownershipsResult) {
                if (!instancesResult.isSuccess) {
                  return _buildError(
                    instancesResult.error ?? 'Failed to load promotions',
                  );
                }
                if (!ownershipsResult.isSuccess) {
                  return _buildError(
                    ownershipsResult.error ?? 'Failed to load ownerships',
                  );
                }

                final instances = instancesResult.data ?? <PromotionInstance>[];
                final ownerships =
                    ownershipsResult.data ?? <PromotionOwnership>[];
                final ownershipMap = {
                  for (final ownership in ownerships) ownership.id: ownership,
                };
                final activeOwnershipIds = instances
                    .where((i) => i.status == InstanceStatus.active)
                    .map((i) => i.ownershipId)
                    .toSet();
                final usableOwnerships = ownerships
                    .where(
                      (o) =>
                          o.canActivate &&
                          o.remainingDurationHours > 0 &&
                          !activeOwnershipIds.contains(o.id),
                    )
                    .toList();

                final active = instances
                    .where((i) => i.status == InstanceStatus.active)
                    .toList();
                final paused = instances
                    .where((i) => i.status == InstanceStatus.paused)
                    .toList();
                final completed = instances
                    .where(
                      (i) =>
                          i.status == InstanceStatus.cancelled ||
                          i.status == InstanceStatus.expired,
                    )
                    .toList();

                return Column(
                  children: [
                    if (usableOwnerships.isNotEmpty)
                      _UsableOwnershipSection(ownerships: usableOwnerships),
                    Expanded(
                      child: TabBarView(
                        children: [
                          _PromotionList(
                            instances: active,
                            ownershipMap: ownershipMap,
                          ),
                          _PromotionList(
                            instances: paused,
                            ownershipMap: ownershipMap,
                          ),
                          _PromotionList(
                            instances: completed,
                            ownershipMap: ownershipMap,
                          ),
                        ],
                      ),
                    ),
                  ],
                );
              },
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (error, _) => _buildError(error.toString()),
            );
          },
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (error, _) => _buildError(error.toString()),
        ),
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
}

class _PromotionList extends StatelessWidget {
  final List<PromotionInstance> instances;
  final Map<String, PromotionOwnership> ownershipMap;

  const _PromotionList({required this.instances, required this.ownershipMap});

  @override
  Widget build(BuildContext context) {
    if (instances.isEmpty) {
      return const Center(child: Text('No promotions'));
    }

    return ListView.separated(
      padding: const EdgeInsets.all(16),
      itemCount: instances.length,
      separatorBuilder: (context, index) => const SizedBox(height: 12),
      itemBuilder: (context, index) {
        final instance = instances[index];
        final ownership = ownershipMap[instance.ownershipId];
        final expiresAt = ownership?.expiresAt;
        final remainingHours = ownership?.remainingDurationHours ?? 0;

        return InkWell(
          onTap: () =>
              context.push('${RoutePaths.sellerPromotions}/${instance.id}'),
          borderRadius: BorderRadius.circular(12),
          child: Container(
            padding: const EdgeInsets.all(14),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(12),
              border: Border.all(color: AppColors.neutralGray200),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  PromotionUiMapper.mapTargetName(instance),
                  style: const TextStyle(
                    fontWeight: FontWeight.w700,
                    fontSize: 16,
                  ),
                ),
                const SizedBox(height: 8),
                Text('Status: ${PromotionUiMapper.mapStatus(instance.status)}'),
                Text('Remaining: ${_formatHours(remainingHours)}'),
                Text('Expires: ${_formatDate(expiresAt)}'),
              ],
            ),
          ),
        );
      },
    );
  }

  static String _formatDate(DateTime? date) {
    if (date == null) return '-';
    return DateFormat('dd MMM yyyy').format(date);
  }

  static String _formatHours(int hours) => '${hours}h';
}

class _UsableOwnershipSection extends StatelessWidget {
  final List<PromotionOwnership> ownerships;

  const _UsableOwnershipSection({required this.ownerships});

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 8),
      color: AppColors.neutralGray50,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text('Use Now', style: TextStyle(fontWeight: FontWeight.w700)),
          const SizedBox(height: 8),
          ...ownerships.map(
            (ownership) => Container(
              width: double.infinity,
              margin: const EdgeInsets.only(bottom: 8),
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(12),
                border: Border.all(color: AppColors.neutralGray200),
                color: Colors.white,
              ),
              child: Row(
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          ownership.packageId,
                          style: const TextStyle(fontWeight: FontWeight.w600),
                        ),
                        Text('Remaining: ${ownership.remainingDurationHours}h'),
                        Text(
                          'Valid until: ${_formatDate(ownership.expiresAt)}',
                        ),
                      ],
                    ),
                  ),
                  ElevatedButton(
                    onPressed: () {
                      context.push(
                        RoutePaths.sellerPromotionActivate,
                        extra: {'preselectedOwnershipId': ownership.id},
                      );
                    },
                    child: const Text('Use Now'),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  static String _formatDate(DateTime date) =>
      DateFormat('dd MMM yyyy').format(date);
}
