import json
import os
import platform
import re
from typing import Any, Dict, List, cast

import numpy as np
import pytesseract
from PIL import Image
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.metrics.pairwise import cosine_similarity

from services.gemini_service import get_gemini_client

# Try importing pypdf for PDF support
try:
    import pypdf
    HAS_PYPDF = True
except ImportError:
    HAS_PYPDF = False

# Set Tesseract executable path for Windows
if platform.system() == "Windows":
    tess_path = r'C:\Program Files\Tesseract-OCR\tesseract.exe'
    if os.path.exists(tess_path):
        pytesseract.pytesseract.tesseract_cmd = tess_path

REQUIRED_FIELDS = [
    "certificate_number",
    "mine_name",
    "mine_code",
    "document_type",
    "inspection_date",
    "inspector_name",
    "compliance_status",
    "violation_details",
    "risk_level",
    "corrective_action",
    "due_date",
    "issue_date",
    "expiry_date",
    "regulatory_reference",
    "ocr_raw_text",
]

CATEGORY_DEFINITIONS = {
    "Mine Details": {
        "keywords": ["mine name", "mine code", "colliery", "pit", "seam", "opencast", "underground", "location", "subsidiary", "cil", "secl", "wcl", "ncl", "mcl", "ecl", "bccl"],
        "field_keys": ["mine_name", "mine_code"]
    },
    "Inspection": {
        "keywords": ["inspection", "inspector", "survey", "audit", "dgms", "certificate", "officer", "inspected on", "authorized by", "clearance"],
        "field_keys": ["document_type", "inspection_date", "inspector_name", "certificate_number"]
    },
    "Compliance": {
        "keywords": ["compliant", "non-compliant", "violation", "breach", "regulation", "cmr 2017", "statutory", "rule", "circular", "regulatory reference", "legal"],
        "field_keys": ["compliance_status", "regulatory_reference", "violation_details"]
    },
    "Risk & Safety": {
        "keywords": ["risk", "hazard", "safety", "critical", "severity", "ppe", "ventilation", "gas monitoring", "methane", "strata control", "roof fall"],
        "field_keys": ["risk_level", "violation_details"]
    },
    "Action & Timeline": {
        "keywords": ["corrective action", "due date", "expiry date", "issue date", "deadline", "remedial", "remediation", "action required", "valid until"],
        "field_keys": ["corrective_action", "due_date", "issue_date", "expiry_date"]
    }
}


def _safe_json_loads(text):
    if not text:
        return {}
    cleaned = text.strip().replace('```json', '').replace('```', '').strip()
    if not cleaned:
        return {}
    try:
        parsed = json.loads(cleaned)
        return parsed if isinstance(parsed, dict) else {}
    except Exception:
        return {}


def _extract_date(text):
    if not text:
        return ""
    patterns = [
        r'\b(?:19|20)\d{2}[-/.](?:0?[1-9]|1[0-2])[-/.](?:0?[1-9]|[12][0-9]|3[01])\b',
        r'\b(?:0?[1-9]|[12][0-9]|3[01])[-/.](?:0?[1-9]|1[0-2])[-/.](?:19|20)?\d{2}\b',
    ]
    for pat in patterns:
        match = re.search(pat, text)
        if match:
            return match.group(0)
    return ""


def _extract_by_labels(text, labels):
    if not text:
        return ""
    lines = text.splitlines()
    # First attempt: line-by-line key-value lookup
    for label in labels:
        label_pattern = re.compile(rf'^\s*{re.escape(label)}\s*[:\-=]?\s*(.*)$', re.IGNORECASE)
        for line in lines:
            m = label_pattern.search(line)
            if m:
                val = m.group(1).strip()
                if val and val.lower() not in {'n/a', 'na', 'none', 'unknown', '-'}:
                    return val

    # Second attempt: inline search across full text
    norm = re.sub(r'\s+', ' ', text)
    for label in labels:
        pat = rf'{re.escape(label)}\s*[:\-=]?\s*([A-Za-z0-9/\-.,() &]{{2,100}})'
        m = re.search(pat, norm, re.IGNORECASE)
        if m:
            val = m.group(1).strip(' :;,-')
            # Trim if another label starts
            for other_label in ["mine", "inspection", "inspector", "compliance", "violation", "risk", "action", "due", "expiry", "certificate", "regulatory"]:
                if other_label in val.lower() and not label.lower().startswith(other_label):
                    idx = val.lower().find(other_label)
                    val = val[:idx].strip(' :;,-')
                    break
            if val and val.lower() not in {'n/a', 'na', 'none', 'unknown', '-'}:
                return val
    return ""


