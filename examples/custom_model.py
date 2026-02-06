"""Axonize with a custom model — general-purpose integration pattern.

Shows how to instrument any inference pipeline (PyTorch, TensorFlow,
ONNX Runtime, etc.) with Axonize tracing and GPU attribution.

Requirements:
    pip install axonize

Usage:
    python examples/custom_model.py
"""

import time

import axonize

axonize.init(
    endpoint="localhost:4317",
    service_name="custom-model-service",
    environment="development",
    gpu_profiling=True,
)


def run_inference(input_data: dict) -> dict:
    """Instrument a custom inference pipeline."""
    with axonize.span("inference") as root:
        root.set_attribute("ai.model.name", "my-custom-model")
        root.set_attribute("ai.model.version", "v2.1")
        root.set_attribute("ai.inference.type", "classification")
        root.set_gpus(["cuda:0"])

        # Preprocessing
        with axonize.span("preprocess") as pre:
            pre.set_attribute("input_size", len(str(input_data)))
            processed = {"tensor": [1, 2, 3]}  # Your preprocessing
            time.sleep(0.001)

        # Model forward pass
        with axonize.span("forward") as fwd:
            fwd.set_attribute("batch_size", 1)
            fwd.set_gpus(["cuda:0"])
            result = {"class": "cat", "confidence": 0.95}  # Your model
            time.sleep(0.005)

        # Postprocessing
        with axonize.span("postprocess") as post:
            post.set_attribute("output_class", result["class"])
            time.sleep(0.001)

        # Cost tracking
        root.set_attribute("cost.usd", 0.001)

        return result


def run_streaming_llm(prompt: str) -> str:
    """Instrument a custom streaming LLM with llm_span."""
    with axonize.llm_span(
        "custom-llm-generate",
        model="my-llm-v3",
        model_version="3.0",
    ) as s:
        s.set_tokens_input(len(prompt.split()))
        s.set_gpus(["cuda:0"])

        # Simulate streaming token generation
        tokens = []
        for i in range(50):
            s.record_token()
            tokens.append(f"word{i}")
            time.sleep(0.002)

        return " ".join(tokens)


if __name__ == "__main__":
    print("=== Custom Model + Axonize ===\n")

    # Classification inference
    result = run_inference({"image": "cat.jpg"})
    print(f"Classification: {result}")

    # Streaming LLM
    output = run_streaming_llm("Tell me about GPU monitoring")
    print(f"LLM output: {output[:60]}...")

    axonize.shutdown()
    print("\nDone! Check traces at http://localhost:3000/traces")
