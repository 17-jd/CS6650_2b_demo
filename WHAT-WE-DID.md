# Homework 5, in simple words

## The 30-second version

1. **Go + Gin:** we made two API endpoints to read and update a product.
2. **Map + lock:** we stored products in memory and prevented requests from changing the map at the same time.
3. **Docker:** we packaged the server into a container.
4. **Terraform + AWS:** we deployed that container to ECS and stored its image in ECR.
5. **Locust:** we sent many requests and measured speed, delay, and failures.
6. **Prometheus + Grafana:** we collected and displayed request rate, errors, and response time locally.
7. **JSON logs:** we saved a clear record of individual requests.

The remaining sections explain each step when you want more detail.

## 1. We read the API contract

`api.yaml` tells us what a request and response must look like. We implemented only its Product section.

- GET `/products/12345` asks for one product. Success returns JSON and status 200.
- POST `/products/12345/details` changes that product's details. Success returns status 204 and no body.
- 400 means the POST request contains invalid data.
- 404 means the product does not exist.
- 500 means something went wrong inside the server.

The file also requires authentication. We use its `X-API-Key` option. The classroom demo key is `hw5-local-demo`; a missing or incorrect key returns 401.

There is a small ambiguity: the assignment calls this “creating” products, but the YAML describes adding/updating details and lists a 404 response. We follow that contract by starting with products 1 and 12345, updating those, and returning 404 for other IDs. There is no extra create endpoint.

## 2. We wrote one small Go server

Read `src/product-service.go` in this order:

1. **Product:** lists the six fields required by the YAML. Weight uses a pointer because zero is allowed, but leaving the field out is not allowed.
2. **Store / newStore:** creates the map and its two sample products.
3. **getProduct:** reads an ID, looks in the map, and sends the product or 404.
4. **updateProduct:** checks the ID and JSON, checks field limits, and updates an existing product.
5. **observe:** counts requests and errors, measures time, and prints a JSON log.
6. **newRouter:** connects URL paths to the functions. It also adds the API key check and a handler for unexpected errors.
7. **main:** starts the HTTP server on port 8080.

The path through the code is:

**Request → metrics timer → error protection → API key check → GET/POST function → locked map → response → metrics and JSON log.**

## 3. We protected shared memory

A map gives fast lookup by product ID, usually O(1). Many requests can read at the same time with `RLock`. A write takes `Lock`, so readers and other writers wait until the update is finished. Without the lock, overlapping reads and writes could cause data races or crash the program.

This map is temporary: restarting the process restores the sample data. Two ECS tasks would have separate maps. We keep one task for this assignment. A real store would use shared, persistent storage.

## 4. We added Docker and Terraform

Docker packages the Go server so it can run the same way on different machines. The Dockerfile first compiles Go, then copies only the program into a small runtime image.

Terraform describes the AWS resources we want. It works out how to create or update them:

- `provider.tf`: connects Terraform to AWS and Docker.
- `variables.tf`: settings such as region, port, and allowed client IP addresses.
- `main.tf`: connects the modules, builds the image, and pushes it to ECR.
- `modules/network`: finds the default VPC and subnets and creates a security group.
- `modules/ecr`: creates the container image repository.
- `modules/logging`: creates the CloudWatch log group.
- `modules/ecs`: creates the cluster, task definition, and service that run the container.
- `outputs.tf`: prints useful resource names.

We adapted the instructor's starter rather than writing a different infrastructure project. We changed the server, names, region, image build architecture, and allowed IP configuration. ECS waits until the image is pushed before starting the service. It uses the image digest (its exact content ID), so future image changes also trigger an ECS deployment.

“Declarative” means describing the desired result: “run one task with this image.” An imperative program would list each operation: “create a cluster, register a task, start a service.” Terraform compares the desired state with the recorded state and plans the necessary changes.

## 5. We test responses and performance separately

`tests/check_api.py` sends real HTTP requests and verifies the required response codes. The Go tests also check input edge cases and concurrent reads/writes with Go's race detector.

To demonstrate a 500 safely and repeatably, start a local server with `ENABLE_TEST_ERRORS=true`, then send the header `X-Test-Error: 500`. It intentionally triggers the error handler. The deployed server keeps this feature off. A test-induced 500 is not evidence of a spontaneous server failure.

Locust creates pretend users. Our normal workload is about 80% reads and 20% updates because customers usually browse products more often than product details change. We run the same tasks with HttpUser and FastHttpUser, then increase users and remove the wait between requests for stress testing.

Do not confuse Locust users with requests per second. A user sends the next request only after the previous task and wait finish. More users can increase latency without increasing throughput once something is saturated.

## 6. We observe the server

Prometheus collects measurements from `/metrics` every five seconds. Grafana turns them into graphs:

- **Rate:** how many requests arrive per second.
- **Errors:** what percent of responses are not 2xx.
- **Duration:** how long requests take. p95 means about 95% completed within that time.

The dashboard's duration is measured inside the server. Locust also includes the network and time waiting for the response, so the numbers need not match. Prometheus percentiles are estimates from histogram buckets.

Use the dashboard to see trends and when the system became busy or slow. Use the JSON logs to inspect individual requests and their status codes after a problem occurs.

## 7. How a larger store could work (design only)

Put a load balancer/API gateway in front of separate Product, Cart, Inventory, Order, and Payment services. Use a persistent product database and cache frequent product reads. Keep inventory changes atomic to prevent overselling. Give payment and checkout requests idempotency keys so retries do not charge twice. Use a queue for shipping and other work that can finish asynchronously, with retries and a dead-letter queue. Track metrics, logs, and request IDs across services.

This is a design discussion for the interview; those services are outside the implementation scope.

## Quick interview explanation

“I built a Product API with Gin. Products are stored in a map protected by a read/write mutex. I validate the JSON against the supplied schema. I package the server with Docker and adapt the instructor's Terraform modules for ECS and ECR. I test every required response code, compare two Locust clients, and watch rate, errors, and latency in local Grafana. JSON logs let me inspect individual requests. The in-memory store resets on restart, so a production version would need a database.”

For the actual completed runs and deployment status, see `README.md` and the files under `results/`.
