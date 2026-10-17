"""Find the current public IP of the single ECS task. Run from terraform/."""
import json
import subprocess

def aws(*args):
    return json.loads(subprocess.check_output(["aws", *args, "--region", "us-east-1", "--output", "json"]))

cluster = subprocess.check_output(["terraform", "output", "-raw", "ecs_cluster_name"], text=True)
service = subprocess.check_output(["terraform", "output", "-raw", "ecs_service_name"], text=True)
tasks = aws("ecs", "list-tasks", "--cluster", cluster, "--service-name", service)["taskArns"]
if not tasks:
    raise SystemExit("No running task yet. Check ECS service events.")
task = aws("ecs", "describe-tasks", "--cluster", cluster, "--tasks", tasks[0])["tasks"][0]
details = {item["name"]: item["value"] for attachment in task["attachments"] for item in attachment["details"]}
network = aws("ec2", "describe-network-interfaces", "--network-interface-ids", details["networkInterfaceId"])
ip = network["NetworkInterfaces"][0]["Association"]["PublicIp"]
print(f"http://{ip}:8080")
