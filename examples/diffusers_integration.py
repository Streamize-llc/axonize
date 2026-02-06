"""Axonize + HuggingFace Diffusers integration example.

Shows how to trace image generation pipelines with GPU attribution.

Requirements:
    pip install axonize diffusers torch

Usage:
    python examples/diffusers_integration.py
"""

import axonize

# Initialize Axonize with GPU profiling
axonize.init(
    endpoint="localhost:4317",
    service_name="diffusion-service",
    environment="development",
    gpu_profiling=True,
)


def generate_image(prompt: str, num_steps: int = 30) -> None:
    """Trace a Stable Diffusion image generation."""
    # In real code:
    #   from diffusers import StableDiffusionXLPipeline
    #   pipe = StableDiffusionXLPipeline.from_pretrained("stabilityai/sdxl-1.0")
    #   pipe = pipe.to("cuda")

    with axonize.span("text-to-image") as root:
        root.set_attribute("ai.model.name", "stable-diffusion-xl")
        root.set_attribute("ai.inference.type", "diffusion")
        root.set_attribute("ai.diffusion.steps", num_steps)
        root.set_attribute("prompt", prompt[:100])
        root.set_gpus(["cuda:0"])

        # Phase 1: Text encoding
        with axonize.span("text-encoding") as enc:
            enc.set_attribute("encoder", "CLIP")
            # pipe.encode_prompt(prompt)
            pass

        # Phase 2: Diffusion loop
        with axonize.span("diffusion-loop") as diff:
            diff.set_attribute("steps", num_steps)
            diff.set_attribute("cfg_scale", 7.5)
            diff.set_attribute("scheduler", "euler")
            diff.set_gpus(["cuda:0"])

            for step in range(num_steps):
                with axonize.span(f"step-{step}") as s:
                    s.set_attribute("step", step)
                    # UNet forward pass happens here
                    pass

        # Phase 3: VAE decode
        with axonize.span("vae-decode") as vae:
            vae.set_attribute("decoder", "sdxl-vae")
            vae.set_gpus(["cuda:0"])
            # pipe.vae.decode(latents)
            pass


def generate_batch(prompts: list[str]) -> None:
    """Trace a batch of image generations."""
    with axonize.span("batch-generation") as batch:
        batch.set_attribute("batch_size", len(prompts))

        for i, prompt in enumerate(prompts):
            generate_image(prompt, num_steps=20)
            print(f"  Generated image {i+1}/{len(prompts)}")


if __name__ == "__main__":
    print("=== Diffusers + Axonize ===\n")

    # Single image
    print("Generating single image...")
    generate_image("A futuristic city with flying cars at sunset", num_steps=30)
    print("Done!")

    # Batch
    print("\nGenerating batch...")
    generate_batch([
        "A cat wearing a spacesuit on Mars",
        "Abstract art in the style of Kandinsky",
        "Photorealistic mountain landscape at dawn",
    ])

    axonize.shutdown()
    print("\nDone! Check traces at http://localhost:3000/traces")
