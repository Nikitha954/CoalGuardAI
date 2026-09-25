import gc
import io
import os
import tempfile
import unittest
from unittest.mock import patch

import app as ai_app

from PIL import Image

from services import ocr_processor


class FakeResponse:
    def __init__(self, payload):
        self.text = payload


class FakeClient:
    def __init__(self, payload):
        self.payload = payload
        self.models = self

    def generate_content(self, model, contents):
        return FakeResponse(self.payload)


class TestOCRProcessor(unittest.TestCase):
    def test_extracts_real_fields_from_valid_mining_document(self):
        raw_text = (
            'Coal Mine Safety Compliance Certificate\n\n'
            'Mine Name: Lakhanpur Colliery\n\n'
            'Mine Code: LC-07\n\n'
            'Inspection Date: 2026-09-20\n\n'
            'Inspector Name: R. S. Sharma\n\n'
            'Compliance Status: COMPLIANT\n\n'
            'Violation Details: None identified\n\n'
            'Risk Level: LOW\n\n'
            'Corrective Action: Continue routine monitoring\n'
            'Due Date: 2026-10-15\n\n'
            'Certificate Number: CMR-2026-1187\n\n'
            'Expiry Date: 2027-09-20\n\n'
            'Regulatory Reference: DGMS Circular 2026/07'
        )

        result = ocr_processor._build_structured_fields(raw_text)

        self.assertEqual(result['certificate_number'], 'CMR-2026-1187')
        self.assertEqual(result['mine_name'], 'Lakhanpur Colliery')
        self.assertEqual(result['mine_code'], 'LC-07')
        self.assertEqual(result['inspection_date'], '2026-09-20')
        self.assertEqual(result['violation_details'], 'None identified')
        self.assertEqual(result['compliance_status'], 'COMPLIANT')

    def test_rejects_gemini_confabulation_for_non_mining_documents(self):
        with tempfile.NamedTemporaryFile(suffix='.png', delete=False) as tmp:
            image = Image.new('RGB', (200, 200), color='white')
            image.save(tmp.name)
            image.close()
            temp_path = tmp.name

        try:
            raw_text = 'This is a random photo of a city street and a dog. There is no mining data here.'
            fake_payload = '{"certificate_number":"FAKE-123","mine_name":"Fake Mine","mine_code":"FM-01","document_type":"Safety Certificate","inspection_date":"2024-01-01","inspector_name":"John Doe","compliance_status":"COMPLIANT","violation_details":"None","risk_level":"LOW","corrective_action":"No action","due_date":"2024-12-31","issue_date":"2024-01-01","expiry_date":"2024-12-31","regulatory_reference":"City Rules 2024"}'

            with patch.object(ocr_processor, 'get_gemini_client', return_value=FakeClient(fake_payload)), \
                 patch.object(ocr_processor.pytesseract, 'image_to_string', return_value=raw_text):
                result = ocr_processor.process_document_ocr(temp_path)

            gc.collect()

            self.assertIn('This is a random photo', result['ocr_raw_text'])
            self.assertEqual(result['certificate_number'], 'UNSPECIFIED')
            self.assertEqual(result['mine_name'], 'Not detected')
            self.assertEqual(result['mine_code'], 'N/A')
            self.assertIn('No mining compliance details detected', result['violation_details'])
        finally:
            gc.collect()
            if os.path.exists(temp_path):
                os.unlink(temp_path)

    def test_direct_upload_route_returns_structured_ocr_data(self):
        img = Image.new('RGB', (280, 120), color='white')
        buffer = io.BytesIO()
        img.save(buffer, format='PNG')
        img.close()
        buffer.seek(0)

        with patch.object(ai_app, 'process_document_ocr', return_value={
            'certificate_number': 'CMR-2026-1187',
            'mine_name': 'Lakhanpur Colliery',
            'mine_code': 'LC-07',
            'document_type': 'Safety Certificate',
            'inspection_date': '2026-09-20',
            'inspector_name': 'R. S. Sharma',
            'compliance_status': 'COMPLIANT',
            'violation_details': 'None identified',
            'risk_level': 'LOW',
            'corrective_action': 'Continue routine monitoring',
            'due_date': '2026-10-15',
            'issue_date': '2026-09-20',
            'expiry_date': '2027-09-20',
            'regulatory_reference': 'DGMS Circular 2026/07',
            'ocr_raw_text': 'Mine Name: Lakhanpur Colliery',
        }):
            client = ai_app.app.test_client()
            response = client.post('/api/ocr/extract', data={'document': (buffer, 'sample.png')}, content_type='multipart/form-data')

        self.assertEqual(response.status_code, 200)
        payload = response.get_json()
        self.assertTrue(payload['success'])
        self.assertIn('data', payload)
        self.assertEqual(payload['data']['mine_name'], 'Lakhanpur Colliery')


if __name__ == '__main__':
    unittest.main()
