# Troubleshooting

This guide covers common issues you might encounter when using Axonize and how to resolve them.

## Installation Issues

### Python Version Compatibility

**Error**: `ImportError` or syntax errors when importing `axonize`

**Solution**:
- Axonize requires Python 3.10 or higher
- Check your version: `python --version`
- Use a compatible Python version or virtual environment

### NVIDIA GPU Support Installation

**Error**: `ModuleNotFoundError: No module named 'pynvml'`

**Solution**:
```bash
pip install axonize[nvidia]
```

This installs the optional NVIDIA pynvml dependency required for NVIDIA GPU profiling.

### Apple Silicon GPU Support

**Issue**: No GPU metrics showing on M1/M2/M3/M4 Mac

**Solution**:
- Apple Silicon support is built-in (no extra dependencies needed)
- Use device label `mps:0` instead of `cuda:0`
- Enable GPU profiling: `gpu_profiling=True` in `axonize.init()`

Example:
```python
axonize.init(
    endpoint="localhost:4317",
    service_name="my-service",
    api_key=os.getenv("AXONIZE_API_KEY"),
    gpu_profiling=True,
)

with axonize.span("inference") as s:
    s.set_gpus(["mps:0"])  # Apple Silicon device label
```

---

## Server Connection Issues

### Connection Refused on Port 4317

**Error**: `grpc._channel._InactiveRpcError: Connection refused` or `Failed to connect to localhost:4317`

**Solution**:

1. **Check if server is running**:
   ```bash
   docker compose ps
   ```
   You should see `axonize-server` with status "Up"

2. **Start the server if not running**:
   ```bash
   docker compose up -d
   make migrate
   ```

3. **Verify port 4317 is listening**:
   ```bash
   lsof -i :4317
   # or
   netstat -an | grep 4317
   ```

4. **Check firewall settings**: Ensure port 4317 is not blocked by your firewall

5. **Test with health check**:
   ```bash
   curl http://localhost:8080/healthz
   # Should return: {"status":"ok"}
   ```

### Wrong Endpoint Configuration

**Error**: Connection timeout or "No route to host"

**Solution**:
- For local development, use `endpoint="localhost:4317"`
- For Docker containers, use service name: `endpoint="axonize-server:4317"`
- For remote servers, use full address: `endpoint="your-server.com:4317"`

Example:
```python
# Running SDK locally, server in Docker
axonize.init(endpoint="localhost:4317", ...)

# Running SDK in Docker container
axonize.init(endpoint="axonize-server:4317", ...)
```

---

## Authentication Errors

### Authentication Failed

**Error**: `grpc.StatusCode.UNAUTHENTICATED` or "Authentication failed"

**Solution**:

1. **Check server API key** (look in server logs):
   ```bash
   docker compose logs axonize-server | grep "API key"
   # Output: Generated API key: axon_...
   ```

2. **Set API key in SDK**:
   ```python
   import os
   import axonize

   axonize.init(
       endpoint="localhost:4317",
       service_name="my-service",
       api_key=os.getenv("AXONIZE_API_KEY"),  # Required!
   )
   ```

3. **Set environment variable**:
   ```bash
   export AXONIZE_API_KEY=axon_your_key_here
   ```

4. **Or create `.env` file** in your project root:
   ```bash
   AXONIZE_API_KEY=axon_your_key_here
   ```

### Missing API Key in SDK

**Error**: "Missing required argument: 'api_key'" or authentication fails silently

**Solution**:
- The SDK `api_key` parameter is required when the server has `AXONIZE_API_KEY` set (default behavior)
- Always include `api_key=os.getenv("AXONIZE_API_KEY")` in `axonize.init()`

### API Key Mismatch

**Error**: Authentication works initially, then fails after restart

**Solution**:
- Server generates a new API key on first startup if not set in `.env`
- To persist the same key across restarts, set `AXONIZE_API_KEY` in `.env` file before starting:
  ```bash
  # In .env file
  AXONIZE_API_KEY=your-static-key
  ```

