import pytest
import requests

BASE_URL = "https://api.example.com"  # Replace with actual base URL

@pytest.fixture
def valid_order_id():
    return "order12345"

@pytest.fixture
def valid_user_id():
    return "user67890"

@pytest.mark.parametrize("reason, force_delete, expected_status", [
    ("Customer request", False, 204),
    ("Inventory error", True, 204),
    ("", False, 204),
    ("Testing", None, 204),
])
def test_delete_order_success(valid_order_id, valid_user_id, reason, force_delete, expected_status):
    """Test successful order deletion with various valid parameters"""
    params = {
        "reason": reason,
        "requested_by": valid_user_id
    }
    
    if force_delete is not None:
        params["force_delete"] = str(force_delete).lower()
    
    response = requests.delete(
        f"{BASE_URL}/orders/{valid_order_id}",
        params=params
    )
    
    assert response.status_code == expected_status
    assert response.text == ""  # 204 responses should have no content

def test_delete_order_not_found(valid_user_id):
    """Test deletion of non-existent order returns 404"""
    non_existent_order_id = "nonexistent_order"
    params = {
        "reason": "Testing",
        "requested_by": valid_user_id
    }
    
    response = requests.delete(
        f"{BASE_URL}/orders/{non_existent_order_id}",
        params=params
    )
    
    assert response.status_code == 404
    assert "Order not found" in response.text.lower()

def test_delete_order_permission_denied():
    """Test deletion without proper permissions returns 403"""
    valid_order_id = "order12345"
    unauthorized_user_id = "unauthorized_user"
    params = {
        "reason": "Testing",
        "requested_by": unauthorized_user_id
    }
    
    response = requests.delete(
        f"{BASE_URL}/orders/{valid_order_id}",
        params=params
    )
    
    assert response.status_code == 403
    assert "permission denied" in response.text.lower()