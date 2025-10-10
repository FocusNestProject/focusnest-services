#!/bin/bash

# GitHub Secrets Setup for FocusNest Services
# Run this script to set all required secrets for GitHub Actions

echo "🔐 Setting up GitHub Secrets for FocusNest Services"
echo "=================================================="
echo ""

# Project variables
PROJECT_ID="focusnest-470308"
REGION="us-central1"

echo "📦 Setting Repository Variables (public, non-sensitive)..."
gh variable set GCP_PROJECT_ID --body "$PROJECT_ID"
gh variable set GCP_REGION --body "$REGION"

echo ""
echo "🔑 Now let's set the secrets (sensitive data)..."
echo ""

# GCP Service Account Key (already set, but let's confirm)
echo "1/5 Setting GCP_SA_KEY..."
if [ -f "github-sa-key.json" ]; then
  gh secret set GCP_SA_KEY < github-sa-key.json
  echo "✅ GCP_SA_KEY set from github-sa-key.json"
else
  echo "❌ github-sa-key.json not found!"
fi

echo ""
echo "2/5 Setting CLERK_JWKS_URL..."
echo "Enter your Clerk JWKS URL (e.g., https://your-app.clerk.accounts.dev/.well-known/jwks.json):"
read -r CLERK_JWKS_URL
echo "$CLERK_JWKS_URL" | gh secret set CLERK_JWKS_URL

echo ""
echo "3/5 Setting CLERK_ISSUER..."
echo "Enter your Clerk Issuer (e.g., https://your-app.clerk.accounts.dev):"
read -r CLERK_ISSUER
echo "$CLERK_ISSUER" | gh secret set CLERK_ISSUER

echo ""
echo "4/5 Setting CLERK_AUDIENCE..."
echo "Enter your Clerk Audience (optional, press Enter to skip):"
read -r CLERK_AUDIENCE
if [ -n "$CLERK_AUDIENCE" ]; then
  echo "$CLERK_AUDIENCE" | gh secret set CLERK_AUDIENCE
else
  echo "Skipped CLERK_AUDIENCE"
fi

echo ""
echo "5/5 Setting GCS_BUCKET..."
echo "Enter your GCS bucket name for images (e.g., focusnest-images):"
read -r GCS_BUCKET
echo "$GCS_BUCKET" | gh secret set GCS_BUCKET

echo ""
echo "✅ All secrets configured!"
echo ""
echo "📋 Verify secrets:"
gh secret list

echo ""
echo "📋 Verify variables:"
gh variable list

echo ""
echo "🎯 You're ready to deploy! Push to main to trigger deployment."
