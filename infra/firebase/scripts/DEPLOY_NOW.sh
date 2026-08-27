#!/bin/bash

clear
echo "=========================================="
echo "  DEPLOY SELLER PAYMENT FUNCTIONS"
echo "=========================================="
echo ""
echo "Deploying 2 functions:"
echo "  1. chargeNativePayment (payment creation)"
echo "  2. onSellerPaymentEvent (webhook handler)"
echo ""
echo "=========================================="
echo ""

cd "$(dirname "$0")"

echo "[1/2] Building TypeScript..."
npm run build
if [ $? -ne 0 ]; then
    echo ""
    echo "[ERROR] Build failed!"
    exit 1
fi

echo ""
echo "[2/2] Deploying functions..."
echo ""
firebase deploy --only functions:chargeNativePayment,functions:onSellerPaymentEvent

if [ $? -ne 0 ]; then
    echo ""
    echo "[ERROR] Deployment failed!"
    exit 1
fi

echo ""
echo "=========================================="
echo "  DEPLOYMENT SUCCESSFUL!"
echo "=========================================="
echo ""
echo "Functions deployed:"
echo "  - chargeNativePayment"
echo "  - onSellerPaymentEvent"
echo ""
echo "Next: Test payment flow in app"
echo ""
