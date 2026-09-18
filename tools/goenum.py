import os

import sh

suffix = ".enum.gen"

sh.go(
    [
        "tool",
        "github.com/abice/go-enum",
        "--marshal",
        "--names",
        "--values",
        "--sql",
        "--output-suffix",
        suffix,
    ],
)

path = os.environ["GOFILE"].removesuffix(".go") + suffix + ".go"
path = os.path.join(os.getcwd(), path)
print(f"go-enum: generated {path}")
