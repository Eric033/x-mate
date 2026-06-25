import xml.etree.ElementTree as ET
import re
import datetime
import os

class Context:
    """Manages variables for test execution."""
    def __init__(self):
        self.variables = {}

    def set(self, key, value):
        self.variables[key] = str(value)

    def get(self, key):
        return self.variables.get(key, "")

    def replace_variables(self, text):
        """Replaces {{variable}} placeholders with values from context."""
        if not isinstance(text, str):
            return text
        
        def replacer(match):
            key = match.group(1)
            return self.variables.get(key, match.group(0))
        
        return re.sub(r'\{\{(.+?)\}\}', replacer, text)

class XmlHelper:
    """Helper for XML operations."""
    
    @staticmethod
    def load_from_file(filepath):
        tree = ET.parse(filepath)
        return tree

    @staticmethod
    def set_value(tree, xpath, value):
        """Sets value of an element found by xpath. 
        Note: ElementTree xpath support is limited. 
        Adjusting to support simple path and // search."""
        
        # ElementTree doesn't support leading / for absolute path in find() on Element, 
        # but find() on ElementTree object works for absolute paths if they start from root.
        # Here we try to accept flexible inputs.
        
        root = tree.getroot()
        
        # Simple handling: remove leading / if generic find is sufficient
        # But user xpath might be specific. 
        # For simplicity in this demo, we assume standard ET supported paths or //TagName
        
        # If xpath starts with /, try find on tree, else find on root
        # ElementTree support for // is via .//
        
        clean_xpath = xpath
        if xpath.startswith("//"):
             # .// means search anywhere
            clean_xpath = "." + xpath 
            
        # Try finding the element
        elements = root.findall(clean_xpath)
        if not elements:
            # Fallback for standard absolute paths like /Reply_Msg/Body... 
            # In ET findall on root, absolute path should just be path from root tag? No, standard ET is tricky.
            # Let's try to match by tag name if path is complex
            tag_name = xpath.split('/')[-1]
            if tag_name:
                 elements = root.findall(f".//{tag_name}")
        
        if elements:
            # Update all matches or just first? Requirement implies specific field update.
            # Usually one match expected for set.
            elements[0].text = str(value)
        else:
            print(f"Warning: XML Node not found for setting value: {xpath}")

    @staticmethod
    def get_value(tree, xpath):
        root = tree.getroot()
        clean_xpath = xpath
        if xpath.startswith("//"):
            clean_xpath = "." + xpath
            
        elements = root.findall(clean_xpath)
        if not elements:
             # Fallback by tag name
            tag_name = xpath.split('/')[-1]
            if tag_name:
                 elements = root.findall(f".//{tag_name}")
                 
        if elements:
            return elements[0].text
        return None

    @staticmethod
    def to_string(tree):
        return ET.tostring(tree.getroot(), encoding='unicode')

class NetworkHelper:
    """Simulates TCP network operations."""
    
    _balance = 1000.00 # Initial mock balance

    @classmethod
    def send_xml_message(cls, trancode, xml_content):
        print(f"Sending TCP Message [TranCode: {trancode}]:")
        
        # Simple Logic to update mock balance
        # If transaction 010102, deduct 0.01
        if trancode == "010102":
            cls._balance = round(cls._balance - 0.01, 2)

        # MOCK RESPONSE
        response = f"""
        <Reply_Msg>
            <Sys_Head>
                <SEQ_NO>MOCK_SEQ_{datetime.datetime.now().strftime('%H%M%S')}</SEQ_NO>
                <BRANCH_ID>MOCK_BRANCH</BRANCH_ID>
            </Sys_Head>
            <Body>
                <RET_CODE>000000</RET_CODE>
                <ACCT_ARRAY>
                    <Row>
                        <LEDGER_BAL>{cls._balance:.2f}</LEDGER_BAL>
                    </Row>
                </ACCT_ARRAY>
                <LEDGER_BAL>{cls._balance:.2f}</LEDGER_BAL>
            </Body>
        </Reply_Msg>
        """
        return ET.ElementTree(ET.fromstring(response))

class DbHelper:
    """Simulates Database operations."""
    
    @staticmethod
    def execute_sql(sql):
        print(f"Executing SQL: {sql}")
        # MOCK: Always return 1 as expected by the test cases
        return 1
