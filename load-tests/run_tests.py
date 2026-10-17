"""Run experiments one at a time. Usage: .venv/bin/python load-tests/run_tests.py HOST PREFIX"""
import datetime
import json
import os
from pathlib import Path
import subprocess
import sys

host, prefix = sys.argv[1:3]
experiments = [
    ("http-50", "ProductHttpUser", 50, 10, "0.1"),
    ("fast-50", "ProductFastHttpUser", 50, 10, "0.1"),
    ("stress-1000", "ProductFastHttpUser", 1000, 100, "0"),
]
timeline = []
for name, user, users, spawn, wait in experiments:
    result = f"results/{prefix}-{name}"
    start = datetime.datetime.now(datetime.timezone.utc).isoformat()
    command = [sys.executable, "-m", "locust", "-f", "load-tests/locustfile.py", user,
               "--headless", "--host", host, "-u", str(users), "-r", str(spawn),
               "-t", "60s", "--csv", result, "--html", result + ".html"]
    print(f"Starting {name}: {users} users, spawn {spawn}/s, wait {wait}s", flush=True)
    with open(result + ".log", "w") as output:
        completed = subprocess.run(command, env=dict(os.environ, WAIT_SECONDS=wait),
                                   stdout=output, stderr=subprocess.STDOUT)
    timeline.append(dict(name=name, client=user, users=users, spawn_rate=spawn,
                         wait_seconds=wait, host=host, start=start,
                         end=datetime.datetime.now(datetime.timezone.utc).isoformat(),
                         exit_code=completed.returncode))
    Path(f"results/{prefix}-timeline.json").write_text(json.dumps(timeline, indent=2))
    print(f"Finished {name}: exit {completed.returncode}", flush=True)
