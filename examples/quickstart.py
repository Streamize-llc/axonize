"""Axonize Quick Start — minimal example to get tracing working."""

import axonize

# 1. Initialize the SDK
axonize.init(
    endpoint="localhost:4317",
    service_name="my-inference-service",
    environment="development",
)

# 2. Trace an inference operation
with axonize.span("image-generation") as s:
    s.set_attribute("ai.model.name", "stable-diffusion-xl")
    s.set_attribute("batch_size", 4)
    s.set_gpus(["cuda:0"])

    # Nested span for sub-operations
    with axonize.span("preprocessing") as child:
        child.set_attribute("step", "tokenize")
        # ... your preprocessing code ...

    with axonize.span("diffusion-loop") as child:
        child.set_attribute("steps", 30)
        # ... your diffusion code ...

# 3. Shutdown (flushes remaining spans)
axonize.shutdown()

print("Done! Check the Axonize dashboard at http://localhost:3000")
