import argparse
from src.config import config
from src.parser.swagger_parser import SwaggerParser
from src.generator.test_generator import TestGenerator

def main():
    parser = argparse.ArgumentParser(description="Generate Auto Tests from Swagger using LLM")
    parser.add_argument("--swagger", required=True, help="Path or URL to Swagger/OpenAPI definition")
    parser.add_argument("--output", default="generated_tests", help="Output directory for tests")
    
    args = parser.parse_args()
    
    print(f"Processing Swagger: {args.swagger}")
    
    # 1. Parse Swagger
    try:
        swagger_parser = SwaggerParser(args.swagger)
        api_model = swagger_parser.parse()
        print(f"Successfully parsed API: {api_model.title}")
    except Exception as e:
        print(f"Failed to parse Swagger: {e}")
        return

    # 2. Generate Tests
    generator = TestGenerator()
    generator.generate(api_model, args.output)
    
    print("Done.")

if __name__ == "__main__":
    main()
