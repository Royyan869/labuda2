#!/bin/bash

# Architecture Guard Script
# Checks for Flutter form architecture violations
# Usage: bash tools/architecture_guard.sh [directory]
# Default directory: apps/mobile/lib/features

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default directory to check
DIR="${1:-apps/mobile/lib/features}"

echo -e "${BLUE}=== Architecture Guard Check ===${NC}"
echo "Checking directory: $DIR"
echo ""

# Track violations
VIOLATIONS=0
WARNINGS=0

# Function to print section header
print_section() {
  echo -e "${BLUE}=== $1 ===${NC}"
}

# Function to print violation
print_violation() {
  echo -e "${RED}❌ VIOLATION:${NC} $1"
  ((VIOLATIONS++))
}

# Function to print warning
print_warning() {
  echo -e "${YELLOW}⚠️  WARNING:${NC} $1"
  ((WARNINGS++))
}

# Function to print pass
print_pass() {
  echo -e "${GREEN}✅ PASS:${NC} $1"
}

# 1. Check for forbidden boolean patterns
print_section "1. Checking for forbidden boolean patterns"

if grep -rn "bool _isLoading" "$DIR" --include="*.dart" 2>/dev/null | grep -v "// OK" | grep -v "// allowed"; then
  print_violation "Found 'bool _isLoading' - use 'controller.isLoading' instead"
else
  print_pass "No forbidden '_isLoading' patterns found"
fi

if grep -rn "bool _obscure" "$DIR" --include="*.dart" 2>/dev/null | grep -v "// OK" | grep -v "// allowed"; then
  print_violation "Found 'bool _obscure*' - use 'controller.isPasswordVisible' instead"
else
  print_pass "No forbidden '_obscure' patterns found"
fi

if grep -rn "bool _is.*Error" "$DIR" --include="*.dart" 2>/dev/null | grep -v "// OK" | grep -v "// allowed" | grep -v "isEmailVerified\|isPhoneVerified"; then
  print_violation "Found 'bool _is*Error' - use 'controller.errorMessage' instead"
else
  print_pass "No forbidden '_is*Error' patterns found"
fi

echo ""

# 2. Check for setState(() => loading) pattern
print_section "2. Checking for direct loading state mutation"

if grep -rn "setState(() => _isLoading" "$DIR" --include="*.dart" 2>/dev/null; then
  print_violation "Found 'setState(() => _isLoading' - use 'controller.setLoading()' instead"
else
  print_pass "No direct loading state mutation found"
fi

echo ""

# 3. Check for manual TextFormField usage
print_section "3. Checking for manual TextFormField usage"

MANUAL_TEXTFORM=$(grep -rn "TextFormField(" "$DIR" --include="*.dart" 2>/dev/null | \
  grep -v "auth_text_field.dart" | \
  grep -v "profile_text_field.dart" | \
  grep -v "shared/" | \
  grep -v "// OK" | \
  grep -v "// allowed" || true)

if [ -n "$MANUAL_TEXTFORM" ]; then
  print_warning "Manual TextFormField found - consider using shared widgets"
  echo "$MANUAL_TEXTFORM" | head -5
  if [ $(echo "$MANUAL_TEXTFORM" | wc -l) -gt 5 ]; then
    echo "... and more"
  fi
else
  print_pass "No manual TextFormField usage found (or using shared widgets)"
fi

echo ""

# 4. Count Visibility widgets per file
print_section "4. Checking for excessive Visibility usage"

for file in $(find "$DIR" -name "*_screen.dart" -o -name "*_form.dart" 2>/dev/null); do
  count=$(grep -c "Visibility(" "$file" 2>/dev/null || echo 0)
  if [ "$count" -gt 3 ]; then
    print_warning "$file has $count Visibility widgets - consider state-driven rendering"
  fi
done

if [ $WARNINGS -eq 0 ]; then
  print_pass "No files with excessive Visibility usage"
fi

echo ""

# 5. Verify controller naming
print_section "5. Checking controller naming conventions"

NON_STANDARD=$(grep -rn "class.*FormController" "$DIR" --include="*.dart" 2>/dev/null | \
  grep -v "AuthFormController\|ProfileFormController" | \
  grep -v "// OK" | \
  grep -v "// allowed" || true)

if [ -n "$NON_STANDARD" ]; then
  print_warning "Non-standard controller naming found:"
  echo "$NON_STANDARD"
else
  print_pass "All controllers follow naming convention"
fi

echo ""

# 6. Check for StateView usage
print_section "6. Checking for StateView usage"

SCREEN_FILES=$(find "$DIR" -name "*_screen.dart" 2>/dev/null)
HAS_STATEVIEW=0

for file in $SCREEN_FILES; do
  if grep -q "StateView" "$file" 2>/dev/null; then
    ((HAS_STATEVIEW++))
  fi
done

if [ $HAS_STATEVIEW -gt 0 ]; then
  print_pass "$HAS_STATEVIEW screen(s) using StateView pattern"
else
  print_warning "No screens using StateView pattern - consider adopting it"
fi

echo ""

# 7. Check for field-level state in controllers
print_section "7. Checking for field-level state in controllers"

FIELD_LEVEL=$(grep -rn "Validation\|ValueNotifier\|FocusNode" "$DIR" --include="*controller.dart" 2>/dev/null | \
  grep -v "// OK" | \
  grep -v "// allowed" | \
  grep -v "FormState" || true)

if [ -n "$FIELD_LEVEL" ]; then
  print_violation "Possible field-level state found in controllers:"
  echo "$FIELD_LEVEL" | head -5
else
  print_pass "No field-level state found in controllers"
fi

echo ""

# Final summary
print_section "SUMMARY"
echo -e "Violations: ${RED}$VIOLATIONS${NC}"
echo -e "Warnings: ${YELLOW}$WARNINGS${NC}"
echo ""

if [ $VIOLATIONS -gt 0 ]; then
  echo -e "${RED}❌ ARCHITECTURE GUARD FAILED${NC}"
  echo "Please fix violations before committing."
  exit 1
elif [ $WARNINGS -gt 0 ]; then
  echo -e "${YELLOW}⚠️  ARCHITECTURE GUARD PASSED WITH WARNINGS${NC}"
  echo "Please review warnings and consider improvements."
  exit 0
else
  echo -e "${GREEN}✅ ARCHITECTURE GUARD PASSED${NC}"
  exit 0
fi
