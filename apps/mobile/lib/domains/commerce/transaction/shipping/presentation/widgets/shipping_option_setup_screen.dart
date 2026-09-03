library;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart';
import 'package:labuda/shared/models/wilayah_models.dart';
import 'package:labuda/shared/providers/wilayah_provider_simple.dart';
import 'package:labuda/shared/utils/app_formatters.dart';
import 'package:labuda/shared/widgets/wilayah/city_dropdown.dart';

class ShippingSetupScreen extends ConsumerStatefulWidget {
  /// When non-null the screen operates in edit mode: form fields are
  /// pre-filled from this option and coverages are shown read-only.
  final ShippingSetup? editOption;

  /// When non-null the screen fetches the canonical detail by this ID
  /// and then hydrates the form. Takes precedence over [editOption].
  final String? editOptionId;

  const ShippingSetupScreen({super.key, this.editOption, this.editOptionId});

  /// Opens the setup page in create mode.
  static Future<ShippingSetup?> open(BuildContext context) {
    return context.push<ShippingSetup>(RoutePaths.sellerShippingSetup);
  }

  /// Opens the setup page in edit mode for the given option.
  /// Prefer [openEditById] to ensure the editor is hydrated from the
  /// canonical detail endpoint.
  static Future<ShippingSetup?> openEdit(
    BuildContext context,
    ShippingSetup option,
  ) {
    return context.push<ShippingSetup>(
      RoutePaths.sellerShippingSetup,
      extra: option,
    );
  }

  /// Opens the setup page in edit mode by fetching the canonical detail
  /// for [optionId]. This is the recommended path — it guarantees the
  /// form is hydrated with full coverages and city rules.
  static Future<ShippingSetup?> openEditById(
    BuildContext context,
    String optionId,
  ) {
    return context.push<ShippingSetup>(
      RoutePaths.sellerShippingSetup,
      extra: optionId,
    );
  }

  bool get isEditMode => editOption != null || editOptionId != null;

  @override
  ConsumerState<ShippingSetupScreen> createState() =>
      _ShippingSetupScreenState();
}

class ShippingCityRulesRouteArgs {
  final Province province;
  final int provinceTariff;
  final List<ShippingCityRuleDraft> currentRules;

  const ShippingCityRulesRouteArgs({
    required this.province,
    required this.provinceTariff,
    required this.currentRules,
  });
}

class ShippingCityRulesScreen extends ConsumerStatefulWidget {
  final ShippingCityRulesRouteArgs args;

  const ShippingCityRulesScreen({super.key, required this.args});

  static Future<List<ShippingCityRuleDraft>?> open(
    BuildContext context,
    ShippingCityRulesRouteArgs args,
  ) {
    return context.push<List<ShippingCityRuleDraft>>(
      RoutePaths.sellerShippingSetupCityRules,
      extra: args,
    );
  }

  @override
  ConsumerState<ShippingCityRulesScreen> createState() =>
      _ShippingCityRulesScreenState();
}

