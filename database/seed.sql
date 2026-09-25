-- =====================================================================
-- SEED DATA
-- All demo accounts use password: Coal@2026
-- PostgreSQL-compatible version
-- =====================================================================

INSERT INTO roles (role_key, role_name, description) VALUES
('SUPER_ADMIN',        'Super Administrator',   'Full system access: users, subsidiaries, mines, compliance rules'),
('MINE_MANAGER',       'Mine Manager',          'Manages inspections, violations and corrective actions for assigned mine(s)'),
('SAFETY_OFFICER',     'Safety Officer',        'Creates inspections, reports violations, verifies corrective actions'),
('INSPECTOR',          'Field Inspector',       'Conducts field inspections, submits observations with GPS/timestamp'),
('CORPORATE_MANAGER',  'Corporate Manager',     'Views all mines, compares performance, views analytics'),
('REGULATORY_OFFICER', 'Regulatory Officer',    'Views compliance status, violations, audit history and regulatory reports');

INSERT INTO subsidiaries (name, code, headquarters, status) VALUES
('South Eastern Coalfields Limited', 'SECL', 'Bilaspur, Chhattisgarh', 'ACTIVE'),
('Northern Coalfields Limited',      'NCL',  'Singrauli, Madhya Pradesh', 'ACTIVE'),
('Mahanadi Coalfields Limited',      'MCL',  'Sambalpur, Odisha', 'ACTIVE');

-- password_hash = bcrypt("Coal@2026")
INSERT INTO users (full_name, email, password_hash, role_id, subsidiary_id, phone, status) VALUES
('System Administrator',   'admin@coal.gov',     '$2b$10$V/goxVRUUnt78Fk7oRyUYeg/qRVgxhTR75.Flor9CKa6yMNB.VI5e', 1, NULL, '9000000001', 'ACTIVE'),
('Ramesh Kumar (Mine Mgr)','manager@mine.gov',   '$2b$10$V/goxVRUUnt78Fk7oRyUYeg/qRVgxhTR75.Flor9CKa6yMNB.VI5e', 2, 1,    '9000000002', 'ACTIVE'),
('Sunita Sharma (Safety)', 'safety@mine.gov',    '$2b$10$V/goxVRUUnt78Fk7oRyUYeg/qRVgxhTR75.Flor9CKa6yMNB.VI5e', 3, 1,    '9000000003', 'ACTIVE'),
('Arun Verma (Inspector)', 'inspector@mine.gov', '$2b$10$V/goxVRUUnt78Fk7oRyUYeg/qRVgxhTR75.Flor9CKa6yMNB.VI5e', 4, 1,    '9000000004', 'ACTIVE'),
('Priya Nair (Corporate)', 'corporate@coal.gov', '$2b$10$V/goxVRUUnt78Fk7oRyUYeg/qRVgxhTR75.Flor9CKa6yMNB.VI5e', 5, NULL, '9000000005', 'ACTIVE'),
('Deepak Rao (Regulator)', 'regulator@gov.in',   '$2b$10$V/goxVRUUnt78Fk7oRyUYeg/qRVgxhTR75.Flor9CKa6yMNB.VI5e', 6, NULL, '9000000006', 'ACTIVE');