def _safe_unidentified_structure(raw_text: str) -> Dict[str, Any]:
    safe_text = (raw_text or '').strip()
    if not safe_text:
        safe_text = 'No readable text detected by OCR engine.'
    return {
        'certificate_number': 'UNSPECIFIED',
        'mine_name': 'Not detected',
        'mine_code': 'N/A',
        'document_type': 'Unidentified Document',
        'inspection_date': '',
        'inspector_name': '',
        'compliance_status': 'UNKNOWN',
        'violation_details': 'No mining compliance details detected in the uploaded image.',
        'risk_level': 'LOW',
        'corrective_action': 'Upload a valid mine compliance certificate or inspection form.',
        'due_date': '',
        'issue_date': '',
        'expiry_date': '',
        'regulatory_reference': '',
        'ocr_raw_text': safe_text,
        'field_categories': [
            {
                'category': 'Unidentified',
                'score': 0.0,
                'percentage': '0%',
                'matched_terms': [],
                'fields': []
            }
        ]
    }


def _is_mining_context(raw_text: str) -> bool:
    text = (raw_text or '').lower()
    if not text:
        return False
    mining_terms = [
        'mine', 'colliery', 'coal', 'dgms', 'cmr', 'safety', 'inspection', 'compliance',
        'regulation', 'pit', 'opencast', 'underground', 'project', 'secl', 'ncl', 'mcl', 'wcl', 'ecl', 'bccl'
    ]
    normalized = re.sub(r'[^a-z0-9]+', ' ', text)
    matched = sum(1 for term in mining_terms if term in normalized)
    return matched >= 1 and len(normalized.strip().split()) >= 6


def _categorize_ocr_with_tfidf(raw_text: str, structured: Dict[str, Any]) -> List[Dict[str, Any]]:
    """
    Computes TF-IDF vectorization cosine similarity between document text & category definitions.
    Returns categorized breakdown with TF-IDF scores, matched terms, and field mappings.
    """
    full_doc = f"{raw_text} " + " ".join([str(v) for k, v in structured.items() if k != "ocr_raw_text" and isinstance(v, str)])
    full_doc_clean = re.sub(r'[^\w\s]', ' ', full_doc.lower())

    category_docs = {
        cat: " ".join(data["keywords"])
        for cat, data in CATEGORY_DEFINITIONS.items()
    }
    categories = list(category_docs.keys())
    corpus = [full_doc_clean] + [category_docs[c] for c in categories]

    try:
        vectorizer = TfidfVectorizer(stop_words='english', ngram_range=(1, 2))
        tfidf_matrix = cast(Any, vectorizer.fit_transform(corpus))

        doc_vec = tfidf_matrix[0:1, :]
        cat_vecs = tfidf_matrix[1:, :]

        similarities = cosine_similarity(doc_vec, cat_vecs).reshape(-1)

        result = []
        for idx, cat_name in enumerate(categories):
            score = float(similarities[idx])
            # Find top matching TF-IDF terms
            cat_keywords = CATEGORY_DEFINITIONS[cat_name]["keywords"]
            matched_terms = [kw for kw in cat_keywords if kw in full_doc_clean]

            # Field values mapping for this category
            mapped_fields = []
            for fkey in CATEGORY_DEFINITIONS[cat_name]["field_keys"]:
                val = structured.get(fkey, "")
                if val and str(val).strip() not in {"", "-", "N/A", "None identified", "LOW", "COMPLIANT"}:
                    field_label = fkey.replace("_", " ").title()
                    mapped_fields.append({"field": field_label, "value": str(val)})

            result.append({
                "category": cat_name,
                "score": round(score, 3),
                "percentage": f"{int(round(score * 100))}%",
                "matched_terms": matched_terms[:5],
                "fields": mapped_fields
            })

        # Sort by relevance score descending
        result.sort(key=lambda x: x["score"], reverse=True)
        return result
    except Exception as e:
        print(f"[TF-IDF Categorization Warning] {e}")
        return [
            {
                "category": cat,
                "score": 0.5 if idx == 0 else 0.2,
                "percentage": "50%" if idx == 0 else "20%",
                "matched_terms": data["keywords"][:3],
                "fields": []
            }
            for idx, (cat, data) in enumerate(CATEGORY_DEFINITIONS.items())
        ]