class _ShippingSetupScreenState
    extends ConsumerState<ShippingSetupScreen> {
  final _nameController = TextEditingController();
  final _internalNoteController = TextEditingController();
  final List<_CoverageDraft> _coverages = [_CoverageDraft()];

  ShippingType? _type;
  bool _isSubmitting = false;
  bool _hasUnsavedChanges = false;
  String? _errorMessage;

  // ID-based detail fetching state
  bool _detailLoading = false;
  String? _detailError;

  bool get _isEditMode => widget.isEditMode;

  @override
  void initState() {
    super.initState();
    final option = widget.editOption;
    if (option != null) {
      _hydrateFromOption(option);
    } else if (widget.editOptionId != null) {
      _detailLoading = true;
      WidgetsBinding.instance.addPostFrameCallback((_) => _fetchDetail());
    }
  }

  void _hydrateFromOption(ShippingSetup option) {
    _type = option.type;
    _nameController.text = option.name;
    _internalNoteController.text = option.internalNote ?? '';
    _coverages.clear();
    if (option.coverageAreas.isEmpty) {
      _coverages.add(_CoverageDraft());
    } else {
      for (final cov in option.coverageAreas) {
        final draft = _CoverageDraft()
          ..province = Province(id: cov.provinceId, name: cov.provinceName);
        draft.tariffController.text =
            (cov.provinceRate ?? 0).toInt().toString();
        draft.cityRules.addAll(
          cov.cityOverrides.map(
            (city) => ShippingCityRuleDraft(
              cityId: city.cityId,
              cityName: city.cityName,
              overrideTariff: city.rate > 0 ? city.rate.toInt() : null,
              excluded: city.excluded == true,
            ),
          ),
        );
        _coverages.add(draft);
      }
    }
  }

  Future<void> _fetchDetail() async {
    final optionId = widget.editOptionId!;
    final result = await ref
        .read(shippingRepositoryProvider)
        .getShippingSetupById(optionId);

    if (!mounted) return;

    if (result.isSuccess && result.data != null) {
      setState(() {
        _detailLoading = false;
        _detailError = null;
        _hydrateFromOption(result.data!);
      });
    } else {
      setState(() {
        _detailLoading = false;
        _detailError = 'Gagal memuat detail opsi pengiriman. Coba lagi.';
      });
    }
  }

  @override
  void dispose() {
    _nameController.dispose();
    _internalNoteController.dispose();
    for (final coverage in _coverages) {
      coverage.dispose();
    }
    super.dispose();
  }

  void _markDirty() {
    if (!_hasUnsavedChanges) {
      setState(() => _hasUnsavedChanges = true);
    }
  }

  void _addCoverage() {
    setState(() {
      _coverages.add(_CoverageDraft());
      _errorMessage = null;
      _hasUnsavedChanges = true;
    });
  }

  void _removeCoverage(int index) {
    if (_coverages.length == 1) return;
    setState(() {
      _coverages.removeAt(index).dispose();
      _errorMessage = null;
      _hasUnsavedChanges = true;
    });
  }

  void _onProvinceChanged(int coverageIndex, Province? province) {
    if (province == null) return;
    final duplicate = _coverages.asMap().entries.any(
      (entry) =>
          entry.key != coverageIndex && entry.value.province?.id == province.id,
    );
    if (duplicate) {
      setState(() {
        _errorMessage = 'Provinsi ${province.name} sudah dipilih.';
      });
      return;
    }

    setState(() {
      _coverages[coverageIndex].province = province;
      _errorMessage = null;
      _hasUnsavedChanges = true;
    });
  }

  Future<void> _openCityRules(int coverageIndex) async {
    final coverage = _coverages[coverageIndex];
    final province = coverage.province;
    if (province == null) {
      setState(() {
        _errorMessage =
            'Pilih provinsi terlebih dahulu sebelum mengatur kota/kabupaten.';
      });
      return;
    }

    final tariff = int.tryParse(coverage.tariffController.text.trim());
    if (tariff == null || tariff <= 0) {
      setState(() {
        _errorMessage =
            'Masukkan tarif provinsi yang valid sebelum mengatur kota/kabupaten.';
      });
      return;
    }

    final updatedRules = await ShippingCityRulesScreen.open(
      context,
      ShippingCityRulesRouteArgs(
        province: province,
        provinceTariff: tariff,
        currentRules: coverage.cityRules,
      ),
    );
    if (!mounted || updatedRules == null) return;

    setState(() {
      coverage.cityRules
        ..clear()
        ..addAll(updatedRules);
      _errorMessage = null;
      _hasUnsavedChanges = true;
    });
  }

  void _removeCityRule(int coverageIndex, int ruleIndex) {
    setState(() {
      _coverages[coverageIndex].cityRules.removeAt(ruleIndex);
      _errorMessage = null;
      _hasUnsavedChanges = true;
    });
  }

  Future<bool> _confirmDiscardChanges() async {
    if (!_hasUnsavedChanges) return true;
    if (_isSubmitting) return false;

    final shouldDiscard = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('Buang perubahan?'),
        content: const Text(
          'Ada perubahan yang belum disimpan. Jika kembali, semua draft pada halaman ini akan dibuang.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(false),
            child: const Text('Tetap di sini'),
          ),
          ElevatedButton(
            onPressed: () => Navigator.of(dialogContext).pop(true),
            style: ElevatedButton.styleFrom(
              backgroundColor: AppColors.error,
              foregroundColor: Colors.white,
            ),
            child: const Text('Buang'),
          ),
        ],
      ),
    );
    return shouldDiscard == true;
  }

  Future<void> _handleBack() async {
    if (await _confirmDiscardChanges() && mounted) {
      Navigator.of(context).pop();
    }
  }

  String? _validate() {
    if (_type == null) {
      return 'Pilih jenis transportasi.';
    }

    final name = _nameController.text.trim();
    if (name.isEmpty) {
      return 'Nama opsi wajib diisi.';
    }

    if (_coverages.isEmpty) {
      return 'Tambahkan minimal satu provinsi.';
    }

    final seenProvinces = <String>{};
    for (var i = 0; i < _coverages.length; i++) {
      final coverage = _coverages[i];
      final province = coverage.province;
      if (province == null) {
        return 'Pilih provinsi untuk coverage ke-${i + 1}.';
      }
      if (!seenProvinces.add(province.id)) {
        return 'Provinsi ${province.name} sudah dipilih.';
      }

      final tariff = int.tryParse(coverage.tariffController.text.trim());
      if (tariff == null || tariff <= 0) {
        return 'Tarif untuk ${province.name} harus lebih dari 0.';
      }

      final seenCities = <String>{};
      for (final rule in coverage.cityRules) {
        if (!seenCities.add(rule.cityId)) {
          return 'Kota ${rule.cityName} sudah ditambahkan untuk ${province.name}.';
        }
        if (!rule.cityId.startsWith(province.id)) {
          return 'Kota ${rule.cityName} tidak cocok dengan provinsi ${province.name}.';
        }
        if (rule.excluded && rule.overrideTariff != null) {
          return 'Kota ${rule.cityName} tidak boleh sekaligus dikecualikan dan diberi tarif override.';
        }
        if (!rule.excluded && rule.overrideTariff == null) {
          return 'Kota ${rule.cityName} perlu tarif override atau harus dikecualikan.';
        }
        if (rule.overrideTariff != null && rule.overrideTariff! <= 0) {
          return 'Tarif override untuk ${rule.cityName} harus lebih dari 0.';
        }
      }
    }

    return null;
  }

  Future<void> _submit() async {
    final validationError = _validate();
    if (validationError != null) {
      setState(() => _errorMessage = validationError);
      return;
    }

    setState(() {
      _isSubmitting = true;
      _errorMessage = null;
    });

    final repo = ref.read(shippingRepositoryProvider);

    if (_isEditMode) {
      final fullRequest = UpdateShippingSetupFullRequest(
        name: _nameController.text.trim(),
        transportType: _type!,
        internalNote: _internalNoteController.text.trim().isEmpty
            ? null
            : _internalNoteController.text.trim(),
        coverages: _coverages
            .map((coverage) {
              final province = coverage.province!;
              final tariff =
                  int.parse(coverage.tariffController.text.trim());
              return UpdateShippingCoverageRequest(
                provinceId: province.id,
                provinceName: province.name,
                tariff: tariff,
                cityRules: coverage.cityRules
                    .map(
                      (rule) => CreateShippingCityRuleRequest(
                        cityId: rule.cityId,
                        cityName: rule.cityName,
                        overrideTariff: rule.overrideTariff,
                        excluded: rule.excluded,
                      ),
                    )
                    .toList(growable: false),
              );
            })
            .toList(growable: false),
      );

      final result = await ref
          .read(shippingRepositoryProvider)
          .updateShippingSetupFull(
            widget.editOption!.id,
            fullRequest,
          );

      if (!mounted) return;

      if (result.isSuccess && result.data != null) {
        Navigator.of(context).pop(result.data);
        return;
      }
      setState(() {
        _isSubmitting = false;
        _errorMessage = _mapError(result.error);
      });
      return;
    }

    // Create mode — full atomic request with coverages
    final request = CreateShippingSetupRequest(
      name: _nameController.text.trim(),
      type: _type!,
      internalNote: _internalNoteController.text.trim().isEmpty
          ? null
          : _internalNoteController.text.trim(),
      coverages: _coverages
          .map((coverage) {
            final province = coverage.province!;
            final tariff = int.parse(coverage.tariffController.text.trim());
            return CreateShippingCoverageRequest(
              provinceId: province.id,
              provinceName: province.name,
              tariff: tariff,
              cityRules: coverage.cityRules
                  .map(
                    (rule) => CreateShippingCityRuleRequest(
                      cityId: rule.cityId,
                      cityName: rule.cityName,
                      overrideTariff: rule.overrideTariff,
                      excluded: rule.excluded,
                    ),
                  )
                  .toList(growable: false),
            );
          })
          .toList(growable: false),
    );

    final result = await repo.createShippingSetup(request);

    if (!mounted) return;

    if (result.isSuccess && result.data != null) {
      Navigator.of(context).pop(result.data);
      return;
    }

    setState(() {
      _isSubmitting = false;
      _errorMessage = _mapError(result.error);
    });
  }

  String _mapError(String? error) {
    if (error == null || error.isEmpty) {
      return 'Gagal membuat opsi pengiriman.';
    }
    // Go/JSON serialization errors
    if (error.contains('json:') ||
        error.contains('unmarshal') ||
        error.contains('struct field') ||
        error.contains('int64') ||
        error.contains('cannot unmarshal')) {
      return 'Format tarif tidak valid. Periksa kembali tarif provinsi dan kota.';
    }
    // Raw SQL/persistence errors — must never reach production UI
    if (error.contains('SQLSTATE') ||
        error.contains('null value in column') ||
        error.contains('violates not-null constraint') ||
        (error.contains('relation') && error.contains('does not exist')) ||
        error.contains('create city override failed') ||
        error.contains('failed to create city rule')) {
      return 'Gagal menyimpan aturan kota/kabupaten. Coba lagi.';
    }
    return error;
  }

  @override
  Widget build(BuildContext context) {
    // ID-based detail fetch loading / error states
    if (_detailLoading) {
      return Scaffold(
        appBar: AppBar(
          title: const Text('Edit Opsi Pengiriman'),
          backgroundColor: AppColors.primaryRed,
          foregroundColor: Colors.white,
        ),
        body: const Center(child: CircularProgressIndicator()),
      );
    }

    if (_detailError != null) {
      return Scaffold(
        appBar: AppBar(
          title: const Text('Edit Opsi Pengiriman'),
          backgroundColor: AppColors.primaryRed,
          foregroundColor: Colors.white,
          leading: IconButton(
            icon: const Icon(Icons.arrow_back),
            onPressed: () => Navigator.of(context).pop(),
          ),
        ),
        body: ListView(
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.all(24),
          children: [
            const SizedBox(height: 80),
            Icon(Icons.error_outline, size: 64, color: AppColors.error),
            const SizedBox(height: 16),
            const Text(
              'Gagal memuat detail opsi pengiriman',
              textAlign: TextAlign.center,
              style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 8),
            Text(
              _detailError!,
              textAlign: TextAlign.center,
              style: TextStyle(fontSize: 13, color: AppColors.neutralGray600),
            ),
            const SizedBox(height: 24),
            ElevatedButton(
              onPressed: () {
                setState(() {
                  _detailLoading = true;
                  _detailError = null;
                });
                _fetchDetail();
              },
              child: const Text('Coba Lagi'),
            ),
          ],
        ),
      );
    }

    final provincesAsync = ref.watch(provincesProvider);

    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) async {
        if (didPop || _isSubmitting) return;
        final navigator = Navigator.of(context);
        if (await _confirmDiscardChanges() && mounted) {
          navigator.pop();
        }
      },
      child: Scaffold(
        appBar: AppBar(
          title: Text(_isEditMode ? 'Edit Opsi Pengiriman' : 'Setup Opsi Pengiriman'),
          backgroundColor: AppColors.primaryRed,
          foregroundColor: Colors.white,
          leading: IconButton(
            icon: const Icon(Icons.arrow_back),
            onPressed: _handleBack,
          ),
        ),
        bottomNavigationBar: provincesAsync.maybeWhen(
          data: (_) => _buildActionBar(),
          orElse: () => null,
        ),
        body: provincesAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (error, stack) => ListView(
            physics: const AlwaysScrollableScrollPhysics(),
            padding: const EdgeInsets.all(24),
            children: [
              const SizedBox(height: 80),
              Icon(Icons.error_outline, size: 64, color: AppColors.error),
              const SizedBox(height: 16),
              const Text(
                'Gagal memuat provinsi',
                textAlign: TextAlign.center,
                style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
              ),
              const SizedBox(height: 8),
              Text(
                '$error',
                textAlign: TextAlign.center,
                style: TextStyle(fontSize: 13, color: AppColors.neutralGray600),
              ),
              const SizedBox(height: 24),
              ElevatedButton(
                onPressed: () => setState(() {}),
                child: const Text('Coba Lagi'),
              ),
            ],
          ),
          data: (provinces) => SingleChildScrollView(
            keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
            padding: const EdgeInsets.fromLTRB(16, 16, 16, 96),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text(
                  _isEditMode
                      ? 'Edit lengkap opsi pengiriman termasuk cakupan, tarif, dan aturan kota.'
                      : 'Lengkapi satu opsi pengiriman, lalu simpan seluruh coverage sekaligus.',
                  style: const TextStyle(fontSize: 13),
                ),
                const SizedBox(height: 16),
                const Text(
                  'Jenis transportasi',
                  style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600),
                ),
                const SizedBox(height: 8),
                Wrap(
                  spacing: 8,
                  runSpacing: 8,
                  children: ShippingType.values
                      .map(
                        (type) => ChoiceChip(
                          label: Text('${type.emoji} ${type.label}'),
                          selected: _type == type,
                          onSelected: (_) {
                            setState(() {
                              _type = type;
                              _errorMessage = null;
                              _hasUnsavedChanges = true;
                            });
                          },
                        ),
                      )
                      .toList(growable: false),
                ),
                const SizedBox(height: 16),
                TextField(
                  controller: _nameController,
                  onChanged: (_) {
                    _markDirty();
                    setState(() => _errorMessage = null);
                  },
                  decoration: const InputDecoration(
                    labelText: 'Nama ekspedisi / layanan *',
                    hintText: 'Contoh: Bus Kencana',
                    border: OutlineInputBorder(),
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _internalNoteController,
                  onChanged: (_) {
                    _markDirty();
                    setState(() => _errorMessage = null);
                  },
                  decoration: const InputDecoration(
                    labelText: 'Catatan internal (opsional)',
                    hintText: 'Contoh: box besar',
                    border: OutlineInputBorder(),
                  ),
                ),
                const SizedBox(height: 20),
                const Text(
                  'Cakupan dan tarif',
                  style: TextStyle(fontSize: 16, fontWeight: FontWeight.w700),
                ),
                const SizedBox(height: 8),
                ...List.generate(_coverages.length, (index) {
                  return Padding(
                    padding: const EdgeInsets.only(bottom: 12),
                    child: _CoverageCard(
                      coverage: _coverages[index],
                      provinces: provinces,
                      onProvinceChanged: (province) =>
                          _onProvinceChanged(index, province),
                      onRemove: _coverages.length == 1
                          ? null
                          : () => _removeCoverage(index),
                      onOpenCityRules: () => _openCityRules(index),
                      onRemoveCityRule: (ruleIndex) =>
                          _removeCityRule(index, ruleIndex),
                      onTariffChanged: () {
                        _markDirty();
                        setState(() => _errorMessage = null);
                      },
                    ),
                  );
                }),
                if (_errorMessage != null) ...[
                  const SizedBox(height: 8),
                  Text(
                    _errorMessage!,
                    style: const TextStyle(
                      color: AppColors.error,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  const SizedBox(height: 12),
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildActionBar() {
    return SafeArea(
      minimum: const EdgeInsets.fromLTRB(16, 8, 16, 16),
      child: Row(
        children: [
          Expanded(
            child: OutlinedButton.icon(
              onPressed: _isSubmitting ? null : _addCoverage,
              icon: const Icon(Icons.add),
              label: const Text('Tambah Provinsi'),
              style: OutlinedButton.styleFrom(
                minimumSize: const Size.fromHeight(50),
              ),
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: ElevatedButton(
              onPressed: _isSubmitting ? null : _submit,
              style: ElevatedButton.styleFrom(
                backgroundColor: AppColors.primaryRed,
                foregroundColor: Colors.white,
                minimumSize: const Size.fromHeight(50),
              ),
              child: _isSubmitting
                  ? const SizedBox(
                      height: 20,
                      width: 20,
                      child: CircularProgressIndicator(
                        strokeWidth: 2,
                        valueColor: AlwaysStoppedAnimation<Color>(Colors.white),
                      ),
                    )
                  : const Text('Simpan'),
            ),
          ),
        ],
      ),
    );
  }
}

class _ShippingCityRulesScreenState
    extends ConsumerState<ShippingCityRulesScreen> {
  late final List<ShippingCityRuleDraft> _rules;

  @override
  void initState() {
    super.initState();
    _rules = widget.args.currentRules.map((rule) => rule.copy()).toList();
  }

  Future<void> _openRuleEditor({int? existingRuleIndex}) async {
    final existingRule = existingRuleIndex == null
        ? null
        : _rules[existingRuleIndex];
    final result = await showDialog<ShippingCityRuleDraft>(
      context: context,
      builder: (_) => _CityRuleEditorDialog(
        province: widget.args.province,
        provinceTariff: widget.args.provinceTariff,
        existingRules: _rules,
        initialRule: existingRule,
      ),
    );
    if (result == null || !mounted) return;

    setState(() {
      if (existingRuleIndex != null) {
        _rules[existingRuleIndex] = result;
      } else {
        _rules.add(result);
      }
    });
  }

  void _removeRule(int index) {
    setState(() {
      _rules.removeAt(index);
    });
  }

  void _save() {
    Navigator.of(context).pop(_rules.map((rule) => rule.copy()).toList());
  }

  String _summaryText() {
    if (_rules.isEmpty) {
      return 'Semua kota/kabupaten mengikuti tarif provinsi.';
    }

    final visibleRules = _rules.take(2).map((rule) {
      final detail = rule.excluded
          ? 'Tidak dilayani'
          : AppFormatters.formatCurrencyInt(rule.overrideTariff ?? widget.args.provinceTariff);
      return '- ${rule.cityName}: $detail';
    }).toList();
    final overflow = _rules.length > 2
        ? '\n- +${_rules.length - 2} aturan lainnya'
        : '';
    return '${_rules.length} aturan khusus\n${visibleRules.join('\n')}$overflow';
  }

  @override
  Widget build(BuildContext context) {
    final province = widget.args.province;
    return Scaffold(
      appBar: AppBar(
        title: const Text('Atur Kota/Kabupaten'),
        backgroundColor: AppColors.primaryRed,
        foregroundColor: Colors.white,
      ),
      body: ListView(
        padding: const EdgeInsets.fromLTRB(16, 16, 16, 24),
        children: [
          Container(
            padding: const EdgeInsets.all(14),
            decoration: BoxDecoration(
              color: AppColors.primaryRed.withValues(alpha: 0.05),
              borderRadius: BorderRadius.circular(12),
              border: Border.all(
                color: AppColors.primaryRed.withValues(alpha: 0.2),
              ),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  province.name,
                  style: const TextStyle(
                    fontSize: 18,
                    fontWeight: FontWeight.w700,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  'Tarif provinsi: ${AppFormatters.formatCurrencyInt(widget.args.provinceTariff)}',
                ),
                const SizedBox(height: 6),
                const Text('Semua kota/kabupaten mengikuti tarif provinsi.'),
              ],
            ),
          ),
          const SizedBox(height: 16),
          Row(
            children: [
              const Expanded(
                child: Text(
                  'Aturan khusus',
                  style: TextStyle(fontSize: 16, fontWeight: FontWeight.w700),
                ),
              ),
              TextButton.icon(
                onPressed: () => _openRuleEditor(),
                icon: const Icon(Icons.add),
                label: const Text('Tambah aturan'),
              ),
            ],
          ),
          const SizedBox(height: 8),
          if (_rules.isEmpty)
            Container(
              padding: const EdgeInsets.all(16),
              decoration: BoxDecoration(
                border: Border.all(color: AppColors.neutralGray300),
                borderRadius: BorderRadius.circular(12),
              ),
              child: const Text(
                'Belum ada aturan khusus. Semua kota/kabupaten otomatis memakai tarif provinsi.',
              ),
            )
          else
            ...List.generate(_rules.length, (index) {
              final rule = _rules[index];
              return Padding(
                padding: const EdgeInsets.only(bottom: 8),
                child: Card(
                  elevation: 0,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(12),
                    side: BorderSide(color: AppColors.neutralGray200),
                  ),
                  child: ListTile(
                    title: Text(rule.cityName),
                    subtitle: Text(
                      rule.excluded
                          ? 'Tidak dilayani'
                          : 'Override tarif: ${AppFormatters.formatCurrencyInt(rule.overrideTariff ?? widget.args.provinceTariff)}',
                    ),
                    onTap: () => _openRuleEditor(existingRuleIndex: index),
                    trailing: PopupMenuButton<String>(
                      onSelected: (value) {
                        if (value == 'edit') {
                          _openRuleEditor(existingRuleIndex: index);
                        } else if (value == 'reset') {
                          _removeRule(index);
                        }
                      },
                      itemBuilder: (_) => const [
                        PopupMenuItem(value: 'edit', child: Text('Edit')),
                        PopupMenuItem(
                          value: 'reset',
                          child: Text('Reset ke tarif provinsi'),
                        ),
                      ],
                    ),
                  ),
                ),
              );
            }),
          const SizedBox(height: 12),
          Text(
            _summaryText(),
            style: TextStyle(fontSize: 13, color: AppColors.neutralGray700),
          ),
        ],
      ),
      bottomNavigationBar: SafeArea(
        minimum: const EdgeInsets.fromLTRB(16, 8, 16, 16),
        child: ElevatedButton(
          onPressed: _save,
          style: ElevatedButton.styleFrom(
            backgroundColor: AppColors.primaryRed,
            foregroundColor: Colors.white,
            minimumSize: const Size.fromHeight(50),
          ),
          child: const Text('Simpan'),
        ),
      ),
    );
  }
}

class _CoverageDraft {
  Province? province;
  final TextEditingController tariffController = TextEditingController();
  final List<ShippingCityRuleDraft> cityRules = [];

  void dispose() {
    tariffController.dispose();
  }
}

class ShippingCityRuleDraft {
  final String cityId;
  final String cityName;
  final int? overrideTariff;
  final bool excluded;

  const ShippingCityRuleDraft({
    required this.cityId,
    required this.cityName,
    this.overrideTariff,
    required this.excluded,
  });

  ShippingCityRuleDraft copy({
    String? cityId,
    String? cityName,
    int? overrideTariff,
    bool? excluded,
  }) {
    return ShippingCityRuleDraft(
      cityId: cityId ?? this.cityId,
      cityName: cityName ?? this.cityName,
      overrideTariff: overrideTariff ?? this.overrideTariff,
      excluded: excluded ?? this.excluded,
    );
  }
}

class _CoverageCard extends StatelessWidget {
  final _CoverageDraft coverage;
  final List<Province> provinces;
  final ValueChanged<Province?> onProvinceChanged;
  final VoidCallback? onRemove;
  final VoidCallback onOpenCityRules;
  final ValueChanged<int> onRemoveCityRule;
  final VoidCallback onTariffChanged;

  const _CoverageCard({
    required this.coverage,
    required this.provinces,
    required this.onProvinceChanged,
    required this.onRemove,
    required this.onOpenCityRules,
    required this.onRemoveCityRule,
    required this.onTariffChanged,
  });

  String _summaryText() {
    if (coverage.cityRules.isEmpty) {
      return 'Semua kota/kabupaten mengikuti tarif provinsi.';
    }

    final visibleRules = coverage.cityRules.take(2).map((rule) {
      final detail = rule.excluded
          ? 'Tidak dilayani'
          : AppFormatters.formatCurrencyInt(rule.overrideTariff ?? 0);
      return '- ${rule.cityName}: $detail';
    }).toList();
    final overflow = coverage.cityRules.length > 2
        ? '\n- +${coverage.cityRules.length - 2} aturan lainnya'
        : '';
    return '${coverage.cityRules.length} aturan khusus\n${visibleRules.join('\n')}$overflow';
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        border: Border.all(color: AppColors.neutralGray200),
        borderRadius: BorderRadius.circular(16),
        color: isDark ? AppColors.darkGray900 : AppColors.neutralWhite,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              Expanded(
                child: DropdownButtonFormField<Province>(
                  initialValue: coverage.province,
                  isExpanded: true,
                  decoration: const InputDecoration(
                    labelText: 'Provinsi *',
                    border: OutlineInputBorder(),
                  ),
                  items: provinces
                      .map(
                        (province) => DropdownMenuItem<Province>(
                          value: province,
                          child: Text(province.name),
                        ),
                      )
                      .toList(growable: false),
                  onChanged: onProvinceChanged,
                ),
              ),
              if (onRemove != null) ...[
                const SizedBox(width: 8),
                IconButton(
                  onPressed: onRemove,
                  icon: const Icon(Icons.delete_outline),
                  color: AppColors.error,
                ),
              ],
            ],
          ),
          const SizedBox(height: 12),
          TextField(
            controller: coverage.tariffController,
            keyboardType: TextInputType.number,
            inputFormatters: [FilteringTextInputFormatter.digitsOnly],
            onChanged: (_) => onTariffChanged(),
            decoration: const InputDecoration(
              labelText: 'Tarif provinsi *',
              prefixText: 'Rp ',
              border: OutlineInputBorder(),
            ),
          ),
          const SizedBox(height: 12),
          Text(
            _summaryText(),
            style: TextStyle(fontSize: 13, color: AppColors.neutralGray700),
          ),
          const SizedBox(height: 8),
          Align(
            alignment: Alignment.centerLeft,
            child: TextButton.icon(
              onPressed: coverage.province == null ? null : onOpenCityRules,
              icon: const Icon(Icons.location_city_outlined),
              label: const Text('Atur kota/kabupaten'),
            ),
          ),
        ],
      ),
    );
  }
}

