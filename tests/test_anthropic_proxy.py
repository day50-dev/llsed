import os
import time
import unittest
import requests

class TestAnthropicRouting(unittest.TestCase):
    """Sanity checks to ensure Anthropic API routing works."""

    def test_list_models(self):
        """Verify that the Anthropic models endpoint returns a model list."""
        api_key = os.getenv("ANTHROPIC_API_KEY")
        self.assertIsNotNone(api_key, "ANTHROPIC_API_KEY environment variable must be set")

        headers = {
            "x-api-key": api_key,
            "Content-Type": "application/json",
        }

        response = requests.get(
            "https://api.anthropic.com/v1/models",
            headers=headers,
        )
        self.assertEqual(response.status_code, 200, f"Unexpected status code: {response.status_code}")
        data = response.json()
        # Anthropic returns a list under the 'model' key or similar; adjust as needed
        self.assertTrue(data, "Response should contain model information")

    def test_haiku_3_hello(self):
        """Invoke Claude Haiku 3 model and ensure it returns the text \"hello\"."""
        api_key = os.getenv("ANTHROPIC_API_KEY")
        self.assertIsNotNone(api_key, "ANTHROPIC_API_KEY environment variable must be set")

        headers = {
            "x-api-key": api_key,
            "Content-Type": "application/json",
        }

        payload = {
            "model": "claude-3-haiku-20240307",
            "max_tokens": 10,
            "messages": [
                {
                    "role": "user",
                    "content": "respond only with the text \"hello\""
                }
            ]
        }

        response = requests.post(
            "https://api.anthropic.com/v1/chat/completions",
            json=payload,
            headers=headers,
        )
        self.assertEqual(response.status_code, 200, f"Unexpected status code: {response.status_code}")
        data = response.json()
        # Ensure the response contains a 'choices' field with the expected text
        self.assertIn("choices", data, "Response should contain 'choices'")
        returned_text = data["choices"][0]["message"]["content"].strip().lower()
        self.assertIn("hello", returned_text, f"Expected 'hello' in response, got: {returned_text}")

if __name__ == "__main__":
    unittest.main()
