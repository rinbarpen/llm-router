import os


# Providers default to curl_cffi in runtime. For tests we use httpx backend so
# existing respx mocks keep intercepting outbound HTTP calls.
os.environ.setdefault("LLM_ROUTER_HTTP_BACKEND", "httpx")
