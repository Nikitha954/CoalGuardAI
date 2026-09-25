# SIH Demo Scenario (Full flow — completed across all phases)

This is the target demonstration flow described in the problem statement.
Phase 1 (this release) provides the foundation — login, RBAC, and mine
data — that every later step in this scenario builds on.

## Setup
- 10 mines seeded across 3 subsidiaries (SECL, NCL, MCL)
- 6 demo accounts, one per role (see README credentials table)
- Compliance rule catalog seeded (10 sample rules across 5 categories)

## Scenario: "Mine A" Risk Escalation (Gevra Opencast Mine)

1. **Baseline (today, working):** Log in as `corporate@coal.gov` → Dashboard
   shows Gevra with 0 open violations, ACTIVE status, no risk score yet.
2. **Simulation Control (Phase 5):** From the admin panel, trigger
   `HIGH_RISK_MODE` for Gevra.
3. **Inspection (Phase 2):** Log in as `inspector@mine.gov` → conduct a field
   inspection at Gevra with GPS + timestamp auto-captured → flag a PPE
   violation.
4. **Violation created (Phase 2):** Violation SAF-001 (Critical) appears
   under Gevra with a 24-hour deadline.
5. **Corrective action (Phase 2):** Log in as `manager@mine.gov` → assign a
   corrective action to a responsible worker.
6. **Overdue + escalation (Phase 2/3):** Simulation Control advances the
   clock past the deadline → action auto-marks OVERDUE → escalates to
   Safety Officer, then Mine Manager, then Corporate Management.
7. **AI risk scoring (Phase 4):** The risk engine recomputes Gevra's score
   using the new violation, overdue action, and any anomaly signals from the
   simulator → score jumps from LOW to HIGH/CRITICAL, with a visible factor
   breakdown (e.g. "+25 critical violations, +15 overdue actions").
8. **Anomaly detection (Phase 4):** Simulator triggers `PRODUCTION_ANOMALY`
   for Gevra → Isolation Forest flags an anomaly ("Production decreased 31%
   from expected range") → appears on the dashboard and GIS map (marker
   turns red).
9. **Notifications (Phase 3):** Corporate Manager and Regulatory Officer
   receive an in-app "High risk detected" notification.
10. **Reporting (Phase 5):** Generate a High-Risk Mine Report (PDF) for
    Gevra showing the full timeline above.

Target run-time for the full walkthrough: **5–7 minutes.**

## What Phase 1 already lets you demo today
- Role-based login across all 6 personas (try the demo account switcher on
  the login page).
- SUPER_ADMIN creating/editing/deactivating a mine, with the change
  immediately reflected on the GIS map and dashboard mines table.
- RBAC in action: log in as `inspector@mine.gov` and confirm the "Add Mine"
  button and Users nav item are hidden, while `admin@coal.gov` sees both.
- The compliance rule catalog, filterable by category and severity.
