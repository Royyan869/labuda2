# Firebase Infrastructure

Firebase Cloud Functions and configurations for Labuda.

## Structure

```
infra/firebase/
├── functions/           # Cloud Functions source code
│   ├── src/            # TypeScript source files
│   ├── lib/            # Compiled JavaScript
│   ├── package.json    # Dependencies
│   └── tsconfig.json   # TypeScript config
├── scripts/            # Deployment scripts
│   ├── DEPLOY_NOW.sh
│   ├── DEPLOY_SELLER_FIX.sh
│   ├── deploy-seller-handler.sh
│   └── deploy-webhook.sh
├── firebase.json       # Firebase project config
└── .firebaserc         # Project aliases

```

## Setup

### Prerequisites
- Node.js 18+ (required for Firebase Functions)
- Firebase CLI: `npm install -g firebase-tools`
- Firebase project access

### Install Dependencies
```bash
cd infra/firebase/functions
npm install
```

### Login to Firebase
```bash
firebase login
```

## Development

### Local Testing
```bash
cd infra/firebase/functions
npm run serve
```

This starts the Firebase emulator for local testing.

### Build Functions
```bash
cd infra/firebase/functions
npm run build
```

Compiles TypeScript to JavaScript in the `lib/` directory.

### Lint Code
```bash
cd infra/firebase/functions
npm run lint
```

## Deployment

### Deploy All Functions
```bash
cd infra/firebase
firebase deploy --only functions
```

### Deploy Specific Function
```bash
cd infra/firebase
firebase deploy --only functions:functionName
```

### Using Deployment Scripts

#### Deploy All Functions
```bash
cd infra/firebase/scripts
./DEPLOY_NOW.sh
```

#### Deploy Seller Payment Handler
```bash
cd infra/firebase/scripts
./deploy-seller-handler.sh
```

#### Deploy Webhook
```bash
cd infra/firebase/scripts
./deploy-webhook.sh
```

#### Deploy Seller Fix
```bash
cd infra/firebase/scripts
./DEPLOY_SELLER_FIX.sh
```

## Functions Overview

### Scheduled Jobs
- **processSellerPayments**: Processes monthly seller subscription payments
  - Schedule: Every month on the 1st at 01:00 WIB
  - Timezone: Asia/Jakarta

### HTTP Endpoints
- **webhookHandler**: Handles Midtrans payment notifications
- **sellerPaymentHandler**: Processes seller payment requests

## Configuration

### Environment Variables
Set environment variables using Firebase CLI:

```bash
# Development
firebase functions:config:set midtrans.server_key="your-dev-key"

# Production
firebase use production
firebase functions:config:set midtrans.server_key="your-prod-key"
```

### View Current Config
```bash
firebase functions:config:get
```

## Firestore Rules

### Deploy Rules Only
```bash
cd infra/firebase
firebase deploy --only firestore:rules
```

### Test Rules Locally
```bash
cd infra/firebase
firebase emulators:start --only firestore
```

## Monitoring

### View Logs
```bash
# All functions
firebase functions:log

# Specific function
firebase functions:log --only functionName

# Tail logs (real-time)
firebase functions:log --only functionName --tail
```

### View in Console
Visit [Firebase Console](https://console.firebase.google.com/) to:
- Monitor function execution
- View detailed logs
- Check performance metrics
- Set up alerts

## Troubleshooting

### Build Errors
```bash
cd infra/firebase/functions
rm -rf node_modules lib
npm install
npm run build
```

### Deployment Fails
1. Check Firebase CLI version: `firebase --version` (should be 12.0.0+)
2. Verify you're using the correct project: `firebase use`
3. Check Node.js version in functions/package.json matches runtime
4. Review deployment logs for specific errors

### Function Not Triggering
1. Check function deployment status in Firebase Console
2. Verify trigger configuration (HTTP, Pub/Sub, scheduled)
3. Check function logs for errors
4. Ensure proper IAM permissions

## Best Practices

1. **Use TypeScript**: All functions should be written in TypeScript
2. **Environment Config**: Never hardcode secrets, use Firebase config
3. **Error Handling**: Always catch and log errors properly
4. **Timeouts**: Set appropriate timeout values (max 540s)
5. **Memory**: Adjust memory allocation based on function needs
6. **Testing**: Test locally with emulators before deploying
7. **Monitoring**: Set up alerts for critical functions
8. **Costs**: Monitor usage to avoid unexpected bills

## References

- [Firebase Functions Documentation](https://firebase.google.com/docs/functions)
- [Firebase CLI Reference](https://firebase.google.com/docs/cli)
- [Deployment Guide](../../docs/deployment/DEPLOY_FUNCTIONS_QUICK.md)
