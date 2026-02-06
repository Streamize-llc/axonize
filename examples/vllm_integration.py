"""Axonize + vLLM integration example.

Shows how to instrument a vLLM inference server with Axonize for
streaming token tracking, TTFT/TPOT metrics, and GPU attribution.

Requirements:
    pip install axonize vllm

Usage:
    python examples/vllm_integration.py
"""

import axonize

# Initialize Axonize with GPU profiling enabled
axonize.init(
    endpoint="localhost:4317",
    service_name="vllm-server",
    environment="production",
    gpu_profiling=True,
)


def generate_with_tracing(prompt: str, max_tokens: int = 256) -> str:
    """Example: instrument a vLLM generate call."""
    # In real code, you'd import and use vLLM:
    #   from vllm import LLM, SamplingParams
    #   llm = LLM(model="meta-llama/Llama-3-70B")

    with axonize.llm_span("vllm.generate", model="llama-3-70b") as s:
        s.set_tokens_input(len(prompt.split()))  # Approximate token count
        s.set_gpus(["cuda:0", "cuda:1"])  # Multi-GPU inference

        # Simulate streaming generation
        # In real code: for output in llm.generate(prompt, params):
        output_tokens = []
        for i in range(max_tokens):
            # Each token emission
            s.record_token()
            token = f"token_{i}"  # placeholder
            output_tokens.append(token)

            # Stop condition (simulated)
            if i >= 20:
                break

        return " ".join(output_tokens)


def batch_inference_with_tracing(prompts: list[str]) -> list[str]:
    """Example: trace a batch of inference requests."""
    results = []

    with axonize.span("batch-inference") as batch_span:
        batch_span.set_attribute("batch_size", len(prompts))
        batch_span.set_gpus(["cuda:0"])

        for i, prompt in enumerate(prompts):
            with axonize.llm_span(
                f"generate-{i}",
                model="llama-3-70b",
            ) as s:
                s.set_tokens_input(len(prompt.split()))
                # Simulate token generation
                for _ in range(15):
                    s.record_token()
                results.append(f"Response to: {prompt[:30]}...")

    return results


if __name__ == "__main__":
    # Single request
    print("=== Single Generation ===")
    result = generate_with_tracing("Explain quantum computing in simple terms")
    print(f"Generated: {result[:50]}...")

    # Batch request
    print("\n=== Batch Generation ===")
    prompts = [
        "What is machine learning?",
        "Write a haiku about GPUs",
        "Explain transformers architecture",
    ]
    results = batch_inference_with_tracing(prompts)
    for r in results:
        print(f"  {r}")

    axonize.shutdown()
    print("\nDone! Check traces at http://localhost:3000/traces")
