import time
import os
import random
import mysql.connector
from dotenv import load_dotenv

# Search for .env files to load DB credentials
root_path = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
load_dotenv(os.path.join(root_path, "backend", ".env"))
load_dotenv(os.path.join(root_path, ".env"))

DB_HOST = os.getenv("DB_HOST", "127.0.0.1")
DB_PORT = os.getenv("DB_PORT", "3306")
DB_USER = os.getenv("DB_USER", "root")
DB_PASSWORD = os.getenv("DB_PASSWORD", "")
DB_NAME = os.getenv("DB_NAME", "coal_governance")

MODE_FILE = os.path.join(root_path, "simulator_mode.txt")

def get_current_mode():
    if os.path.exists(MODE_FILE):
        try:
            with open(MODE_FILE, "r") as f:
                return f.read().strip()
        except:
            pass
    return "NORMAL_MODE"

def connect_db():
    return mysql.connector.connect(
        host=DB_HOST,
        port=DB_PORT,
        user=DB_USER,
        password=DB_PASSWORD,
        database=DB_NAME
    )

def generate_telemetry():
    print(f"Simulator started. Writing to DB: {DB_NAME} on {DB_HOST}:{DB_PORT}")
    print(f"Control mode file: {MODE_FILE}")
    
    while True:
        mode = get_current_mode()
        print(f"Current Simulation Mode: {mode} (ticking...)")
        
        try:
            db = connect_db()
            cursor = db.cursor()
            
            # Fetch all active mines
            cursor.execute("SELECT id, mine_name, production_capacity FROM mines WHERE status='ACTIVE'")
            mines = cursor.fetchall()
            
            for mine_id, mine_name, capacity in mines:
                # capacity is annual, calculate expected daily production (tonnes)
                expected_daily = float(capacity) / 365.0
                
                # Setup defaults based on mode
                prod_factor = random.uniform(0.95, 1.05)
                att_pct = random.uniform(92.0, 96.0)
                equip_health = random.uniform(90.0, 95.0)
                
                aqi = random.uniform(50.0, 90.0)
                wqi = random.uniform(80.0, 90.0)
                noise = random.uniform(65.0, 75.0)
                dust = random.uniform(80.0, 130.0)
                
                # Apply anomalies based on modes for Gevra (Mine ID 1) specifically to match the SIH scenario
                if mine_id == 1:
                    if mode == "PRODUCTION_ANOMALY":
                        prod_factor = 0.55 # 45% drop in production
                        equip_health = 68.0 # equipment health drop
                        
                        # Write an anomaly log directly
                        cursor.execute("SELECT COUNT(*) FROM anomalies WHERE mine_id=1 AND anomaly_type='PRODUCTION' AND status='NEW'")
                        if cursor.fetchone()[0] == 0:
                            desc = "Production decreased 45% below target expected range (Equipment health decline)."
                            cursor.execute("""
                                INSERT INTO anomalies (mine_id, anomaly_type, description, detected_value, expected_value, severity, status)
                                VALUES (1, 'PRODUCTION', %s, %s, %s, 'HIGH', 'NEW')""",
                                (desc, expected_daily * 0.55, expected_daily))
                            db.commit()
                            print(f"[ANOMALY TRIGGERED] Logged production anomaly for Gevra")
                            
                    elif mode == "ENVIRONMENTAL_ALERT":
                        aqi = 265.0 # extreme air quality index
                        dust = 310.0 # high dust levels
                        
                        cursor.execute("SELECT COUNT(*) FROM anomalies WHERE mine_id=1 AND anomaly_type='ENVIRONMENTAL' AND status='NEW'")
                        if cursor.fetchone()[0] == 0:
                            desc = f"CPCB limit exceeded: AQI at {aqi:.1f} and Dust concentration at {dust:.1f} ug/m3."
                            cursor.execute("""
                                INSERT INTO anomalies (mine_id, anomaly_type, description, detected_value, expected_value, severity, status)
                                VALUES (1, 'ENVIRONMENTAL', %s, %s, 100.0, 'CRITICAL', 'NEW')""",
                                (desc, aqi))
                            db.commit()
                            print(f"[ANOMALY TRIGGERED] Logged environmental alert for Gevra")
                            
                    elif mode == "SAFETY_INCIDENT":
                        cursor.execute("SELECT COUNT(*) FROM incidents WHERE mine_id=1 AND status='OPEN'")
                        if cursor.fetchone()[0] == 0:
                            desc = "Slope failure detected at Pit Wall Section-B. Operations temporarily suspended for safety inspection."
                            # Reported by রমেশ (Mine Manager = ID 2)
                            cursor.execute("""
                                INSERT INTO incidents (mine_id, incident_type, description, severity, reported_by, incident_date, status)
                                VALUES (1, 'PIT_SLOPE_FAILURE', %s, 'CRITICAL', 2, NOW(), 'OPEN')""",
                                (desc,))
                            # Also raise critical notification
                            cursor.execute("""
                                INSERT INTO notifications (recipient_id, title, message, severity, type, is_read)
                                SELECT id, 'CRITICAL SAFETY INCIDENT', %s, 'CRITICAL', 'INCIDENT', FALSE FROM users WHERE status='ACTIVE'""",
                                (f"Gevra Mine: {desc}",))
                            db.commit()
                            print(f"[INCIDENT TRIGGERED] Logged slope failure safety incident for Gevra")
                            
                    elif mode == "HIGH_RISK_MODE":
                        # In High Risk Mode, Gevra gets critical failures in violations
                        prod_factor = 0.75
                        att_pct = 82.0
                        equip_health = 74.0
                        aqi = 190.0
                        
                        # Generate some random open violations to drive risk score up
                        cursor.execute("SELECT COUNT(*) FROM violations WHERE mine_id=1 AND status='OPEN'")
                        if cursor.fetchone()[0] == 0:
                            # Generate a Critical safety violation
                            cursor.execute("""
                                INSERT INTO violations (violation_code, mine_id, category_id, description, severity, reported_by, deadline, status)
                                VALUES ('VIO-2026-909', 1, 1, 'Critical machinery guarding missing on Crusher belt #3.', 'CRITICAL', 3, DATE_SUB(CURDATE(), INTERVAL 1 DAY), 'OPEN')""")
                            db.commit()
                            print(f"[VIOLATION TRIGGERED] Created critical open violation for Gevra")
                
                # Calculate final values
                actual_production = expected_daily * prod_factor
                record_date = time.strftime("%Y-%m-%d")
                
                # Write to operational_data
                cursor.execute("""
                    INSERT INTO operational_data (mine_id, record_date, production_tonnes, expected_production, equipment_health_pct, attendance_pct)
                    VALUES (%s, %s, %s, %s, %s, %s)
                    ON DUPLICATE KEY UPDATE 
                        production_tonnes = VALUES(production_tonnes),
                        equipment_health_pct = VALUES(equipment_health_pct),
                        attendance_pct = VALUES(attendance_pct)""",
                    (mine_id, record_date, actual_production, expected_daily, equip_health, att_pct))
                
                # Write to environmental_data
                cursor.execute("""
                    INSERT INTO environmental_data (mine_id, record_date, aqi, water_quality_index, noise_level_db, dust_level)
                    VALUES (%s, %s, %s, %s, %s, %s)
                    ON DUPLICATE KEY UPDATE 
                        aqi = VALUES(aqi),
                        water_quality_index = VALUES(water_quality_index),
                        noise_level_db = VALUES(noise_level_db),
                        dust_level = VALUES(dust_level)""",
                    (mine_id, record_date, aqi, wqi, noise, dust))
                
            db.commit()
            cursor.close()
            db.close()
            
        except Exception as e:
            print(f"Simulator DB error: {e}")
            
        # Tick every 5 seconds
        time.sleep(5)

if __name__ == "__main__":
    # Create default simulator mode file if missing
    if not os.path.exists(MODE_FILE):
        try:
            with open(MODE_FILE, "w") as f:
                f.write("NORMAL_MODE")
        except:
            pass
            
    generate_telemetry()
