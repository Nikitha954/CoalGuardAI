import os
import json
import re

# Inject system certificate store for Windows SSL verification
try:
    import truststore
    truststore.inject_into_ssl()
except ImportError:
    pass

from dotenv import load_dotenv
from google import genai
from google.genai import types

# Load environment variables from backend/.env
current_dir = os.path.dirname(os.path.abspath(__file__))
locations = [
    os.path.join(current_dir, "..", "..", "backend", ".env"),
    os.path.join(current_dir, "..", "backend", ".env"),
    os.path.join(current_dir, "backend", ".env"),
    os.path.join(os.getcwd(), "backend", ".env"),
    os.path.join(os.getcwd(), ".env")
]
for loc in locations:
    if os.path.exists(loc):
        load_dotenv(loc, override=True)
        break

def get_gemini_client():
    api_key = os.getenv("GEMINI_API_KEY")
    if not api_key:
        print("[Gemini AI] GEMINI_API_KEY not found in env.")
        return None
        
    # Print masked key for diagnostics
    masked = api_key[:4] + "..." + api_key[-4:] if len(api_key) > 8 else "too short"
    print(f"[Gemini AI] Successfully loaded GEMINI_API_KEY from env: {masked}")
    
    try:
        return genai.Client(api_key=api_key)
    except Exception as e:
        print(f"Error initializing Gemini Client: {e}")
        return None

def analyze_observation_with_gemini(observation, mine_name="Unknown Mine", inspection_type="Safety Audit", compliance_rule=None, previous_violations=None):
    """
    Analyzes observation details using the Gemini 2.5 Flash model.
    If the API key is missing or the request fails, returns a standard fallback response.
    """
    
    # Pre-defined system instructions
    system_instruction = """
    You are an expert safety inspector and compliance auditor for Coal India Limited.
    Analyze the provided inspection findings, observation text, and previous histories against standard DGMS mining safety and environmental regulations.
    Provide your analysis as a single, strict JSON object with the following fields:
    {
      "category": "Compliance category (e.g. Safety, Environmental, Production, Electrical, Ground Control)",
      "severity": "LOW" or "MEDIUM" or "HIGH" or "CRITICAL",
      "risk_level": "LOW" or "MEDIUM" or "HIGH" or "CRITICAL",
      "risk_score": integer between 0 and 100,
      "summary": "A concise executive summary of the observation and its impact",
      "reasoning": "A brief regulatory reasoning explaining why this severity and risk score were chosen",
      "recommended_action": "Practical and specific corrective actions recommended to address the safety breach",
      "recurring_issue": boolean (true if observation text or previous history indicates repeated patterns, otherwise false),
      "urgency": "IMMEDIATE" or "NEEDS_ATTENTION" or "ROUTINE",
      "confidence": float value between 0.0 and 1.0 representing AI confidence
    }
    Output only the raw JSON. Do not include markdown code block styling like ```json.
    """

    prompt = f"""
    Analyze these inspection details:
    - Mine Name: {mine_name}
    - Inspection Type: {inspection_type}
    - Inspector's Observation: {observation}
    - Associated Compliance Rules: {json.dumps(compliance_rule) if compliance_rule else "None"}
    - Previous Site Violations: {json.dumps(previous_violations) if previous_violations else "[]"}
    """

    client = get_gemini_client()
    if not client:
        print("[Gemini AI] Service unavailable: GEMINI_API_KEY is not set or client initialization failed.")
        return get_fallback_analysis("Unavailable – AI service could not be reached")

    PRIMARY_MODELS = ['gemini-3.1-flash-lite', 'gemini-flash-latest', 'gemini-flash-lite-latest', 'gemini-2.5-flash']
    for m in PRIMARY_MODELS:
        try:
            response = client.models.generate_content(
                model=m,
                contents=prompt,
                config=types.GenerateContentConfig(
                    response_mime_type="application/json",
                    system_instruction=system_instruction,
                    temperature=0.2
                )
            )
            text_response = response.text.strip()
            if text_response.startswith("```"):
                text_response = re.sub(r'^```(?:json)?\n?', '', text_response)
                text_response = re.sub(r'\n?```$', '', text_response).strip()
            result = json.loads(text_response)
            return result
        except Exception as e:
            continue

    return get_fallback_analysis("Unavailable – AI service could not be reached")

def get_fallback_analysis(status_msg):
    """
    Returns a standard mock/fallback analysis object when the AI service is offline.
    """
    return {
        "category": "Compliance",
        "severity": "MEDIUM",
        "risk_level": "MEDIUM",
        "risk_score": 50,
        "summary": status_msg,
        "reasoning": "The automated AI analysis was bypassed or timed out because the external Gemini AI service is currently offline or unconfigured. Deterministic rule evaluation remains fully operational.",
        "recommended_action": "Review the observation manually and log standard corrective action plans.",
        "recurring_issue": False,
        "urgency": "NEEDS_ATTENTION",
        "confidence": 0.0
    }

LANGUAGE_PROFILES = {
    "en-IN": {
        "name": "Indian English",
        "script": "English alphabet",
        "sample": "Gevra mine has an active risk score of 35 with 2 pending violations."
    },
    "en-US": {
        "name": "English",
        "script": "English alphabet",
        "sample": "Gevra mine has an active risk score of 35 with 2 pending violations."
    },
    "hi-IN": {
        "name": "Hindi (हिंदी)",
        "script": "Devanagari script (देवनागरी)",
        "sample": "गेवड़ा खदान का वर्तमान जोखिम स्कोर 35 है और 2 उल्लंघन लंबित हैं।"
    },
    "ta-IN": {
        "name": "Tamil (தமிழ்)",
        "script": "Tamil script (தமிழ் எழுத்துக்கள்)",
        "sample": "கேவ்ரா சுரங்கத்தின் தற்போதைய ஆபத்து மதிப்பீடு 35 ஆக உள்ளது மற்றும் 2 மீறல்கள் நிலுவையில் உள்ளன."
    },
    "te-IN": {
        "name": "Telugu (తెలుగు)",
        "script": "Telugu script (తెలుగు లిపి)",
        "sample": "గేవ్రా గని ప్రస్తుత రిస్క్ స్కోరు 35 మరియు 2 ఉల్లంఘనలు పెండింగ్‌లో ఉన్నాయి."
    }
}

