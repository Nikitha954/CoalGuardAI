def calculate_mine_risk(stats):
    """
    Computes a risk score from 0 to 100 based on weighted mine telemetry indicators
    and returns a classification and an itemized breakdown of contributing factors.
    """
    score = 0.0
    factors = []

    # 1. Violations
    critical_vios = stats.get("critical_violations", 0)
    high_vios = stats.get("high_violations", 0)
    med_vios = stats.get("medium_violations", 0)
    low_vios = stats.get("low_violations", 0)

    if critical_vios > 0:
        added = min(critical_vios * 25, 45) # cap contributions to prevent single metrics dominating
        score += added
        factors.append({"name": f"{critical_vios} critical violations", "impact": added})

    if high_vios > 0:
        added = min(high_vios * 15, 30)
        score += added
        factors.append({"name": f"{high_vios} high violations", "impact": added})

    if med_vios > 0:
        added = min(med_vios * 8, 20)
        score += added
        factors.append({"name": f"{med_vios} medium violations", "impact": added})

    if low_vios > 0:
        added = min(low_vios * 3, 10)
        score += added
        factors.append({"name": f"{low_vios} low violations", "impact": added})

    # 2. Overdue Corrective Actions
    overdue_actions = stats.get("overdue_actions", 0)
    if overdue_actions > 0:
        added = min(overdue_actions * 15, 35)
        score += added
        factors.append({"name": f"{overdue_actions} overdue corrective actions", "impact": added})

    # 3. Anomaly Signals
    has_prod_anomaly = stats.get("production_anomaly", False)
    if has_prod_anomaly:
        score += 12
        factors.append({"name": "Abnormal production drop detected", "impact": 12})

    has_att_anomaly = stats.get("attendance_anomaly", False)
    if has_att_anomaly:
        score += 10
        factors.append({"name": "Worker attendance anomaly", "impact": 10})

    # 4. Environmental readings (AQI / Water Quality Index)
    env_issues = stats.get("environmental_alerts", 0)
    if env_issues > 0:
        added = min(env_issues * 10, 20)
        score += added
        factors.append({"name": f"{env_issues} environmental warning readings", "impact": added})

    # 5. Recurring Violations & Chronic Non-Compliance
    recurring_violations = stats.get("recurring_violations_count", 0)
    recurring_category = stats.get("recurring_category") or "Safety & Operations"
    if recurring_violations > 0:
        added = min(recurring_violations * 10, 30)
        score += added
        factors.append({"name": f"Recurring violations in {recurring_category} ({recurring_violations} occurrences)", "impact": added})

    # Cap overall score at 100
    score = min(score, 100.0)

    # Classify risk
    classification = "LOW"
    if score >= 81:
        classification = "CRITICAL"
    elif score >= 61:
        classification = "HIGH"
    elif score >= 31:
        classification = "MEDIUM"

    return {
        "score": round(score, 1),
        "classification": classification,
        "factors": factors
    }
