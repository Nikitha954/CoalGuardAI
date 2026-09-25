# Architecture — AI-Based Smart Governance and Compliance Monitoring System

**Problem Statement:** SIH26024 · Ministry of Coal / Coal India Limited

## System Overview

```
Frontend (HTML/CSS/JS, Chart.js, Leaflet)
    |
    | REST API (JWT auth) / WebSocket (future: live dashboard updates)
    v
Go Backend (Gin)
    |
    +--- MySQL 8+           (all transactional & governance data)
    +--- Python AI Service  (risk scoring, anomaly detection — Phase 4)
    +--- File Storage       (evidence photos, OCR'd documents)
    +--- Notification Layer (in-app alerts, escalation triggers)
```

## Backend Layout

```
backend/
├── main.go            Application entrypoint, CORS, router wiring
├── config/            Environment variable loading
├── database/          MySQL connection pool
├── models/            Struct definitions mirroring DB tables
├── controllers/       Request handlers (one file per module)
├── middleware/         JWT auth + RBAC role guards
├── routes/            Central route registration
├── utils/             JWT, bcrypt, audit logging, response envelopes
└── uploads/           Evidence photos / OCR'd documents (gitignored contents)
```

## Authentication & RBAC Flow

1. `POST /api/auth/login` validates email + bcrypt password hash, issues a JWT
   (12h default expiry) carrying `user_id`, `email`, `role_key`.
2. Every protected route runs `middleware.AuthRequired`, which parses and
   validates the JWT and injects the identity into the Gin context.
3. Mutating routes (create/update/delete) additionally run
   `middleware.RequireRoles(...)`, which checks the authenticated user's
   `role_key` against an explicit allow-list per endpoint.
4. All authentication events and mutating actions are written to `audit_logs`
   via `utils.LogAudit`, satisfying the platform's transparency requirement.

## Database Design Principles

- Every table uses `AUTO_INCREMENT` primary keys, explicit foreign keys, and
  `created_at`/`updated_at` timestamps (see `database/schema.sql`).
- Status fields use `ENUM` types matching the workflow states defined in the
  problem statement (e.g. violation status: OPEN → IN_PROGRESS → RESOLVED →
  VERIFIED → CLOSED / OVERDUE).
- Mines are **soft-deleted** (status = INACTIVE) rather than hard-deleted, to
  preserve historical inspection/violation/audit records.

## Phase Roadmap (see README for full detail)

| Phase | Scope |
|---|---|
| 1 (this release) | Structure, DB, auth, RBAC, mine management |
| 2 | Compliance rule engine (write ops), inspections, violations, corrective actions |
| 3 | Dashboard charts, GIS map, notifications |
| 4 | Python AI service: risk scoring, anomaly detection, recurring-violation analysis |
| 5 | OCR, reports, audit trail UI, simulation engine |
| 6 | Integration, security hardening, Docker, demo scenario |
