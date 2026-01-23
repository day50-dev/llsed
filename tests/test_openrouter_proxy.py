import os
import subprocess
import time
import unittest
import requests

class TestOpenRouterProxy(unittest.TestCase):
    """Sanity checks for the llsed proxy when routing to OpenRouter."""

    @classmethod
    def setUpClass(cls):
        # Start the llsed proxy server in a subprocess.
        # Use the existing config file (can be empty) and point to OpenRouter's API endpoint.
        cls.server_process = subprocess.Popen(
            [
                "go", "run", "llsed.go",
                "--host", "127.0.0.1",
                "--port", "8080",
                "--map_file", "config.json",
                "--server", "https://openrouter.ai/api/v1"
            ],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        # Give the server a moment to start.
        time.sleep(2)

    @classmethod
    def tearDownClass(cls):
        # Terminate the proxy server.
        cls.server_process.terminate()
        cls.server_process.wait()

    def test_gemini_flash(self):
        """Invoke the Gemini flash model via the proxy and verify a successful response."""
        api_key = os.getenv("OPENROUTER_API_KEY")
        self.assertIsNotNone(api_key, "OPENROUTER_API_KEY environment variable must be set")

        headers = {
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json",
        }

        payload = {
            "model": "google/gemini-2.0-flash-exp:free",
            "max_tokens": 10,
            "messages": [
                {
                    "role": "user",
                    "content": "respond only with the text \"hello\""
                }
            ]
        }

        response = requests.post(
            "http://127.0.0.1:8080/v1/chat/completions",
            json=payload,
            headers=headers,
        )
        self.assertEqual(response.status_code, 200,
                         f"Unexpected status code from proxy: {response.status_code}")
        data = response.json()
        # Ensure the response contains a 'choices' field.
        self.assertIn("choices", data, "Response should contain 'choices'")
        # Verify that the returned content includes the word 'hello'.
        returned_text = data["choices"][0]["message"]["content"].strip().lower()
        self.assertIn("hello", returned_text,
                      f"Expected 'hello' in response, got: {returned_text}")

if __name__ == "__main__":
    unittest.main()
