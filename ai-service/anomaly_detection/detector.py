import numpy as np
import pandas as pd
from sklearn.ensemble import IsolationForest

def build_baseline_history():
    """
    Generates a synthetic baseline history representing 'normal' mine operations.
    Used to seed/train the Isolation Forest model when actual historical logs are sparse.
    """
    np.random.seed(42)
    n_samples = 60
    
    # Normal distributions
    prod_ratio = np.random.normal(0.98, 0.05, n_samples)          # normal: 93% to 103% of expected production
    equip_health = np.random.normal(92.0, 3.0, n_samples)        # normal: 86% to 98%
    attendance = np.random.normal(94.0, 2.0, n_samples)          # normal: 90% to 98%
    aqi = np.random.normal(65.0, 15.0, n_samples)                 # normal: 35 to 95
    water_quality = np.random.normal(82.0, 5.0, n_samples)       # normal: 72 to 92
    noise = np.random.normal(70.0, 5.0, n_samples)               # normal: 60 to 80 dB
    dust = np.random.normal(120.0, 20.0, n_samples)              # normal: 80 to 160
    
    df = pd.DataFrame({
        "prod_ratio": prod_ratio,
        "equipment_health_pct": equip_health,
        "attendance_pct": attendance,
        "aqi": aqi,
        "water_quality_index": water_quality,
        "noise_level_db": noise,
        "dust_level": dust
    })
    
    # Clip values to realistic ranges
    df["prod_ratio"] = df["prod_ratio"].clip(0.0, 1.2)
    df["equipment_health_pct"] = df["equipment_health_pct"].clip(0.0, 100.0)
    df["attendance_pct"] = df["attendance_pct"].clip(0.0, 100.0)
    
    return df

def detect_anomalies(history_list, latest_record):
    """
    Runs Isolation Forest outlier detection.
    Returns:
      is_anomaly: bool
      reasons: list of strings detailing why it is flagged (e.g. low attendance, low production, high aqi)
    """
    df_base = build_baseline_history()

    # If we have some actual database history, convert it to features and append to baseline
    actual_rows = []
    for r in history_list:
        actual = float(r.get("production_tonnes", 0.0))
        expected = float(r.get("expected_production", 1.0))
        if expected <= 0:
            expected = 1.0
        
        actual_rows.append({
            "prod_ratio": actual / expected,
            "equipment_health_pct": float(r.get("equipment_health_pct", 95.0)),
            "attendance_pct": float(r.get("attendance_pct", 95.0)),
            "aqi": float(r.get("aqi", 60.0)),
            "water_quality_index": float(r.get("water_quality_index", 80.0)),
            "noise_level_db": float(r.get("noise_level_db", 70.0)),
            "dust_level": float(r.get("dust_level", 110.0))
        })
        
    if len(actual_rows) > 0:
        df_actual = pd.DataFrame(actual_rows)
        df_train = pd.concat([df_base, df_actual], ignore_index=True)
    else:
        df_train = df_base

    # Fit Isolation Forest
    model = IsolationForest(n_estimators=100, contamination=0.08, random_state=42)
    model.fit(df_train)

    # Process latest record
    lat_actual = float(latest_record.get("production_tonnes", 0.0))
    lat_expected = float(latest_record.get("expected_production", 1.0))
    if lat_expected <= 0:
        lat_expected = 1.0
        
    lat_features = pd.DataFrame([{
        "prod_ratio": lat_actual / lat_expected,
        "equipment_health_pct": float(latest_record.get("equipment_health_pct", 95.0)),
        "attendance_pct": float(latest_record.get("attendance_pct", 95.0)),
        "aqi": float(latest_record.get("aqi", 60.0)),
        "water_quality_index": float(latest_record.get("water_quality_index", 80.0)),
        "noise_level_db": float(latest_record.get("noise_level_db", 70.0)),
        "dust_level": float(latest_record.get("dust_level", 110.0))
    }])

    # Predict
    pred = model.predict(lat_features)[0] # -1 for outlier, 1 for normal
    
    is_anomaly = (pred == -1)
    reasons = []

    # Identify individual features causing the outlier status using standard threshold limits
    prod_ratio = lat_actual / lat_expected
    if prod_ratio < 0.85:
        drop_pct = round((1.0 - prod_ratio) * 100)
        reasons.append(f"Production decreased {drop_pct}% from expected target")
        is_anomaly = True # Force flag for critical drop
    
    if float(latest_record.get("attendance_pct", 100)) < 85.0:
        reasons.append(f"Worker attendance drop ({latest_record['attendance_pct']}%)")
        is_anomaly = True

    if float(latest_record.get("equipment_health_pct", 100)) < 80.0:
        reasons.append(f"Critical equipment health decline ({latest_record['equipment_health_pct']}%)")
        is_anomaly = True

    if float(latest_record.get("aqi", 0)) > 150.0:
        reasons.append(f"Air Quality Index (AQI: {latest_record['aqi']}) exceeds CPCB safety norms")
        is_anomaly = True

    if float(latest_record.get("water_quality_index", 100)) < 65.0:
        reasons.append(f"Water discharge quality index dropped to {latest_record['water_quality_index']}")
        is_anomaly = True

    if float(latest_record.get("dust_level", 0)) > 200.0:
        reasons.append(f"Dust level concentration ({latest_record['dust_level']} ug/m3) exceeds threshold limits")
        is_anomaly = True

    # If it is an outlier predicted by Isolation Forest but no specific threshold was hit
    if is_anomaly and len(reasons) == 0:
        reasons.append("Multi-variable operational anomaly detected by AI model")

    return is_anomaly, reasons
