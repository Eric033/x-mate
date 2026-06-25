import pytest
import requests

BASE_URL = "https://api.example.com"  # Replace with actual base URL

@pytest.fixture
def valid_order_data():
    from faker import Faker
    fake = Faker()
    return {
        "customer_id": f"cust_{fake.random_int(min=10000, max=99999)}",
        "items": [
            {
                "product_id": f"prod_{fake.random_int(min=100, max=999)}",
                "quantity": fake.random_int(min=1, max=10)
            }
        ],
        "total_amount": round(fake.random_number(digits=3, fix_len=True) + 0.5, 2),
        "shipping_address": fake.address(),
        "payment_method": fake.random_element(elements=("CREDIT_CARD", "PAYPAL", "BANK_TRANSFER"))
    }

def test_create_order_valid_data(valid_order_data):
    """Test creating a new order with valid data"""
    response = requests.post(f"{BASE_URL}/orders", json=valid_order_data)
    
    assert response.status_code == 201
    response_data = response.json()
    assert "order_id" in response_data
    assert "status" in response_data
    assert response_data["status"] == "created"