def _build_structured_fields(raw_text: str) -> Dict[str, Any]:
    text = raw_text or ""

    # Certificate Number
    cert_no = _extract_by_labels(text, [
        "certificate number", "certificate no", "cert no.", "cert no", "permit no", "authorization no", "dgms no", "reference no"
    ])
    if not cert_no:
        m = re.search(r'\b(?:DGMS|CMR|CIL|SECL|WCL|NCL|MCL|ECL|BCCL)[A-Z0-9/\-]{3,}\b', text, re.IGNORECASE)
        if m:
            cert_no = m.group(0)
        else:
            cert_no = "UNSPECIFIED"

    # Mine Name & Code
    mine_name = _extract_by_labels(text, ["mine name", "name of mine", "colliery name", "colliery", "mine"])
    if not mine_name:
        m_mine = re.search(r'([A-Za-z0-9\s]+(?:Colliery|Mine|OCP|UG|Opencast|Project))', text, re.IGNORECASE)
        if m_mine:
            mine_name = m_mine.group(1).strip()
        else:
            mine_name = "Not detected"

    mine_code = _extract_by_labels(text, ["mine code", "code no", "mine id"])
    if not mine_code:
        m_code = re.search(r'\b[A-Z]{2,4}-\d{2,4}\b', text)
        if m_code:
            mine_code = m_code.group(0)
        else:
            mine_code = "N/A"

    # Document Type
    doc_type = _extract_by_labels(text, ["document type", "certificate type", "report type", "title"])
    if not doc_type:
        lower_t = text.lower()
        if "safety" in lower_t:
            doc_type = "Safety Compliance Certificate"
        elif "environment" in lower_t:
            doc_type = "Environmental Monitoring Clearance"
        elif "inspection" in lower_t:
            doc_type = "Statutory Inspection Report"
        elif "ventilation" in lower_t:
            doc_type = "Ventilation & Gas Audit"
        else:
            doc_type = "Unidentified Document"

    # Inspection Date & Inspector
    ins_date = _extract_by_labels(text, ["inspection date", "inspected on", "date of inspection", "audit date"])
    if not ins_date:
        ins_date = _extract_date(text)

    inspector = _extract_by_labels(text, ["inspector name", "inspected by", "authorised by", "authorized by", "inspector", "surveyor"])

    # Compliance Status
    lower = text.lower()
    comp_status = "COMPLIANT"
    if "non-compliant" in lower or "non compliant" in lower or "breach" in lower or "violation detected" in lower:
        comp_status = "NON_COMPLIANT"
    elif "conditional" in lower or "partially compliant" in lower:
        comp_status = "PARTIALLY_COMPLIANT"

    # Violation Details
    violations = _extract_by_labels(text, ["violation details", "violations", "observations", "non compliance", "breach", "defects"])
    if not violations:
        if comp_status == "NON_COMPLIANT":
            violations = "Statutory breach or hazard observed during inspection"
        else:
            violations = "None identified"

    # Risk Level
    risk_level = "LOW"
    if "critical" in lower:
        risk_level = "CRITICAL"
    elif "high risk" in lower or "high severity" in lower or "high" in lower:
        risk_level = "HIGH"
    elif "medium" in lower or "moderate" in lower:
        risk_level = "MEDIUM"

    # Corrective Action
    action = _extract_by_labels(text, ["corrective action", "remedial action", "action required", "corrective measure", "recommendation"])
    if not action:
        if comp_status == "NON_COMPLIANT":
            action = "Immediate remediation and safety audit required"
        else:
            action = "No specific corrective action detected"

    # Dates
    due_date = _extract_by_labels(text, ["due date", "deadline", "target date"])
    if not due_date and comp_status == "NON_COMPLIANT":
        due_date = _extract_date(text)

    issue_date = _extract_by_labels(text, ["issue date", "issued on", "date of issue"])
    if not issue_date:
        issue_date = ins_date or _extract_date(text)

    expiry_date = _extract_by_labels(text, ["expiry date", "valid until", "expires on", "validity"])
    if not expiry_date:
        # Search for date after issue date
        dates = re.findall(r'\b(?:19|20)\d{2}[-/.](?:0?[1-9]|1[0-2])[-/.](?:0?[1-9]|[12][0-9]|3[01])\b', text)
        if len(dates) > 1:
            expiry_date = dates[-1]

    # Regulatory Reference
    reg_ref = _extract_by_labels(text, ["regulatory reference", "statutory reference", "reference", "under regulation", "cmr", "dgms circular", "act"])
    if not reg_ref:
        m_reg = re.search(r'(?:DGMS\s+Circular\s+\d+/\d+|CMR\s+\d+|Coal\s+Mines\s+Regulations\s+2017|Mines\s+Act\s+1952)', text, re.IGNORECASE)
        if m_reg:
            reg_ref = m_reg.group(0)
        else:
            reg_ref = "Coal Mines Regulations (CMR) 2017"

    structured = {
        "certificate_number": cert_no,
        "mine_name": mine_name,
        "mine_code": mine_code,
        "document_type": doc_type,
        "inspection_date": ins_date,
        "inspector_name": inspector,
        "compliance_status": comp_status,
        "violation_details": violations,
        "risk_level": risk_level,
        "corrective_action": action,
        "due_date": due_date,
        "issue_date": issue_date,
        "expiry_date": expiry_date,
        "regulatory_reference": reg_ref,
        "ocr_raw_text": text,
    }
    return structured


