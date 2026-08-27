# Infrastructure

This directory contains all infrastructure-related configurations for Labuda.

## Structure

```
infra/
├── firebase/           # Firebase configurations
│   ├── functions/      # Cloud Functions
│   ├── firebase.json   # Firebase project config
│   └── .firebaserc     # Firebase project aliases
│
├── docker/             # Docker configurations
│   └── docker-compose.yml  # Local development environment
│
└── aws/                # AWS configurations (future)
    └── lambda/         # Lambda functions (future)
```

## Firebase

### Setup
```bash
cd infra/firebase
npm install
```

### Deploy Functions
```bash
cd infra/firebase
firebase deploy --only functions
```

### Deploy Firestore Rules
```bash
cd infra/firebase
firebase deploy --only firestore:rules
```

## Docker

### Local Development
Start all services (PostgreSQL, Redis, Adminer, Redis Commander):
```bash
cd infra/docker
docker-compose up -d
```

Stop all services:
```bash
cd infra/docker
docker-compose down
```

### Services
- **PostgreSQL**: `localhost:5433`
- **Redis**: `localhost:6379`
- **Adminer** (DB GUI): `http://localhost:8081`
- **Redis Commander**: `http://localhost:8082`

## AWS

AWS infrastructure will be added here in the future (Lambda functions, S3 configs, etc.).
