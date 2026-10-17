"""Same tasks for both clients: 80% reads, 20% updates.
Set WAIT_SECONDS=0 for stress; default is 0.1 seconds between tasks.
"""
import os
from locust import HttpUser, FastHttpUser, constant

class ProductTasks:
    abstract = True
    wait_time = constant(float(os.getenv("WAIT_SECONDS", "0.1")))

    def on_start(self):
        self.headers = {"X-API-Key": os.getenv("PRODUCT_API_KEY", "hw5-local-demo")}

    def read_product(self):
        self.client.get("/products/12345", headers=self.headers,
                        name="GET /products/{id}")

    def update_product(self):
        self.client.post("/products/12345/details", headers=self.headers,
                         json={"product_id": 12345, "sku": "ABC-123-XYZ",
                               "manufacturer": "Acme Corporation", "category_id": 456,
                               "weight": 1250, "some_other_id": 789},
                         name="POST /products/{id}/details")

    tasks = [read_product] * 4 + [update_product]

class ProductHttpUser(ProductTasks, HttpUser):
    abstract = False

class ProductFastHttpUser(ProductTasks, FastHttpUser):
    abstract = False
    # Fail the same way as HttpUser on very long requests.
    network_timeout = 10
    connection_timeout = 10