INSERT INTO mines (mine_name, mine_code, subsidiary_id, state, district, latitude, longitude, mine_type, production_capacity, manager_id, status) VALUES
('Gevra Opencast Mine',      'SECL-GEV-01', 1, 'Chhattisgarh', 'Korba',    22.3595, 82.6892, 'OPENCAST',   35000000, 2, 'ACTIVE'),
('Kusmunda Opencast Mine',   'SECL-KUS-01', 1, 'Chhattisgarh', 'Korba',    22.3167, 82.5833, 'OPENCAST',   28000000, 2, 'ACTIVE'),
('Dipka Opencast Mine',      'SECL-DIP-01', 1, 'Chhattisgarh', 'Korba',    22.2833, 82.6333, 'OPENCAST',  26000000, NULL, 'ACTIVE'),
('Jayant Opencast Mine',     'NCL-JAY-01',  2, 'Madhya Pradesh','Singrauli',24.1167, 82.6833, 'OPENCAST', 24000000, NULL, 'ACTIVE'),
('Nigahi Opencast Mine',     'NCL-NIG-01',  2, 'Madhya Pradesh','Singrauli',24.2000, 82.6500, 'OPENCAST', 20000000, NULL, 'ACTIVE'),
('Dudhichua Opencast Mine',  'NCL-DUD-01',  2, 'Madhya Pradesh','Singrauli',24.1500, 82.7000, 'OPENCAST', 16000000, NULL, 'ACTIVE'),
('Lakhanpur Opencast Mine',  'MCL-LAK-01',  3, 'Odisha',        'Jharsuguda',21.8500, 83.9333, 'OPENCAST',12000000, NULL, 'ACTIVE'),
('Basundhara Opencast Mine', 'MCL-BAS-01',  3, 'Odisha',        'Sundargarh',21.9500, 84.1000, 'OPENCAST',18000000, NULL, 'ACTIVE'),
('Bharatpur Opencast Mine',  'MCL-BHA-01',  3, 'Odisha',        'Angul',    20.9500, 84.9500, 'OPENCAST', 15000000, NULL, 'ACTIVE'),
('Talcher Underground Mine', 'MCL-TAL-01',  3, 'Odisha',        'Angul',    20.9500, 85.2167, 'UNDERGROUND', 5000000, NULL, 'ACTIVE');

INSERT INTO compliance_categories (name, description) VALUES
('Safety',      'Occupational health and safety compliance'),
('Environment', 'Environmental clearance and pollution control compliance'),
('Labour',      'Labour law and worker welfare compliance'),
('Production',  'Production planning and reporting compliance'),
('Contractor',  'Contractor management and contract compliance');

INSERT INTO compliance_rules (rule_code, title, description, category_id, frequency, severity, responsible_dept, due_period_days, status) VALUES
('SAF-001', 'Personal Protective Equipment Compliance', 'All workers must wear mandated PPE (helmet, boots, mask) at all times on site.', 1, 'DAILY', 'CRITICAL', 'Safety', 1, 'ACTIVE'),
('SAF-002', 'Fire Safety Equipment Inspection', 'Fire extinguishers and hydrant systems must be inspected and tested.', 1, 'MONTHLY', 'HIGH', 'Safety', 30, 'ACTIVE'),
('SAF-003', 'Machinery Guarding Compliance', 'All moving machinery parts must have safety guards installed.', 1, 'WEEKLY', 'HIGH', 'Safety', 7, 'ACTIVE'),
('ENV-001', 'Air Quality Monitoring', 'Ambient air quality must be monitored and reported as per CPCB norms.', 2, 'DAILY', 'HIGH', 'Environment', 1, 'ACTIVE'),
('ENV-002', 'Water Discharge Quality Compliance', 'Mine water discharge must meet permissible pollution limits.', 2, 'WEEKLY', 'CRITICAL', 'Environment', 7, 'ACTIVE'),
('ENV-003', 'Afforestation/Plantation Target', 'Annual afforestation target as per environmental clearance conditions.', 2, 'ANNUAL', 'MEDIUM', 'Environment', 365, 'ACTIVE'),
('LAB-001', 'Minimum Wages Compliance', 'All workers including contract labour must be paid statutory minimum wages.', 3, 'MONTHLY', 'HIGH', 'Labour', 30, 'ACTIVE'),
('LAB-002', 'Working Hours Compliance', 'Shift working hours must comply with Mines Act provisions.', 3, 'WEEKLY', 'MEDIUM', 'Labour', 7, 'ACTIVE'),
('PRD-001', 'Daily Production Reporting', 'Production figures must be reported daily to corporate MIS.', 4, 'DAILY', 'MEDIUM', 'Production', 1, 'ACTIVE'),
('CON-001', 'Contractor Safety Certification', 'Contractor workers must hold valid safety training certification.', 5, 'QUARTERLY', 'HIGH', 'Contractor', 90, 'ACTIVE');

INSERT INTO departments (mine_id, dept_name, dept_head_id) VALUES
(1, 'Mining Operations', 2),
(1, 'Safety & Rescue', 3),
(1, 'Heavy Earth Moving Machinery (HEMM)', 2),
(2, 'Mining Operations', 2),
(2, 'Safety & Environmental', 3);

