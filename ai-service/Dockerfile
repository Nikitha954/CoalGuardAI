# Use a lightweight Python Linux image
FROM python:3.10-slim

# Install Tesseract OCR system dependencies
RUN apt-get update && apt-get install -y \
    tesseract-ocr \
    libtesseract-dev \
    && rm -rf /var/lib/apt/lists/*

# Set up the application directory
WORKDIR /app
COPY . /app

# Install Python packages (including a production server)
RUN pip install --no-cache-dir -r requirements.txt
RUN pip install gunicorn

# Expose the port Render uses
EXPOSE 10000

# Boot the Flask app using Gunicorn
CMD ["gunicorn", "--bind", "0.0.0.0:10000", "app:app"]