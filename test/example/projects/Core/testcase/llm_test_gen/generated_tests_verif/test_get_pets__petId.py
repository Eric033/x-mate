import pytest
import requests
from faker import Faker

# Initialize Faker for generating test data
fake = Faker()

# Base URL for the Pet Store API
BASE_URL = "https://petstore.swagger.io/v2"

@pytest.fixture
def valid_pet_id():
    """Generate a valid pet ID for testing."""
    return fake.random_int(min=1, max=10000)

def test_get_pet_by_id(valid_pet_id):
    """Test getting a pet by its ID with valid data."""
    # Send GET request to retrieve pet information
    response = requests.get(f"{BASE_URL}/pets/{valid_pet_id}")
    
    # Assert the response status code is 200
    assert response.status_code == 200
    
    # Assert the response contains expected fields
    response_data = response.json()
    assert "id" in response_data
    assert "name" in response_data
    assert "category" in response_data
    assert "photoUrls" in response_data
    assert "tags" in response_data
    assert "status" in response_data
    
    # Assert the pet ID matches the requested ID
    assert response_data["id"] == valid_pet_id

def test_get_pet_by_id_with_invalid_id():
    """Test getting a pet with an invalid ID (boundary value)."""
    # Test with ID 0 (minimum boundary)
    response = requests.get(f"{BASE_URL}/pets/0")
    assert response.status_code == 404
    
    # Test with very large ID (maximum boundary)
    large_id = fake.random_int(min=1000000, max=9999999)
    response = requests.get(f"{BASE_URL}/pets/{large_id}")
    assert response.status_code == 404
    
    # Test with negative ID (invalid boundary)
    response = requests.get(f"{BASE_URL}/pets/-1")
    assert response.status_code == 404

def test_get_pet_by_id_with_string_id():
    """Test getting a pet with a string ID (invalid input type)."""
    response = requests.get(f"{BASE_URL}/pets/abc")
    assert response.status_code == 400

def test_get_pet_by_id_with_special_characters():
    """Test getting a pet with special characters in ID (invalid input)."""
    response = requests.get(f"{BASE_URL}/pets/%")
    assert response.status_code == 400