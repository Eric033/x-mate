import os
import re
from typing import Optional
from ..parser.models import ApiModel, Endpoint
from ..llm.client import LLMClient
from ..llm.prompts import SYSTEM_PROMPT, USER_PROMPT_TEMPLATE

class TestGenerator:
    def __init__(self):
        self.llm = LLMClient()

    def generate(self, api_model: ApiModel, output_dir: str):
        if not os.path.exists(output_dir):
            os.makedirs(output_dir)

        print(f"Generating tests for API: {api_model.title} ({len(api_model.endpoints)} endpoints)...")

        for endpoint in api_model.endpoints:
            print(f"  - Processing {endpoint.method} {endpoint.path}...")
            script_content = self._generate_endpoint_test(api_model.title, endpoint)
            
            if script_content:
                filename = self._make_filename(endpoint)
                filepath = os.path.join(output_dir, filename)
                with open(filepath, "w", encoding="utf-8") as f:
                    f.write(script_content)
                print(f"    -> Saved to {filepath}")
            else:
                print("    -> Failed to generate.")

    def _generate_endpoint_test(self, api_title: str, endpoint: Endpoint) -> Optional[str]:
        # Format parameters for prompt
        params_str = "\n".join([f"- {p.name} ({p.in_}): {p.description or ''}" for p in endpoint.parameters])
        if not params_str:
            params_str = "None"
            
        responses_str = "\n".join([f"- {code}: {r.description or ''}" for code, r in endpoint.responses.items()])
        
        user_prompt = USER_PROMPT_TEMPLATE.format(
            title=api_title,
            method=endpoint.method,
            path=endpoint.path,
            summary=endpoint.summary or "No summary",
            parameters=params_str,
            request_body=str(endpoint.request_body) if endpoint.request_body else "None",
            responses=responses_str
        )

        llm_response = self.llm.generate_completion(SYSTEM_PROMPT, user_prompt)
        return self._extract_code(llm_response)

    def _extract_code(self, text: str) -> str:
        # Simple extraction of python code blocks
        match = re.search(r"```python(.*?)```", text, re.DOTALL)
        if match:
            return match.group(1).strip()
        # Fallback if no blocks found (or just one block)
        return text.strip()

    def _make_filename(self, endpoint: Endpoint) -> str:
        # Convert path /users/{id} to users_id
        safe_path = re.sub(r"[^a-zA-Z0-9]", "_", endpoint.path).strip("_")
        return f"test_{endpoint.method.lower()}_{safe_path}.py"
