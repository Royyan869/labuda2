# 🛠️ LABUDA Compliance Scripts

Automation scripts untuk check dan track GUIDELINES compliance.

---

## 📋 Available Scripts

### 1. `check_compliance.bat`
**Main compliance checker** - Run ini untuk comprehensive check.

```bash
scripts\check_compliance.bat
```

**What it checks:**
- ✅ Barrel files (28 modules)
- ✅ Cross-module imports
- ✅ File sizes (> 300 lines)
- ✅ Flutter analyze
- 📊 Generates detailed report

**When to run:** Daily (morning standup) atau before commit

---

### 2. `generate_report.ps1`
**Detailed report generator** - PowerShell script untuk detailed metrics.

```bash
powershell -ExecutionPolicy Bypass -File scripts\generate_report.ps1
```

**Output:**
- Barrel file coverage
- File size statistics
- Top 5 largest files
- Overall compliance score
- Saves report to file

**When to run:** Weekly (Monday) untuk track progress

---

## 📊 Understanding the Reports

### Compliance Score Breakdown

**Overall Score = (Barrel Score + File Size Score) / 2**

- **Barrel Score:** % of modules with barrel files (Target: 100%)
- **File Size Score:** % of files < 300 lines (Target: 100%)

### Score Interpretation

| Score | Status | Action |
|-------|--------|--------|
| 90-100% | 🟢 GOOD | Maintenance mode |
| 70-89% | 🟡 NEEDS IMPROVEMENT | Prioritize cleanup |
| < 70% | 🔴 CRITICAL | Immediate action |

---

## 🎯 Usage Examples

### Daily Workflow
```bash
# Morning - Check current state
scripts\check_compliance.bat

# After fixing files - Verify improvement
scripts\check_compliance.bat
```

### Weekly Tracking
```bash
# Monday - Generate detailed report
powershell -ExecutionPolicy Bypass -File scripts\generate_report.ps1

# Review generated report file
notepad compliance_report_[timestamp].txt
```

### Before Git Commit
```bash
# Quick check before committing
scripts\check_compliance.bat

# If PASS → Safe to commit
# If FAIL → Fix violations first
```

---

## 📝 Report Files

Generated reports are saved as:
```
compliance_report_YYYYMMDD_HHMMSS.txt
```

**Example:**
```
compliance_report_20251123_110742.txt
```

Keep these reports untuk track progress over time.

---

## 🔧 Troubleshooting

### Script not running?
```bash
# Enable PowerShell scripts execution
powershell -Command "Set-ExecutionPolicy RemoteSigned -Scope CurrentUser"
```

### Permission denied?
```bash
# Run as Administrator
# Right-click → Run as Administrator
```

### Slow execution?
Tunggu 30-60 detik untuk check 700+ files normal.

---

## 📈 Progress Tracking

### Baseline (2025-11-23)
- **Score:** 94%
- **Barrel Files:** 100% (28/28)
- **File Sizes:** 88% (96 files > 300 lines)
- **Cross-imports:** 0 ✅

### Target (Week 6)
- **Score:** 100%
- **Barrel Files:** 100%
- **File Sizes:** 100% (0 files > 300 lines)
- **Cross-imports:** 0

---

## 🚀 Future Enhancements

**Planned:**
- [ ] Auto-fix script untuk extract widgets
- [ ] Git pre-commit hook
- [ ] CI/CD integration
- [ ] Visual dashboard (HTML report)

---

**Last Updated:** 2025-11-23
**Maintained By:** Development Team
