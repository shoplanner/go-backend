import os
import subprocess

suffix = ".enum.gen.go"

subprocess.run(
    [
        "go",
        "tool",
        "github.com/abice/go-enum",
        "--marshal",
        "--names",
        "--values",
        "--sql",
        "--output-suffix",
        suffix,
    ],
    stdout=subprocess.DEVNULL,
)

path = os.environ["GOFILE"] + suffix
path = os.path.join(os.getcwd(), path)
print(f"go-enum: generated {path}")