INSERT INTO contractors (mine_id, company_name, contact_person, phone, email, contract_type, contract_start, contract_end, status, blacklist_reason) VALUES
(1, 'Apex Mining Logistics Pvt Ltd', 'Vikram Rathore', '9876543210', 'vikram@apexmining.in', 'HEMM Transport', '2025-01-01', '2026-12-31', 'ACTIVE', NULL),
(1, 'Eastern Blasting & Explosives Ltd', 'Mohit Sen', '9876543211', 'mohit@easternblasting.com', 'Blasting Services', '2024-06-01', '2026-09-20', 'ACTIVE', NULL),
(1, 'Suraksha Safety Equipments Ltd', 'Rajesh K', '9876543212', 'rajesh@surakshasafety.in', 'PPE & Safety Supplies', '2024-01-01', '2025-12-31', 'ACTIVE', NULL),
(2, 'Shree Ram Earthmovers', 'Anil Verma', '9876543213', 'anil@srearthmovers.com', 'Overburden Removal', '2023-01-01', '2024-01-01', 'BLACKLISTED', 'Repeated failure to enforce DGMS PPE standards and fatal safety violation during night shift');

INSERT INTO workers (mine_id, worker_code, full_name, designation, department_id, contractor_id, status) VALUES
(1, 'W-GEV-101', 'Santosh Mahato', 'Dumper Operator', 1, 1, 'ACTIVE'),
(1, 'W-GEV-102', 'Birendra Yadav', 'Drill Operator', 1, 2, 'ACTIVE'),
(1, 'W-GEV-103', 'Rajkumar Sahu', 'Safety Inspector Assistant', 2, NULL, 'ACTIVE'),
(1, 'W-GEV-104', 'Kalyan Bhagat', 'HEMM Mechanic', 3, 1, 'ACTIVE'),
(2, 'W-KUS-201', 'Dilip Tirkey', 'Shovel Operator', 4, NULL, 'ACTIVE');

INSERT INTO attendance (mine_id, worker_id, record_date, status, shift, overtime_hours, present_count, total_count, marked_by) VALUES
(1, 1, CURRENT_DATE, 'PRESENT', 'SHIFT_1', 0.00, NULL, NULL, 3),
(1, 2, CURRENT_DATE, 'PRESENT', 'SHIFT_1', 1.50, NULL, NULL, 3),
(1, 3, CURRENT_DATE, 'PRESENT', 'GENERAL', 0.00, NULL, NULL, 3),
(1, 4, CURRENT_DATE, 'LEAVE',   'SHIFT_2', 0.00, NULL, NULL, 3),
(2, 5, CURRENT_DATE, 'PRESENT', 'SHIFT_1', 0.00, NULL, NULL, 2),
(1, NULL, CURRENT_DATE, 'PRESENT', 'GENERAL', 0.00, 480, 520, 2);

INSERT INTO grievances (worker_id, mine_id, category, description, status, assigned_to, resolution_notes) VALUES
(1, 1, 'Safety', 'Dust suppression sprinkler near Haul Road 4 is defective, causing high dust inhalation during dumper rounds.', 'IN_REVIEW', 3, 'Safety team inspecting the sprinkler line today.'),
(2, 1, 'Equipment', 'Damaged safety harness provided in quarry sector 2.', 'SUBMITTED', 2, NULL),
(4, 1, 'Compensation', 'Overtime wage computation discrepancy for night shift operations in August.', 'RESOLVED', 2, 'Discrepancy verified and adjusted in September salary slip.');

INSERT INTO operational_data (mine_id, record_date, production_tonnes, expected_production, equipment_health_pct, attendance_pct) VALUES
(1, CURRENT_DATE - INTERVAL '2 days', 98000.00, 100000.00, 94.50, 92.30),
(1, CURRENT_DATE - INTERVAL '1 day', 102000.00, 100000.00, 96.00, 94.10),
(1, CURRENT_DATE, 99500.00, 100000.00, 95.20, 93.80),
(2, CURRENT_DATE, 78000.00, 80000.00, 91.00, 88.50);

INSERT INTO environmental_data (mine_id, record_date, aqi, water_quality_index, noise_level_db, dust_level) VALUES
(1, CURRENT_DATE - INTERVAL '2 days', 142.50, 78.20, 68.40, 120.50),
(1, CURRENT_DATE - INTERVAL '1 day', 155.00, 76.50, 71.20, 135.00),
(1, CURRENT_DATE, 138.20, 81.00, 66.80, 115.30),
(2, CURRENT_DATE, 168.00, 72.00, 74.50, 148.00);

