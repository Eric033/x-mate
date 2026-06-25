import os
from pydantic import BaseModel
from dotenv import load_dotenv

load_dotenv()

class Config(BaseModel):
    # Support both OpenAI and Zhipu configurations
    API_KEY: str = os.getenv("API_KEY", "")
    # Default to Zhipu's GLM-4 if not specified
    MODEL: str = os.getenv("LLM_MODEL", "glm-4")
    # Zhipu specific (optional if using ZhipuAI client default)
    BASE_URL: str = os.getenv("BASE_URL", "https://open.bigmodel.cn/api/paas/v4/")
    
    # Provider: 'openai' or 'zhipu'
    PROVIDER: str = os.getenv("LLM_PROVIDER", "zhipu")

config = Config()
