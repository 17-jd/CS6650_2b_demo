"""Exercise every specified response against go run, Docker, or ECS.
Run: python3 tests/check_api.py http://localhost:8080 --faults
Only use --faults against a server started with ENABLE_TEST_ERRORS=true.
"""
import argparse
import json
import os
import urllib.request
import urllib.error

parser = argparse.ArgumentParser()
parser.add_argument("host")
parser.add_argument("--faults", action="store_true")
args = parser.parse_args()
product = dict(product_id=12345, sku="ABC-123-XYZ", manufacturer="Acme Corporation",
               category_id=456, weight=1250, some_other_id=789)
cases = [
    ("GET", "/products/12345", None, 200, False),
    ("GET", "/products/999", None, 404, False),
    ("POST", "/products/12345/details", product, 204, False),
    ("POST", "/products/12345/details", {}, 400, False),
    ("POST", "/products/999/details", dict(product, product_id=999), 404, False),
]
if args.faults:
    cases += [("GET", "/products/12345", None, 500, True),
              ("POST", "/products/12345/details", product, 500, True)]
for method, path, body, expected, fault in cases:
    headers = {"X-API-Key": os.getenv("PRODUCT_API_KEY", "hw5-local-demo"),
               "Content-Type": "application/json"}
    if fault:
        headers["X-Test-Error"] = "500"
    data = None if body is None else json.dumps(body).encode()
    request = urllib.request.Request(args.host.rstrip("/") + path, data=data,
                                     headers=headers, method=method)
    try:
        response = urllib.request.urlopen(request, timeout=10)
    except urllib.error.HTTPError as error:
        response = error
    text = response.read().decode()
    print(f"{method} {path}: expected {expected}, received {response.status} {text}")
    assert response.status == expected
    if expected == 204:
        assert not text
    if expected >= 400:
        assert {"error", "message"} <= json.loads(text).keys()
print("PASS: all requested responses matched")