INSERT INTO documents (
    mine_id, contractor_id, document_type, file_path, certificate_number,
    issue_date, expiry_date, ocr_raw_text, mine_code, inspector_name,
    inspection_date, compliance_status, violation_details, risk_level,
    corrective_action, due_date, regulatory_reference, workflow_status,
    uploaded_by, reviewed_by, reviewed_at, approved_by, approved_at, verified_by, verified_at, status
) VALUES
(1, 1, 'Mine Safety Inspection Report', 'uploads/doc_4_sample1.png', 'DGMS/SAF/2026/0914',
 '2026-09-01', '2027-08-31', 'DGMS Statutory Safety Audit. Haul road dust suppression certified. HEMM maintenance records verified.',
 'SECL-GEV-01', 'Arun Verma (Inspector)', '2026-09-01', 'COMPLIANT', 'No critical violations identified during statutory inspection.', 'LOW',
 'Maintain scheduled water sprinkler runs on haul road 3.', '2026-09-25', 'Coal Mines Regulations (CMR) 2017, Regulation 124', 'REGULATORY_VERIFIED',
 4, 3, '2026-09-02 10:30:00', 2, '2026-09-03 14:15:00', 6, '2026-09-04 11:00:00', 'VALID'),

(1, 2, 'Explosive Magazine Clearance', 'uploads/doc_4_sample2.png', 'DGMS/EXP/2026/8821',
 '2026-08-15', '2027-08-14', 'Explosive magazine inventory and lightning arrester grounding test passed.',
 'SECL-GEV-01', 'Arun Verma (Inspector)', '2026-08-15', 'COMPLIANT', 'All explosive storage norms in compliance with CMR 2017 Chapter XII.', 'LOW',
 'Ensure bi-monthly lightning conductor earth resistance audit.', '2026-09-30', 'Explosives Act 1884 & CMR 2017 Reg 156', 'APPROVED',
 4, 3, '2026-08-16 09:45:00', 2, '2026-08-17 16:20:00', NULL, NULL, 'VALID'),

(2, 3, 'Safety Clearance Certificate', 'uploads/doc_3_sample3.png', 'DGMS/SAF/2026/7412',
 '2026-09-04', '2027-09-03', 'Safety clearance audit for Kusmunda Opencast sector 4. PPE compliance review.',
 'SECL-KUS-01', 'Sunita Sharma (Safety)', '2026-09-04', 'CONDITIONAL', 'Minor PPE deficiency noted on night-shift contract workers.', 'MEDIUM',
 'Deploy mandatory high-visibility retro-reflective vests and safety shoes.', '2026-09-20', 'CMR 2017 Regulation 182', 'REVIEWED',
 3, 3, '2026-09-05 08:30:00', NULL, NULL, NULL, NULL, 'VALID'),

(1, NULL, 'Environmental Clearance Compliance', 'uploads/doc_4_sample4.png', 'MOEF/ENV/2025/1109',
 '2025-09-10', '2026-09-20', 'Annual effluent discharge quality audit. PM10 particulate levels near boundary buffer zone.',
 'SECL-GEV-01', 'Arun Verma (Inspector)', '2026-09-04', 'NON_COMPLIANT', 'Particulate PM10 levels exceeded 100 ug/m3 in sector 2 due to broken water tanker nozzle.', 'HIGH',
 'Immediately replace water tanker mist nozzles and conduct intensive wetting.', '2026-09-12', 'Environment (Protection) Act 1986 & CPCB Air Standards', 'VIOLATION_FLAGGED',
 4, 3, '2026-09-04 15:00:00', NULL, NULL, NULL, NULL, 'EXPIRING_SOON'),

(2, NULL, 'Statutory Notice & Audit', 'uploads/doc_6_sample5.png', 'DGMS/NOT/2026/0044',
 '2026-09-05', '2027-09-04', 'Statutory oversight notice issued regarding slope stability monitoring sensor calibration.',
 'SECL-KUS-01', 'Deepak Rao (Regulator)', '2026-09-05', 'CONDITIONAL', 'Slope stability radar calibration verification pending for highwall bench 3.', 'MEDIUM',
 'Submit radar diagnostic report and bench survey log by due date.', '2026-09-18', 'DGMS Tech Circular No 04 of 2020', 'PENDING_REVIEW',
 6, NULL, NULL, NULL, NULL, NULL, NULL, 'VALID');


