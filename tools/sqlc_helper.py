import os
import shutil
import subprocess


shutil.copy2(os.path.join(os.environ["PROJECT_ROOT"], "config", "sqlc.yaml"), ".")
subprocess.run(["go", "tool", "github.com/sqlc-dev/sqlc/cmd/sqlc", "generate"])
print(f"sqlc: generated {os.path.join(os.getcwd(), 'sqlgen')}")
os.remove("sqlc.yaml")
