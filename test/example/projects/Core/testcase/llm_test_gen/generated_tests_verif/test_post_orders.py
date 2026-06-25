import pytest
import requests
from faker import Faker
from datetime import datetime

# Initialize Faker
fake = Faker()

# Base URL for the API
BASE_URL = "https://api.example.com"  # Replace with actual base URL

@pytest.fixture
def valid_order_data():
    """Generate valid order data using Faker"""
    return {
        "customer_id": fake.uuid4(),
        "items": [
            {
                "product_id": fake.uuid4(),
                "quantity": fake.random_int(min=1, max=10)
            },
            {
                "product_id": fake.uuid4(),
                "quantity": fake.random_int(min=1, max=10)
            }
        ],
        "total_amount": round(fake.random.uniform(10.0, 1000.0), 2),
        "shipping_address": fake.address(),
        "payment_method": fake.random_element(elements=['CREDIT_CARD', 'PAYPAL', 'BANK_TRANSFER']),
        "order_date": datetime.now().isoformat()
    }

def test_create_order_success(valid_order_data):
    """Test creating a new order with valid data"""
    response = requests.post(f"{BASE_URL}/orders", json=valid_order_data)
    
    # Verify the response status code
    assert response.status_code == 201
    
    # Verify the response contains the order ID
    response_data = response.json()
    assert "order_id" in response_data
    assert isinstance(response_data["order_id"], str)
    
    # Verify other required fields are present
    assert "customer_id" in response_data
    assert "items" in response_data
    assert "total_amount" in response_data
    assert "status" in response_data

def test_create_order_missing_required_fields():
    """Test creating an order with missing required fields"""
    invalid_data = {
        "customer_id": fake.uuid4(),
        # Missing 'items' and 'total_amount'
        "shipping_address": fake.address(),
        "payment_method": fake.random_element(elements=['CREDIT_CARD', 'PAYPAL', 'BANK_TRANSFER'])
    }
    
    response = requests.post(f"{BASE_URL}/orders", json=invalid_data)
    
    # Verify the response status code
    assert response.status_code == 400
    
    # Verify the error message
    response_data = response.json()
    assert "error" in response_data
    assert "items" in response_data["error"]
    assert "total_amount" in response_data["error"]

def test_create_order_invalid_customer_id():
    """Test creating an order with invalid customer_id"""
    invalid_data = {
        "customer_id": 12345,  # Should be a string
        "items": [
            {
                "product_id": fake.uuid4(),
                "quantity": fake.random_int(min=1, max=10)
            }
        ],
        "total_amount": round(fake.random.uniform(10.0, 1000.0), 2)
    }
    
    response = requests.post(f"{BASE_URL}/orders", json=invalid_data)
    
    # Verify the response status code
    assert response.status_code == 400
    
    # Verify the error message
    response_data = response.json()
    assert "error" in response_data
    assert "customer_id" in response_data["error"]

def test_create_order_invalid_items():
    """Test creating an order with invalid items"""
    invalid_data = {
        "customer_id": fake.uuid4(),
        "items": [
            {
                "product_id": 12345,  # Should be a string
                "quantity": fake.random_int(min=1, max=10)
            }
        ],
        "total_amount": round(fake.random.uniform(10.0, 1000.0), 2)
    }
    
    response = requests.post(f"{BASE_URL}/orders", json=invalid_data)
    
    # Verify the response status code
    assert response.status_code == 400
    
    # Verify the error message
    response_data = response.json()
    assert "error" in response_data
    assert "items" in response_data["error"]

def test_create_order_invalid_total_amount():
    """Test creating an order with invalid total_amount"""
    invalid_data = {
        "customer_id": fake.uuid4(),
        "items": [
            {
                "product_id": fake.uuid4(),
                "quantity": fake.random_int(min=1, max=10)
            }
        ],
        "total_amount": "invalid_amount"  # Should be a number
    }
    
    response = requests.post(f"{BASE_URL}/orders", json=invalid_data)
    
    # Verify the response status code
    assert response.status_code == 400
    
    # Verify the error message
    response_data = response.json()
    assert "error" in response_data
    assert "total_amount" in response_data["error"]

def test_create_order_invalid_payment_method():
    """Test creating an order with invalid payment method"""
    invalid_data = {
        "customer_id": fake.uuid4(),
        "items": [
            {
                "product_id": fake.uuid4(),
                "quantity": fake.random_int(min=1, max=10)
            }
        ],
        "total_amount": round(fake.random.uniform(10.0, 1000.0), 2),
        "payment_method": "INVALID_METHOD"  # Not in enum
    }
    
    response = requests.post(f"{BASE_URL}/orders", json=invalid_data)
    
    # Verify the response status code
    assert response.status_code == 400
    
    # Verify the error message
    response_data = response.json()
    assert "error" in response_data
    assert "payment_method" in response_data["error"]

def test_create_order_empty_items():
    """Test creating an order with empty items array"""
    invalid_data = {
        "customer_id": fake.uuid4(),
        "items": [],  # Empty array
        "total_amount": round(fake.random.uniform(10.0, 1000.0), 2)
    }
    
    response = requests.post(f"{BASE_URL}/orders", json=invalid_data)
    
    # Verify the response status code
    assert response.status_code == 400
    
    # Verify the error message
    response_data = response.json()
    assert "error" in response_data
    assert "items" in response_data["error"]

def test_create_order_zero_quantity():
    """Test creating an order with zero quantity"""
    invalid_data = {
        "customer_id": fake.uuid4(),
        "items": [
            {
                "product_id": fake.uuid4(),
                "quantity": 0  # Should be at least 1
            }
        ],
        "total_amount": round(fake.random.uniform(10.0, 1000.0), 2)
    }
    
    response = requests.post(f"{BASE_URL}/orders", json=invalid_data)
    
    # Verify the response status code
    assert response.status_code == 400
    
    # Verify the error message
    response_data = response.json()
    assert "error" in response_data
    assert "items" in response_data["error"]

def test_create_order_negative_total_amount():
    """Test creating an order with negative total amount"""
    invalid_data = {
        "customer_id": fake.uuid4(),
        "items": [
            {
                "product_id": fake.uuid4(),
                "quantity": fake.random_int(min=1, max=10)
            }
        ],
        "total_amount": -10.0  # Should be positive
    }
    
    response = requests.post(f"{BASE_URL}/orders", json=invalid_data)
    
    # Verify the response status code
    assert response.status_code == 400
    
    # Verify the error message
    response_data = response.json()
    assert "error" in response_data
    assert "total_amount" in response_data["error"]