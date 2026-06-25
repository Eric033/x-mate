import pytest
import requests
from faker import Faker

# Initialize Faker for generating test data
fake = Faker()

# Base URL for the Pet Store API
BASE_URL = "https://petstore.example.com/api"

@pytest.fixture
def pet_data():
    """Fixture to generate random pet data for testing."""
    return {
        "name": fake.name()
    }

def test_create_pet_valid_data(pet_data):
    """Test creating a pet with valid data."""
    # Send POST request to create a pet
    response = requests.post(f"{BASE_URL}/pets", json=pet_data)
    
    # Assert the response status code is 201 (Created)
    assert response.status_code == 201
    
    # Since the expected response is null, we just check the status code
    # In a real scenario, you might want to check the response body if it's not null
    assert response.json() is None

def test_create_pet_empty_name():
    """Test creating a pet with an empty name (boundary value test)."""
    pet_data = {
        "name": ""
    }
    
    response = requests.post(f"{BASE_URL}/pets", json=pet_data)
    
    # Depending on API behavior, this might return 400 or 422
    # For this test, we'll assume it returns 400 Bad Request
    assert response.status_code == 400

def test_create_pet_long_name():
    """Test creating a pet with a very long name (boundary value test)."""
    # Generate a very long name (1000 characters)
    long_name = "a" * 1000
    pet_data = {
        "name": long_name
    }
    
    response = requests.post(f"{BASE_URL}/pets", json=pet_data)
    
    # Depending on API behavior, this might return 400 or 422
    # For this test, we'll assume it returns 400 Bad Request
    assert response.status_code == 400

def test_create_pet_special_characters_name():
    """Test creating a pet with special characters in name (equivalence class test)."""
    pet_data = {
        "name": fake.text(max_nb_chars=20) + "!@#$%^&*()"
    }
    
    response = requests.post(f"{BASE_URL}/pets", json=pet_data)
    
    # This should be a valid request according to the schema
    assert response.status_code == 201
    assert response.json() is None

def test_create_pet_numeric_name():
    """Test creating a pet with numeric name (equivalence class test)."""
    pet_data = {
        "name": "12345"
    }
    
    response = requests.post(f"{BASE_URL}/pets", json=pet_data)
    
    # This should be a valid request according to the schema
    assert response.status_code == 201
    assert response.json() is None