import os
import subprocess

suffix = ".enum.gen"

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

path = os.environ["GOFILE"].removesuffix(".go") + suffix + ".go"
path = os.path.join(os.getcwd(), path)
print(f"go-enum: generated {path}")
