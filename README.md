# FocusNest Services Monorepo

This repository hosts the FocusNest backend microservices, designed for deployment on Google Cloud Run with Firestore, Pub/Sub, Cloud Storage, Firebase Cloud Messaging, and Clerk authentication.

## Structure

- `shared-libs` – Reusable DTOs, event contracts, and shared helpers.
- `gateway-api` – Public gateway that proxies requests to internal services.
- `auth-gateway` – Authentiates requests using Clerk JWTs and enforces RBAC.
- `*-service` – Domain-specific microservices (user, activity, session, notification, media, analytics, webhook).
- `mk/` – Shared Makefile utilities.
- `scripts/` – Helper scripts (e.g., end-to-end runner).

## Prerequisites

- Go 1.22+
- Docker & docker-compose (for local orchestration)

## Quickstart

```sh
go work sync
make -C gateway-api run
```
