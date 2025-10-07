# 🔥 Firestore Setup Guide

## Why Firestore for FocusNest?

✅ **Perfect for NoSQL** - Document-based data structure  
✅ **Real-time sync** - Live updates across devices  
✅ **Scalable** - Handles millions of users  
✅ **Integrated** - Works seamlessly with Firebase  
✅ **Free tier** - 50K reads, 20K writes/day  

## 🏗️ Firestore Data Structure

### **Collections & Documents:**

```
focusnest-app/
├── users/{userId}/
│   ├── profile: { bio, birthdate, backgroundImage }
│   └── settings: { preferences, notifications }
│
├── productivities/{productivityId}/
│   ├── userId: string
│   ├── category: string
│   ├── timeConsumedMinutes: number
│   ├── cycleMode: string
│   ├── cycleCount: number
│   ├── description: string
│   ├── mood: string
│   ├── imageUrl: string
│   ├── startedAt: timestamp
│   ├── endedAt: timestamp
│   └── createdAt: timestamp
│
├── chatbot_sessions/{sessionId}/
│   ├── userId: string
│   ├── title: string
│   ├── messages: array
│   ├── createdAt: timestamp
│   └── updatedAt: timestamp
│
└── analytics/{userId}/
    ├── streak: { current, longest, lastActive }
    ├── stats: { totalHours, totalSessions }
    └── lastUpdated: timestamp
```

## 🚀 Quick Start

### 1. **Development (Local Firestore Emulator)**
```bash
# Start with Firestore emulator
./start.sh

# Your services will connect to:
# FIRESTORE_EMULATOR_HOST=firebase-emulator:8080
```

### 2. **Production (Real Firestore)**
```bash
# Set environment variables
export GCP_PROJECT_ID=your-firebase-project-id
export FIRESTORE_EMULATOR_HOST=""  # Remove emulator host

# Deploy to Cloud Run
gcloud run deploy activity-service --source ./activity-service
```

## 📊 Firestore Queries for FocusNest

### **Productivity Queries:**
```javascript
// Get user's productivity entries for a month
db.collection('productivities')
  .where('userId', '==', userId)
  .where('createdAt', '>=', monthStart)
  .where('createdAt', '<', monthEnd)
  .orderBy('createdAt', 'desc')
  .limit(20)

// Get productivity by category
db.collection('productivities')
  .where('userId', '==', userId)
  .where('category', '==', 'kerja')
  .orderBy('createdAt', 'desc')
```

### **Analytics Queries:**
```javascript
// Get streak data
db.collection('analytics')
  .doc(userId)
  .get()

// Get most productive hours
db.collection('productivities')
  .where('userId', '==', userId)
  .get()
  .then(snapshot => {
    // Process timestamps to find peak hours
  })
```

### **Chatbot Queries:**
```javascript
// Get user's chat sessions
db.collection('chatbot_sessions')
  .where('userId', '==', userId)
  .orderBy('createdAt', 'desc')
  .limit(20)
```

## 🔧 Environment Configuration

### **Development (.env):**
```bash
DATASTORE=firestore
GCP_PROJECT_ID=focusnest-dev
FIRESTORE_EMULATOR_HOST=firebase-emulator:8080
AUTH_MODE=noop
```

### **Production (.env):**
```bash
DATASTORE=firestore
GCP_PROJECT_ID=your-firebase-project-id
# No FIRESTORE_EMULATOR_HOST
AUTH_MODE=clerk
CLERK_JWKS_URL=https://your-clerk-instance.clerk.accounts.dev/.well-known/jwks.json
CLERK_ISSUER=https://your-clerk-instance.clerk.accounts.dev
```

## 💰 Firestore Pricing

### **Free Tier (Per Day):**
- **Reads**: 50,000
- **Writes**: 20,000  
- **Deletes**: 20,000
- **Storage**: 1GB

### **After Free Tier:**
- **Reads**: $0.06 per 100K
- **Writes**: $0.18 per 100K
- **Deletes**: $0.02 per 100K
- **Storage**: $0.18 per GB/month

## 🎯 Firestore Advantages for FocusNest

### **1. Real-time Productivity Tracking**
```javascript
// Live updates when user adds productivity entry
db.collection('productivities')
  .where('userId', '==', userId)
  .onSnapshot(snapshot => {
    // Update UI in real-time
  })
```

### **2. Flexible Data Structure**
```javascript
// Easy to add new fields without migration
{
  category: "kerja",
  timeConsumedMinutes: 120,
  cycleMode: "pomodoro",
  cycleCount: 4,
  mood: "focused",
  tags: ["important", "deadline"],  // New field
  location: "home office"           // New field
}
```

### **3. Built-in Analytics**
```javascript
// Aggregate data for analytics
db.collection('productivities')
  .where('userId', '==', userId)
  .where('category', '==', 'kerja')
  .get()
  .then(snapshot => {
    const totalMinutes = snapshot.docs.reduce((sum, doc) => 
      sum + doc.data().timeConsumedMinutes, 0
    )
  })
```

## 🚀 Deployment Options

### **Option 1: Docker + Firestore Emulator (Development)**
```bash
./start.sh  # Uses local Firestore emulator
```

### **Option 2: Cloud Run + Firestore (Production)**
```bash
# Deploy to Cloud Run with real Firestore
gcloud run deploy activity-service --source ./activity-service
```

### **Option 3: Firebase Functions + Firestore (Alternative)**
```bash
# Deploy as Firebase Functions
firebase deploy --only functions
```

## 🔧 Firestore Security Rules

```javascript
// firestore.rules
rules_version = '2';
service cloud.firestore {
  match /databases/{database}/documents {
    // Users can only access their own data
    match /users/{userId} {
      allow read, write: if request.auth != null && request.auth.uid == userId;
    }
    
    match /productivities/{productivityId} {
      allow read, write: if request.auth != null && 
        resource.data.userId == request.auth.uid;
    }
    
    match /chatbot_sessions/{sessionId} {
      allow read, write: if request.auth != null && 
        resource.data.userId == request.auth.uid;
    }
  }
}
```

## 🎯 Perfect for FocusNest!

Firestore is ideal because:

1. ✅ **NoSQL structure** - Perfect for flexible productivity data
2. ✅ **Real-time sync** - Live updates across devices  
3. ✅ **Scalable** - Handles millions of users
4. ✅ **Integrated** - Works with Firebase ecosystem
5. ✅ **Cost-effective** - Free tier covers development

Your choice of Firestore is perfect for a NoSQL productivity app! 🚀
