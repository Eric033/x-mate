from openai import OpenAI
from zhipuai import ZhipuAI
from ..config import config

class LLMClient:
    def __init__(self):
        self.provider = config.PROVIDER.lower()
        self.model = config.MODEL
        
        if self.provider == "zhipu":
            # ZhipuAI Native Client
            self.client = ZhipuAI(
                api_key=config.API_KEY
            )
        else:
            # Fallback to OpenAI
            self.client = OpenAI(
                api_key=config.API_KEY,
                base_url=config.BASE_URL
            )

    def generate_completion(self, system_prompt: str, user_prompt: str) -> str:
        try:
            response = self.client.chat.completions.create(
                model=self.model,
                messages=[
                    {"role": "system", "content": system_prompt},
                    {"role": "user", "content": user_prompt}
                ],
                temperature=0.2
            )
            return response.choices[0].message.content
        except Exception as e:
            print(f"Error calling LLM ({self.provider}): {e}")
            return ""