def process_document_ocr(file_path: str) -> Dict[str, Any]:
    try:
        if not file_path:
            raise ValueError('No file path provided for OCR')

        if not os.path.exists(file_path):
            raise FileNotFoundError(f'File does not exist: {file_path}')

        ext = os.path.splitext(file_path)[1].lower()
        raw_text = ""

        # Handle PDF files
        if ext == ".pdf":
            if HAS_PYPDF:
                try:
                    reader = pypdf.PdfReader(file_path)
                    extracted_pages = []
                    for page in reader.pages:
                        t = page.extract_text()
                        if t:
                            extracted_pages.append(t)
                    raw_text = "\n".join(extracted_pages).strip()
                except Exception as pdf_err:
                    print(f"[PDF Text Extraction Warning] {pdf_err}")
            if not raw_text:
                raw_text = f"PDF document uploaded: {os.path.basename(file_path)}. (Text extraction pending or scanned PDF)."
        else:
            # Handle Image files
            img = Image.open(file_path)
            try:
                # Convert to RGB to ensure compatible mode
                if img.mode != 'RGB':
                    img_rgb = img.convert('RGB')
                else:
                    img_rgb = img

                # Run Tesseract with page segmentation mode 6 (uniform block of text)
                raw_text = pytesseract.image_to_string(img_rgb, config='--psm 6').strip()
                if not raw_text or len(raw_text) < 10:
                    # Retry with default PSM mode 3
                    raw_text = pytesseract.image_to_string(img_rgb, config='--psm 3').strip()
            finally:
                img.close()

        if not raw_text:
            raw_text = "No readable text detected by OCR engine."

        # Fail closed for blank, unrelated, or non-mining images. Do not fabricate
        # compliance metadata from arbitrary uploads.
        if not raw_text or not _is_mining_context(raw_text):
            structured = _safe_unidentified_structure(raw_text)
            return structured

        # Extract 13 structured fields
        structured = _build_structured_fields(raw_text)

        # Perform TF-IDF Vectorization & Categorization
        category_scores = _categorize_ocr_with_tfidf(raw_text, structured)
        structured['field_categories'] = category_scores

        # Optionally enhance with Gemini AI if key is present
        client = get_gemini_client()
        if client:
            prompt = f"""
            Analyze this OCR text from a statutory coal mining document. Return strict JSON with EXACTLY these keys:
            - certificate_number
            - mine_name
            - mine_code
            - document_type
            - inspection_date
            - inspector_name
            - compliance_status
            - violation_details
            - risk_level
            - corrective_action
            - due_date
            - issue_date
            - expiry_date
            - regulatory_reference

            OCR Text:
            {raw_text}
            """
            for model_name in ['gemini-3.6-flash']:
                try:
                    response = client.models.generate_content(model=model_name, contents=prompt)
                    raw_string = response.text if response and getattr(response, 'text', None) else '{}'
                    parsed = _safe_json_loads(raw_string)
                    if parsed:
                        for key in REQUIRED_FIELDS:
                            if key == 'ocr_raw_text':
                                continue
                            candidate = parsed.get(key)
                            if candidate and str(candidate).strip() and str(candidate).lower() not in {'mine', 'n/a', 'none', 'unknown', '-'}:
                                structured[key] = str(candidate).strip()
                        structured['ocr_raw_text'] = raw_text
                        structured['field_categories'] = _categorize_ocr_with_tfidf(raw_text, structured)
                        break
                except Exception as exc:
                    print(f'[Gemini OCR Warning] Model {model_name} error: {exc}')

        # Ensure all required fields exist
        for field in REQUIRED_FIELDS:
            if field not in structured or structured[field] is None:
                structured[field] = ''

        # Guarantee raw OCR text is preserved
        structured['ocr_raw_text'] = raw_text
        return structured

    except Exception as e:
        print(f'[OCR Processing Error] {e}')
        fallback_text = f"OCR processing error: {str(e)}"
        structured = _safe_unidentified_structure(fallback_text)
        return structured