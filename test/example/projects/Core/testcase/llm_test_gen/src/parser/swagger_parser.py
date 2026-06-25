import yaml
import json
import requests
from typing import Dict, Any
from .models import ApiModel, Endpoint, Parameter, Response

class SwaggerParser:
    def __init__(self, source: str):
        self.source = source
        self.spec: Dict[str, Any] = {}

    def parse(self) -> ApiModel:
        self._load_spec()
        return self._extract_model()

    def _load_spec(self):
        if self.source.startswith("http"):
            response = requests.get(self.source)
            response.raise_for_status()
            content = response.text
        else:
            with open(self.source, "r", encoding="utf-8") as f:
                content = f.read()

        try:
            self.spec = yaml.safe_load(content)
        except yaml.YAMLError:
            self.spec = json.loads(content)

    def _extract_model(self) -> ApiModel:
        info = self.spec.get("info", {})
        title = info.get("title", "Unknown API")
        version = info.get("version", "1.0")
        description = info.get("description")
        
        # Handle servers/basePath
        base_url = "/"
        if "servers" in self.spec and self.spec["servers"]:
            base_url = self.spec["servers"][0].get("url", "/")
        elif "basePath" in self.spec:
            base_url = self.spec["basePath"]

        endpoints = []
        paths = self.spec.get("paths", {})
        for path, methods in paths.items():
            for method, operation in methods.items():
                if method.lower() not in ["get", "post", "put", "delete", "patch", "options", "head"]:
                    continue
                
                endpoint = self._parse_endpoint(path, method, operation)
                endpoints.append(endpoint)

        return ApiModel(
            title=title,
            version=version,
            description=description,
            base_url=base_url,
            endpoints=endpoints
        )

    def _parse_endpoint(self, path: str, method: str, operation: Dict[str, Any]) -> Endpoint:
        parameters = []
        
        # Parse parameters
        if "parameters" in operation:
            for param in operation["parameters"]:
                parameters.append(Parameter(
                    name=param.get("name"),
                    in_=param.get("in"),
                    description=param.get("description"),
                    required=param.get("required", False),
                    schema_=param.get("schema")
                ))

        # Parse request body (OpenAPI 3)
        request_body = None
        if "requestBody" in operation:
            content = operation["requestBody"].get("content", {})
            if "application/json" in content:
                request_body = content["application/json"].get("schema")

        # Parse responses
        responses = {}
        for code, resp in operation.get("responses", {}).items():
            schema = None
            if "content" in resp and "application/json" in resp["content"]:
                schema = resp["content"]["application/json"].get("schema")
            elif "schema" in resp:  # Swagger 2
                schema = resp["schema"]
                
            responses[code] = Response(
                status_code=str(code),
                description=resp.get("description"),
                schema_=schema
            )

        return Endpoint(
            path=path,
            method=method.upper(),
            summary=operation.get("summary"),
            operation_id=operation.get("operationId"),
            parameters=parameters,
            request_body=request_body,
            responses=responses
        )