class _CityRuleEditorDialog extends ConsumerStatefulWidget {
  final Province province;
  final int provinceTariff;
  final List<ShippingCityRuleDraft> existingRules;
  final ShippingCityRuleDraft? initialRule;

  const _CityRuleEditorDialog({
    required this.province,
    required this.provinceTariff,
    required this.existingRules,
    required this.initialRule,
  });

  @override
  ConsumerState<_CityRuleEditorDialog> createState() =>
      _CityRuleEditorDialogState();
}

class _CityRuleEditorDialogState extends ConsumerState<_CityRuleEditorDialog> {
  City? _city;
  final _overrideController = TextEditingController();
  bool _excluded = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    final initial = widget.initialRule;
    if (initial != null) {
      _city = City(
        id: initial.cityId,
        name: initial.cityName,
        provinceId: widget.province.id,
      );
      _excluded = initial.excluded;
      if (initial.overrideTariff != null) {
        _overrideController.text = initial.overrideTariff.toString();
      }
    }
  }

  @override
  void dispose() {
    _overrideController.dispose();
    super.dispose();
  }

  void _submit() {
    final city = _city;
    if (city == null) {
      setState(() => _error = 'Pilih kota/kabupaten.');
      return;
    }

    final initialRule = widget.initialRule;
    final existingIds = widget.existingRules
        .where(
          (rule) => initialRule == null || rule.cityId != initialRule.cityId,
        )
        .map((rule) => rule.cityId)
        .toSet();

    if (existingIds.contains(city.id)) {
      setState(() => _error = 'Kota ini sudah ditambahkan.');
      return;
    }

    final overrideTariffText = _overrideController.text.trim();
    final overrideTariff = overrideTariffText.isEmpty
        ? null
        : int.tryParse(overrideTariffText);
    if (!_excluded && (overrideTariff == null || overrideTariff <= 0)) {
      setState(() => _error = 'Masukkan tarif override yang valid.');
      return;
    }
    if (_excluded && overrideTariff != null) {
      setState(
        () =>
            _error = 'Kota yang dikecualikan tidak boleh punya tarif override.',
      );
      return;
    }

    Navigator.of(context).pop(
      ShippingCityRuleDraft(
        cityId: city.id,
        cityName: city.name,
        overrideTariff: _excluded ? null : overrideTariff,
        excluded: _excluded,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final initial = widget.initialRule;
    return AlertDialog(
      title: Text(
        initial == null
            ? 'Tambah Aturan Kota'
            : 'Edit Aturan Kota - ${widget.province.name}',
      ),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            CityDropdown(
              selectedCity: _city,
              selectedProvince: widget.province,
              onChanged: (city) => setState(() => _city = city),
            ),
            const SizedBox(height: 12),
            SwitchListTile(
              contentPadding: EdgeInsets.zero,
              value: _excluded,
              onChanged: (value) => setState(() => _excluded = value),
              title: const Text('Tidak dilayani'),
              subtitle: const Text('Jika aktif, kota ini tidak akan dilayani.'),
            ),
            if (!_excluded) ...[
              const SizedBox(height: 4),
              TextField(
                controller: _overrideController,
                keyboardType: TextInputType.number,
                inputFormatters: [FilteringTextInputFormatter.digitsOnly],
                decoration: const InputDecoration(
                  labelText: 'Tarif override',
                  prefixText: 'Rp ',
                  border: OutlineInputBorder(),
                ),
              ),
            ],
            if (_error != null) ...[
              const SizedBox(height: 8),
              Text(_error!, style: const TextStyle(color: AppColors.error)),
            ],
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('Batal'),
        ),
        ElevatedButton(onPressed: _submit, child: const Text('Simpan')),
      ],
    );
  }
}
