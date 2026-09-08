library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/dto/promotion_contract_dto.dart';
import 'package:labuda/domains/commerce/pricing/promotion/presentation/providers/canonical_promotion_providers.dart';

class CanonicalPromotionCreateScreen extends ConsumerStatefulWidget {
  const CanonicalPromotionCreateScreen({super.key});

  @override
  ConsumerState<CanonicalPromotionCreateScreen> createState() => _CanonicalPromotionCreateScreenState();
}

class _CanonicalPromotionCreateScreenState extends ConsumerState<CanonicalPromotionCreateScreen> {
  final _formKey = GlobalKey<FormState>();
  String _kind = 'internal';
  final _budgetController = TextEditingController(text: '30000');
  final _durationController = TextEditingController(text: '3');
  final _cityIdsController = TextEditingController();
  bool _isLoading = false;

  @override
  void dispose() {
    _budgetController.dispose();
    _durationController.dispose();
    _cityIdsController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() => _isLoading = true);
    final budget = int.tryParse(_budgetController.text) ?? 0;
    final duration = int.tryParse(_durationController.text) ?? 0;
    final cityIds = _cityIdsController.text
        .split(',')
        .map((e) => e.trim())
        .where((e) => e.isNotEmpty)
        .toList();

    final repo = ref.read(promotionContractRepositoryProvider);
    final result = await repo.createContract(
      kind: _kind,
      budgetRupiah: budget,
      durationDays: duration,
      cityIds: cityIds,
    );
    setState(() => _isLoading = false);
    if (!mounted) return;
    if (result.isSuccess) {
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Promosi berhasil dibuat')));
      ref.invalidate(myPromotionContractsProvider);
      context.pop();
    } else {
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(result.error ?? 'Gagal membuat promosi')));
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Buat Promosi')),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Form(
          key: _formKey,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              DropdownButtonFormField<String>(
                value: _kind,
                decoration: const InputDecoration(labelText: 'Jenis Promosi'),
                items: const [
                  DropdownMenuItem(value: 'internal', child: Text('Internal (For Sale/Auction)')),
                  DropdownMenuItem(value: 'external', child: Text('Eksternal (Event/Business)')),
                ],
                onChanged: (v) => setState(() => _kind = v!),
              ),
              const SizedBox(height: 12),
              TextFormField(
                controller: _budgetController,
                decoration: const InputDecoration(labelText: 'Budget (Rupiah)', hintText: '30000'),
                keyboardType: TextInputType.number,
                validator: (v) => (int.tryParse(v ?? '') ?? 0) <= 0 ? 'Budget harus >0' : null,
              ),
              const SizedBox(height: 12),
              TextFormField(
                controller: _durationController,
                decoration: const InputDecoration(labelText: 'Duration (hari)', hintText: '3'),
                keyboardType: TextInputType.number,
                validator: (v) => (int.tryParse(v ?? '') ?? 0) <= 0 ? 'Duration harus >0' : null,
              ),
              const SizedBox(height: 12),
              TextFormField(
                controller: _cityIdsController,
                decoration: const InputDecoration(
                  labelText: 'City IDs (kosong = nasional)',
                  hintText: '3204,3171,5103',
                ),
              ),
              const SizedBox(height: 8),
              Text(
                'Kosong = nasional (unrestricted). Isi = arbitrary city set, contoh 3204=Bandung, 3171=Jaksel.',
                style: TextStyle(fontSize: 12, color: Colors.grey[600]),
              ),
              const SizedBox(height: 24),
              SizedBox(
                width: double.infinity,
                child: ElevatedButton(
                  onPressed: _isLoading ? null : _submit,
                  child: _isLoading ? const CircularProgressIndicator() : const Text('Buat Promosi'),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
