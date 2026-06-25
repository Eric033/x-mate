import pytest
import requests
from faker import Faker

# Initialize Faker for generating test data
fake = Faker()

# Base URL for the API
BASE_URL = "https://api.example.com"

@pytest.fixture
def order_id():
    """Generate a fake order ID for testing."""
    return fake.uuid4()

@pytest.fixture
def user_id():
    """Generate a fake user ID for testing."""
    return fake.uuid4()

def test_delete_order_success(order_id, user_id):
    """Test successful order deletion with valid parameters."""
    params = {
        "reason": "Customer requested cancellation",
        "force_delete": "false",
        "requested_by": user_id
    }
    
    response = requests.delete(f"{BASE_URL}/orders/{order_id}", params=params)
    
    assert response.status_code == 204
    assert not response.content  # 204 responses should have no content

def test_delete_order_force_delete(order_id, user_id):
    """Test order deletion with force_delete parameter set to true."""
    params = {
        "reason": "System maintenance",
        "force_delete": "true",
        "requested_by": user_id
    }
    
    response = requests.delete(f"{BASE_URL}/orders/{order_id}", params=params)
    
    assert response.status_code == 204
    assert not response.content

def test_delete_order_missing_reason(order_id, user_id):
    """Test order deletion without reason parameter (should still work)."""
    params = {
        "force_delete": "false",
        "requested_by": user_id
    }
    
    response = requests.delete(f"{BASE_URL}/orders/{order_id}", params=params)
    
    assert response.status_code == 204
    assert not response.content

def test_delete_order_missing_requested_by(order_id):
    """Test order deletion without requested_by parameter (should fail)."""
    params = {
        "reason": "Customer requested cancellation",
        "force_delete": "false"
    }
    
    response = requests.delete(f"{BASE_URL}/orders/{order_id}", params=params)
    
    assert response.status_code == 403

def test_delete_order_nonexistent(order_id, user_id):
    """Test deletion of a non-existent order."""
    params = {
        "reason": "Testing non-existent order",
        "force_delete": "false",
        "requested_by": user_id
    }
    
    response = requests.delete(f"{BASE_URL}/orders/{order_id}", params=params)
    
    assert response.status_code == 404

def test_delete_order_unauthorized_user(order_id, user_id):
    """Test order deletion with unauthorized user ID."""
    unauthorized_user_id = fake.uuid4()
    params = {
        "reason": "Unauthorized attempt",
        "force_delete": "false",
        "requested_by": unauthorized_user_id
    }
    
    response = requests.delete(f"{BASE_URL}/orders/{order_id}", params=params)
    
    assert response.status_code == 403

@pytest.mark.parametrize("force_delete_value", ["true", "false", "invalid"])
def test_delete_order_force_delete_variations(order_id, user_id, force_delete_value):
    """Test order deletion with different force_delete parameter values."""
    params = {
        "reason": "Testing force_delete variations",
        "force_delete": force_delete_value,
        "requested_by": user_id
    }
    
    response = requests.delete(f"{BASE_URL}/orders/{order_id}", params=params)
    
    # For invalid values, the API should still work but might behave differently
    if force_delete_value == "invalid":
        assert response.status_code in [204, 400]
    else:
        assert response.status_code == 204
        assert not response.content