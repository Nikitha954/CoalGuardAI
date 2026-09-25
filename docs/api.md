# API Documentation — Phase 1

Base URL (local dev via Docker Compose): `http://localhost:8081/api`

All responses follow this envelope:

```json
// Success
{ "success": true, "message": "...", "data": { } }

// Error
{ "success": false, "message": "...", "error": "..." }
```

Protected routes require:
```
Authorization: Bearer <jwt_token>
```

---

## Auth

### POST /auth/login
Public. Body:
```json
{ "email": "admin@coal.gov", "password": "Coal@2026" }
```
Returns `{ token, user }`.

### POST /auth/logout
Protected. No body. Clears server-side audit entry; client discards token.

### GET /auth/me
Protected. Returns the authenticated user's profile.

---

## Mines

| Method | Path | Roles allowed |
|---|---|---|
| GET | /mines | All authenticated roles |
| GET | /mines/:id | All authenticated roles |
| POST | /mines | SUPER_ADMIN |
| PUT | /mines/:id | SUPER_ADMIN |
| DELETE | /mines/:id (soft-deactivate) | SUPER_ADMIN |

Query filters on `GET /mines`: `subsidiary_id`, `state`, `status`.

Create/update body:
```json
{
  "mine_name": "Gevra Opencast Mine",
  "mine_code": "SECL-GEV-01",
  "subsidiary_id": 1,
  "state": "Chhattisgarh",
  "district": "Korba",
  "latitude": 22.3595,
  "longitude": 82.6892,
  "mine_type": "OPENCAST",
  "production_capacity": 35000000,
  "manager_id": 2,
  "status": "ACTIVE"
}
```

### GET /subsidiaries
Protected, all roles. Returns all subsidiaries (for dropdowns).

---

## Compliance

| Method | Path | Roles allowed |
|---|---|---|
| GET | /compliance/rules | All authenticated roles |
| GET | /compliance/categories | All authenticated roles |
| POST | /compliance/rules | SUPER_ADMIN |
| PUT | /compliance/rules/:id | SUPER_ADMIN |
| DELETE | /compliance/rules/:id | SUPER_ADMIN |

Query filters on `GET /compliance/rules`: `category`, `severity`, `status`.

---

## Analytics

### GET /analytics/dashboard
Protected, all roles. Returns top-level KPI counts:
```json
{
  "total_mines": 10,
  "active_mines": 10,
  "overall_compliance": 0,
  "open_violations": 0,
  "critical_violations": 0,
  "overdue_actions": 0,
  "high_risk_mines": 0,
  "total_compliance_rules": 10,
  "active_compliance_rules": 10
}
```
(Violation/risk figures populate once Phase 2/4 modules are live and seeded.)

---

## Upcoming (Phase 2+)

`POST /inspections`, `POST /violations`, `PUT /violations/:id`,
`POST /corrective-actions`, `PUT /corrective-actions/:id`,
`GET /analytics/risk`, `GET /analytics/anomalies`, `GET /gis/mines`,
`GET /gis/incidents`, `POST /reports/generate`,
`GET /notifications`, `PUT /notifications/:id/read`.
