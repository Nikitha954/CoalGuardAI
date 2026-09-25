import sys
import os
import io
import tempfile
from PIL import Image
import pytesseract
from werkzeug.utils import secure_filename

try:
    import truststore
    truststore.inject_into_ssl()
except ImportError:
    pass

# Ensure package imports work correctly
sys.path.append(os.path.dirname(os.path.abspath(__file__)))

from flask import Flask, request, jsonify
from risk_engine.engine import calculate_mine_risk
from anomaly_detection.detector import detect_anomalies

app = Flask(__name__)

# CRITICAL FOR WINDOWS: Point directly to the Tesseract executable you installed
pytesseract.pytesseract.tesseract_cmd = r'C:\Program Files\Tesseract-OCR\tesseract.exe'

@app.route("/health", methods=["GET"])
def health():
    return jsonify({
        "success": True, 
        "message": "AI service is running"
    })

@app.route("/predict-risk", methods=["POST"])
def predict_risk():
    data = request.get_json() or {}
    stats = data.get("stats", {})
    
    result = calculate_mine_risk(stats)
    
    return jsonify({
        "success": True,
        "message": "Risk score computed successfully",
        "data": result
    })

@app.route("/detect-anomalies", methods=["POST"])
def check_anomalies():
    data = request.get_json() or {}
    history = data.get("history", [])
    latest = data.get("latest", {})
    
    is_anomaly, reasons = detect_anomalies(history, latest)
    
    return jsonify({
        "success": True,
        "data": {
            "is_anomaly": is_anomaly,
            "reasons": reasons
        }
    })

from services.ocr_processor import process_document_ocr
from services.gemini_service import analyze_observation_with_gemini, handle_voice_query, get_gemini_client


def _process_uploaded_ocr_file(file_storage):
    filename = secure_filename(file_storage.filename or 'upload')
    if not filename:
        raise ValueError('Empty filename submitted')

    temp_dir = tempfile.mkdtemp(prefix='ocr_upload_')
    temp_path = os.path.join(temp_dir, filename)
    try:
        file_storage.save(temp_path)
        return process_document_ocr(temp_path)
    finally:
        if os.path.exists(temp_path):
            os.remove(temp_path)
        if os.path.exists(temp_dir):
            os.rmdir(temp_dir)


@app.route("/ocr", methods=["POST"])
def run_ocr():
    data = request.get_json() or {}
    file_path = data.get("file_path", "")
    
    result = process_document_ocr(file_path)
    
    return jsonify({
        "success": True,
        "message": "OCR parsing completed",
        "data": result
    })

# ==========================================
# NEW TESSERACT OCR ENDPOINT (Direct Upload)
# ==========================================
@app.route('/api/ocr/extract', methods=['POST'])
def extract_text():
    if 'document' not in request.files:
        return jsonify({"success": False, "error": "No document provided in the request"}), 400

    file = request.files['document']
    if file.filename == '':
        return jsonify({"success": False, "error": "Empty filename submitted"}), 400

    try:
        result = _process_uploaded_ocr_file(file)
        return jsonify({
            "success": True,
            "message": "OCR parsing completed",
            "data": result,
            "extracted_text": result.get('ocr_raw_text', '')
        }), 200
    except Exception as e:
        print(f"OCR Error: {e}")
        return jsonify({"success": False, "error": str(e)}), 500

@app.route("/ai/analyze-inspection", methods=["POST"])
def analyze_inspection():
    data = request.get_json() or {}
    observation = data.get("observation", "")
    mine_name = data.get("mine_name", "Unknown Mine")
    inspection_type = data.get("inspection_type", "Safety Audit")
    compliance_rule = data.get("compliance_rule")
    previous_violations = data.get("previous_violations")
    
    if not observation:
        return jsonify({
            "success": False,
            "message": "Observation text is required"
        }), 400
        
    analysis = analyze_observation_with_gemini(
        observation=observation,
        mine_name=mine_name,
        inspection_type=inspection_type,
        compliance_rule=compliance_rule,
        previous_violations=previous_violations
    )
    
    return jsonify({
        "success": True,
        "analysis": analysis
    })

@app.route("/ai/voice-assistant", methods=["POST"])
def voice_assistant():
    data = request.get_json() or {}
    query = data.get("query", "")
    language = data.get("language", "en-IN")
    context_data = data.get("context_data", {})
    
    if not query:
        return jsonify({
            "success": False,
            "message": "Query text is required"
        }), 400
        
    try:
        answer = handle_voice_query(
            query=query,
            language=language,
            context_data=context_data
        )
    except Exception as e:
        from services.gemini_service import synthesize_context_answer
        answer = synthesize_context_answer(query, language, context_data)
    
    return jsonify({
        "success": True,
        "message": "Voice query processed",
        "data": {
            "answer": answer
        }
    })

@app.route("/ai/translate", methods=["POST"])
def translate_text():
    data = request.get_json() or {}
    text = data.get("text", "")
    target_language = data.get("target_language", "English")
    
    if not text:
        return jsonify({
            "success": False,
            "message": "Text is required"
        }), 400
        
    client = get_gemini_client()
    if not client:
        return jsonify({
            "success": True,
            "data": {"translated_text": text}
        })
        
    prompt = f"Translate the following mining inspection observation or note accurately and formally into {target_language}. Output ONLY the translated text, nothing else:\n\n{text}"
    
    for m in ['gemini-3.1-flash-lite', 'gemini-flash-latest', 'gemini-2.5-flash']:
        try:
            r = client.models.generate_content(model=m, contents=prompt)
            if r and r.text:
                return jsonify({
                    "success": True,
                    "data": {"translated_text": r.text.strip()}
                })
        except Exception:
            continue
            
    return jsonify({
        "success": True,
        "data": {"translated_text": text}
    })

if __name__ == "__main__":
    port = int(os.environ.get("PORT", 5000))
    app.run(host="0.0.0.0", port=port)