def synthesize_context_answer(query, language, context_data):
    """
    Intelligent context synthesizer that formulates an accurate answer directly from
    the live database snapshot in the requested language (Tamil, Telugu, Hindi, or English)
    if the external Gemini quota is exhausted or rate-limited.
    """
    mines_risk = context_data.get("mines_risk", []) if context_data else []
    violations = context_data.get("pending_violations", []) if context_data else []
    
    q_lower = (query or "").lower()
    
    matched_mine = None
    for m in mines_risk:
        m_name = m.get("mine_name", "")
        tokens = m_name.lower().split()
        if any(t in q_lower for t in tokens if len(t) > 3):
            matched_mine = m
            break
            
    if not matched_mine and mines_risk:
        matched_mine = mines_risk[0]
        
    if matched_mine:
        name = matched_mine.get("mine_name", "Mine")
        score = int(matched_mine.get("risk_score", 0))
        vio_count = 0
        for v in violations:
            if v.get("mine_name") == name:
                vio_count = v.get("open_violations", 0)
                break
                
        if language == "ta-IN":
            return f"{name} சுரங்கத்தின் தற்போதைய ஆபத்து மதிப்பீடு {score} ஆகும், மேலும் {vio_count} மீறல்கள் நிலுவையில் உள்ளன."
        elif language == "te-IN":
            return f"{name} గని ప్రస్తుత రిస్క్ స్కోరు {score}, మరియు {vio_count} ఉల్లంఘనలు పెండింగ్‌లో ఉన్నాయి."
        elif language == "hi-IN":
            return f"{name} का वर्तमान जोखिम स्कोर {score} है, और {vio_count} उल्लंघन लंबित हैं।"
        else:
            return f"{name} currently has a risk score of {score} with {vio_count} pending violations."
            
    # Platform summary
    count = len(mines_risk)
    if language == "ta-IN":
        return f"தற்போது மொத்தம் {count} சுரங்கங்கள் கணினியில் கண்காணிக்கப்படுகின்றன."
    elif language == "te-IN":
        return f"ప్రస్తుతం మొత్తం {count} గనులు వ్యవస్థలో పర్యవేక్షించబడుతున్నాయి."
    elif language == "hi-IN":
        return f"वर्तमान में सिस्टम में कुल {count} खदानों की निगरानी की जा रही है।"
    else:
        return f"Currently a total of {count} mines are being actively monitored in the platform."

def handle_voice_query(query, language, context_data):
    """
    Handles a natural language voice query using Gemini, incorporating
    the provided backend context to answer accurately in the selected language:
      - English (en-IN / en-US)
      - Hindi (hi-IN)
      - Tamil (ta-IN)
      - Telugu (te-IN)
    """
    profile = LANGUAGE_PROFILES.get(language, LANGUAGE_PROFILES.get("en-IN"))
    target_lang_name = profile["name"]
    target_script = profile["script"]

    system_instruction = f"""
    You are an intelligent, concise AI voice assistant for the Coal Governance Platform (Coal India Limited & Ministry of Coal). 
    You assist mining officers, inspectors, and managers by answering operational questions, safety compliance, violation records, and mine status via voice.
    
    Current Database Context (Snapshot):
    {json.dumps(context_data, indent=2) if context_data else "No context available."}
    
    CRITICAL MULTILINGUAL & SCRIPT INSTRUCTIONS:
    1. TARGET LANGUAGE: {target_lang_name} (Code: {language})
    2. TARGET SCRIPT: You MUST write your response entirely in {target_script}.
       - For Tamil ('ta-IN'): Use authentic Tamil script (e.g. {LANGUAGE_PROFILES['ta-IN']['sample']}). NEVER use English or Roman transliteration.
       - For Telugu ('te-IN'): Use authentic Telugu script (e.g. {LANGUAGE_PROFILES['te-IN']['sample']}). NEVER use English or Roman transliteration.
       - For Hindi ('hi-IN'): Use authentic Devanagari script (e.g. {LANGUAGE_PROFILES['hi-IN']['sample']}). NEVER use Roman transliteration.
       - For English ('en-IN'): Use clear Indian English.
    3. STRICTLY DO NOT TRANSLATE TO ENGLISH when Tamil, Telugu, or Hindi is chosen. Respond natively in the requested language.
    4. Base factual details (mine names, risk scores, counts) strictly on the provided context if relevant.
    5. Keep the response concise, clear, and conversational (1 to 2 sentences max), directly optimized for Text-to-Speech (TTS) playback.
    """

    client = get_gemini_client()
    if not client:
        return synthesize_context_answer(query, language, context_data)

    models_to_try = ['gemini-3.1-flash-lite', 'gemini-flash-latest', 'gemini-flash-lite-latest', 'gemini-2.5-flash']

    for m in models_to_try:
        try:
            response = client.models.generate_content(
                model=m,
                contents=query,
                config=types.GenerateContentConfig(
                    system_instruction=system_instruction,
                    temperature=0.2
                )
            )
            if response and response.text and response.text.strip():
                return response.text.strip()
        except Exception as e:
            continue

    return synthesize_context_answer(query, language, context_data)



