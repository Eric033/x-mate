from typing import List, Optional, Dict, Any
from pydantic import BaseModel

class Parameter(BaseModel):
    name: str
    in_: str  # query, header, path, cookie, body
    description: Optional[str] = None
    required: bool = False
    schema_: Optional[Dict[str, Any]] = None  # JSON schema definition

class Response(BaseModel):
    status_code: str
    description: Optional[str] = None
    schema_: Optional[Dict[str, Any]] = None

class Endpoint(BaseModel):
    path: str
    method: str
    summary: Optional[str] = None
    operation_id: Optional[str] = None
    parameters: List[Parameter] = []
    request_body: Optional[Dict[str, Any]] = None
    responses: Dict[str, Response] = {}

class ApiModel(BaseModel):
    title: str
    version: str
    description: Optional[str] = None
    base_url: Optional[str] = None
    endpoints: List[Endpoint] = []
