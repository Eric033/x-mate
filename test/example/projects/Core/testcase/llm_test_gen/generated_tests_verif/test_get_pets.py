import pytest
import requests
from faker import Faker

fake = Faker()

BASE_URL = "https://petstore.example.com/api"  # 替换为实际的API基础URL

@pytest.fixture
def api_client():
    """创建API客户端fixture"""
    return requests.Session()

def test_list_all_pets(api_client):
    """测试获取所有宠物列表的API"""
    # 发送GET请求到/pets端点
    response = api_client.get(f"{BASE_URL}/pets")
    
    # 验证响应状态码为200
    assert response.status_code == 200
    
    # 验证响应内容是JSON格式
    assert response.headers.get('Content-Type') == 'application/json'
    
    # 验证响应体包含宠物列表
    pets = response.json()
    assert isinstance(pets, list)
    
    # 如果有宠物数据，验证每个宠物对象的基本结构
    if pets:
        for pet in pets:
            assert 'id' in pet
            assert 'name' in pet
            assert 'category' in pet
            assert 'photoUrls' in pet
            assert 'tags' in pet
            assert 'status' in pet

def test_list_all_pets_with_empty_database(api_client):
    """测试当数据库中没有宠物时的API响应"""
    # 这个测试可能需要先清空数据库，或者使用一个已知为空的测试环境
    # 这里我们假设可以模拟一个空数据库的情况
    
    # 发送GET请求到/pets端点
    response = api_client.get(f"{BASE_URL}/pets")
    
    # 验证响应状态码为200
    assert response.status_code == 200
    
    # 验证响应体是空列表
    pets = response.json()
    assert pets == []

def test_list_all_pets_with_large_database(api_client):
    """测试当数据库中有大量宠物时的API响应"""
    # 这个测试可能需要先填充大量测试数据，或者使用一个已知包含大量数据的测试环境
    
    # 发送GET请求到/pets端点
    response = api_client.get(f"{BASE_URL}/pets")
    
    # 验证响应状态码为200
    assert response.status_code == 200
    
    # 验证响应体包含宠物列表
    pets = response.json()
    assert isinstance(pets, list)
    
    # 验证返回的宠物数量大于某个阈值（例如100）
    assert len(pets) > 100
    
    # 验证分页信息（如果API支持）
    if 'page' in response.json():
        assert 'page' in response.json()
        assert 'pageSize' in response.json()
        assert 'totalItems' in response.json()