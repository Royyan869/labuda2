#!/bin/bash
# Deploy Seller Payment Handler Cloud Function
# This script deploys only the seller subscription payment handler

echo "========================================"
echo "Deploying Seller Payment Handler"
echo "========================================"
echo

echo "Checking Firebase CLI..."
if ! command -v firebase &> /dev/null; then
    echo "ERROR: Firebase CLI not installed"
    echo "Please install: npm install -g firebase-tools"
    exit 1
fi

firebase --version
echo

echo "Building TypeScript..."
npm run build
if [ $? -ne 0 ]; then
    echo "ERROR: Build failed"
    exit 1
fi

echo
echo "Deploying Cloud Function..."
firebase deploy --only functions:onSellerPaymentEvent
if [ $? -ne 0 ]; then
    echo "ERROR: Deployment failed"
    exit 1
fi

echo
echo "========================================"
echo "Deployment Successful!"
echo "========================================"
echo
echo "Function deployed: onSellerPaymentEvent"
echo "Trigger: Firestore document created in payment_events"
echo "Filter: metadata.type == 'seller_subscription'"
echo
echo "To view logs:"
echo "  firebase functions:log --only onSellerPaymentEvent"
echo
echo "To test:"
echo "  1. Set seller fee in admin panel"
echo "  2. Attempt seller upgrade with payment"
echo "  3. Complete payment via Midtrans"
echo "  4. Check function logs for processing"
echo