---

## Database Issues

### Migrations Failed

**Error**: `ERROR: relation "physical_gpus" does not exist` or similar

**Solution**:

1. **Apply migrations manually**:
   ```bash
   make migrate
   ```

2. **Check database connectivity**:
   ```bash
   docker compose exec postgres psql -U axonize -d axonize -c "SELECT 1"
   ```

3. **Re-run migrations if databases were reset**:
   ```bash
   docker compose down -v  # Remove volumes
   docker compose up -d
   make migrate
   ```

---

## Dashboard Issues

The dashboard lives in a separate repository: [axonize-web](https://github.com/Streamize-llc/axonize-web). See its README for setup and troubleshooting.

### Dashboard Shows "No Traces Found"

**Issue**: Dashboard loads but shows empty state

**Solution**:
1. **Verify server is receiving traces**: Check server logs for "flushed spans"
   ```bash
   docker compose logs axonize-server | grep "flushed"
   ```

2. **Check SDK is sending traces**:
   ```python
   # Add debug logging to SDK code
   import logging
   logging.basicConfig(level=logging.DEBUG)
   ```

3. **Verify database has data**:
   ```bash
   docker compose exec postgres psql -U axonize -d axonize -c \
     "SELECT count(*) FROM spans"
   ```

4. **Check API is accessible**:
   ```bash
   curl -H "Authorization: Bearer $AXONIZE_API_KEY" \
     http://localhost:8080/api/v1/traces
   ```

---

## GPU Profiling Issues

### No GPU Metrics Collected

**Issue**: Traces show up but without GPU metrics

**Solution**:

1. **Enable GPU profiling in SDK**:
   ```python
   axonize.init(
       endpoint="localhost:4317",
       service_name="my-service",
       api_key=os.getenv("AXONIZE_API_KEY"),
       gpu_profiling=True,  # Must be True
   )
   ```

2. **Call `set_gpus()` on spans**:
   ```python
   with axonize.span("inference") as s:
       s.set_gpus(["cuda:0"])  # Attach GPU to this span
       # Your inference code
   ```

3. **Check GPU backend is detected**: Look for SDK startup logs
   - NVIDIA: "Initialized NVIDIA GPU profiler"
   - Apple Silicon: "Initialized Apple GPU profiler"
   - Neither: "GPU profiling disabled (no backend available)"

### Incorrect GPU Device Labels

**Issue**: `set_gpus(["cuda:0"])` fails or shows wrong GPU

**Solution**:
- **NVIDIA**: Use `cuda:0`, `cuda:1`, etc. (matches PyTorch/CUDA device index)
- **Apple Silicon**: Use `mps:0` (Metal Performance Shaders)
- Check available GPUs in your environment:
  ```python
  # NVIDIA
  import torch
  print(torch.cuda.device_count())

  # Apple Silicon
  import torch
  print(torch.backends.mps.is_available())
  ```

### pynvml Errors on NVIDIA

**Error**: `NVMLError_LibraryNotFound` or `NVMLError_DriverNotLoaded`

**Solution**:
- **Check NVIDIA drivers are installed**: `nvidia-smi`
- If `nvidia-smi` works but pynvml doesn't, reinstall:
  ```bash
  pip uninstall pynvml
  pip install axonize[nvidia]
  ```
- If running in Docker, ensure NVIDIA Docker runtime is configured

---

## Performance Issues

### High Memory Usage

**Issue**: SDK consuming excessive memory

**Solution**:

1. **Reduce buffer size**:
   ```python
   axonize.init(
       endpoint="localhost:4317",
       service_name="my-service",
       api_key=os.getenv("AXONIZE_API_KEY"),
       buffer_size=2048,  # Default: 8192
   )
   ```

2. **Reduce batch size**:
   ```python
   axonize.init(
       ...,
       batch_size=256,  # Default: 512
   )
   ```

3. **Increase flush frequency**:
   ```python
   axonize.init(
       ...,
       flush_interval_ms=2000,  # Default: 5000 (flush more often)
   )
   ```

### SDK Blocking Application

**Issue**: Application pauses or slows down during tracing

**Solution**:
- The SDK uses a lock-free ring buffer, so blocking should be rare
- If buffer is full, spans are dropped (not blocking by default)
- Check for error logs indicating dropped spans
- Increase `buffer_size` or `flush_interval_ms`:
  ```python
  axonize.init(
      ...,
      buffer_size=16384,  # Larger buffer
      flush_interval_ms=3000,  # Flush more frequently
  )
  ```

### High Network Traffic

**Issue**: Too much data being sent to server

**Solution**:

Use sampling to reduce data volume:
```python
axonize.init(
    endpoint="localhost:4317",
    service_name="my-service",
    api_key=os.getenv("AXONIZE_API_KEY"),
    sampling_rate=0.1,  # Keep only 10% of spans
)
```

---

## Multi-Tenant Issues

### Admin API Returns 401 Unauthorized

**Error**: "Unauthorized" when calling `/api/v1/admin/*` endpoints

**Solution**:

1. **Check auth mode is multi-tenant**:
   ```bash
   # In .env or environment
   AXONIZE_AUTH_MODE=multi_tenant
   ```

2. **Set admin key**:
   ```bash
   # In .env
   AXONIZE_ADMIN_KEY=your-admin-secret
   ```

3. **Use admin key in requests**:
   ```bash
   curl -H "Authorization: Bearer your-admin-secret" \
     http://localhost:8080/api/v1/admin/tenants
   ```

### Tenant API Key Not Working

**Issue**: Created API key doesn't authenticate

**Solution**:
1. **Verify key was created successfully**: Check response from POST `/api/v1/admin/tenants/{id}/keys`
2. **Use full key with prefix**: `axon_...` (not just the hash)
3. **Check tenant_id matches**: API keys are scoped to specific tenants
4. **Verify auth_mode is multi_tenant**: Static mode ignores API keys table

---

## Common Development Mistakes

### Forgetting to Call `shutdown()`

**Issue**: Last few spans not appearing in dashboard

**Solution**:
```python
import axonize

axonize.init(...)

# Your code here

axonize.shutdown()  # Always call this before exiting!
```

Or use `atexit` (automatic):
```python
# shutdown() is automatically registered with atexit
# So it's called on normal program exit
```

### Creating Spans Before `init()`

**Error**: `RuntimeError: SDK not initialized` or silent failure

**Solution**:
```python
import axonize

# WRONG: Creating span before init
# with axonize.span("test"):  # ERROR!
#     pass

# CORRECT: Initialize first
axonize.init(endpoint="localhost:4317", ...)

with axonize.span("test"):
    pass
```

### Not Setting `service_name`

**Issue**: All traces show up with same generic service name

**Solution**:
- Always set a meaningful `service_name`:
  ```python
  axonize.init(
      endpoint="localhost:4317",
      service_name="my-inference-api",  # Specific name!
      api_key=os.getenv("AXONIZE_API_KEY"),
  )
  ```

---

## Getting Help

If you're still experiencing issues:

1. **Check server logs**:
   ```bash
   docker compose logs -f axonize-server
   ```

2. **Enable SDK debug logging**:
   ```python
   import logging
   logging.basicConfig(level=logging.DEBUG)
   ```

3. **Check health endpoints**:
   ```bash
   # Server health
   curl http://localhost:8080/healthz

   # Database readiness
   curl http://localhost:8080/readyz
   ```

4. **Report issues**: [GitHub Issues](https://github.com/Streamize-llc/axonize/issues)

Include in your report:
- Error message and full stack trace
- SDK version: `pip show axonize`
- Server version: `docker compose logs axonize-server | head -20`
- Environment: OS, Python version, GPU type
- Steps to reproduce
