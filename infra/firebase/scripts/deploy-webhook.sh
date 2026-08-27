#!/bin/bash

# Deploy Webhook Only Script
# Untuk hemat CPU, hanya deploy fungsi webhook yang diperlukan

echo "=========================================="
echo "Deploy Midtrans Webhook - CPU Efficient"
echo "=========================================="
echo ""

echo "Step 1: Building TypeScript..."
npm run build

if [ $? -ne 0 ]; then
    echo "❌ Build failed!"
    exit 1
fi

echo ""
echo "✅ Build successful!"
echo ""
echo "Step 2: Deploying handleMidtransWebhook function..."
echo ""

firebase deploy --only functions:handleMidtransWebhook

if [ $? -ne 0 ]; then
    echo ""
    echo "❌ Deployment failed!"
    exit 1
fi

echo ""
echo "=========================================="
echo "✅ Webhook Deployed Successfully!"
echo "=========================================="
echo ""
echo "Next Steps:"
echo "1. Copy the webhook URL from above"
echo "2. Set it in Midtrans Dashboard:"
echo "   - Sandbox: https://dashboard.sandbox.midtrans.com/"
echo "   - Production: https://dashboard.midtrans.com/"
echo "3. Settings → Configuration → Payment Notification URL"
echo ""
echo "Webhook URL format:"
echo "https://asia-southeast2-[PROJECT-ID].cloudfunctions.net/handleMidtransWebhook"
echo ""
echo "=========================================="
