"""Axonize + Ollama integration example.

Shows how to trace Ollama API calls with streaming token tracking.

Requirements:
    pip install axonize requests

Usage:
    # Start Ollama: ollama serve
    # Pull a model: ollama pull llama3
    python examples/ollama_integration.py
"""

from __future__ import annotations

import json

import axonize

# Initialize Axonize
axonize.init(
    endpoint="localhost:4317",
    service_name="ollama-app",
    environment="development",
)


def chat_with_ollama(
    prompt: str,
    model: str = "llama3",
    base_url: str = "http://localhost:11434",
) -> str:
    """Chat with Ollama and trace the interaction."""
    import requests

    with axonize.llm_span("ollama.chat", model=model) as s:
        s.set_tokens_input(len(prompt.split()))  # Approximate

        # Streaming request to Ollama
        response = requests.post(
            f"{base_url}/api/generate",
            json={"model": model, "prompt": prompt, "stream": True},
            stream=True,
            timeout=60,
        )
        response.raise_for_status()

        full_response = []
        for line in response.iter_lines():
            if not line:
                continue
            data = json.loads(line)
            if "response" in data:
                s.record_token()
                full_response.append(data["response"])
            if data.get("done", False):
                # Ollama provides token counts
                if "prompt_eval_count" in data:
                    s.set_tokens_input(data["prompt_eval_count"])
                break

        return "".join(full_response)


def multi_turn_conversation(messages: list[str], model: str = "llama3") -> None:
    """Trace a multi-turn conversation."""
    with axonize.span("conversation") as conv:
        conv.set_attribute("turns", len(messages))
        conv.set_attribute("ai.model.name", model)

        for i, msg in enumerate(messages):
            response = chat_with_ollama(msg, model=model)
            print(f"  Turn {i+1}: {response[:80]}...")


if __name__ == "__main__":
    print("=== Ollama + Axonize ===")
    print("Note: Requires Ollama running locally with a model pulled.\n")

    try:
        # Single query
        result = chat_with_ollama("What is the capital of France?")
        print(f"Response: {result[:100]}...")

        # Multi-turn
        print("\n=== Multi-turn ===")
        multi_turn_conversation([
            "Tell me about GPU architecture",
            "How does it relate to AI training?",
        ])
    except Exception as e:
        print(f"Ollama not available: {e}")
        print("Start Ollama with: ollama serve && ollama pull llama3")

    axonize.shutdown()
    print("\nDone! Check traces at http://localhost:3000/traces")
