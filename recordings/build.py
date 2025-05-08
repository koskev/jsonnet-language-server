import subprocess
import pathlib
import multiprocessing

from typing import Tuple

SCRIPT_MAP= {
    "forloop.sh": "../pkg/server/testdata/complete/forobj.jsonnet",
    "functionbody.sh": "../pkg/server/testdata/complete/functionbody.jsonnet",
    "rename.sh": "../pkg/server/testdata/complete/function/multilintecall.jsonnet"
}

OUT_DIR = pathlib.Path("out")


def build(d: Tuple[str,str]):
    key = d[0]
    val = d[1]
    print(f"Compiling {key}...")
    
    outName = OUT_DIR / f"{key}.out"
    cmd = ["asciinema-automation", "--wait", "0", "--standard-deviation", "0", "-d", key, outName, "--asciinema-arguments", f"-c \"nvim --cmd 'set noswapfile' {val}\""]
    subprocess.run(cmd)
    subprocess.run(["agg", outName, f"{outName}.gif"])

if __name__ == "__main__":
    print(f"{SCRIPT_MAP}")
    OUT_DIR.mkdir(exist_ok=True)
    with multiprocessing.Pool() as pool:
        pool.map(build, SCRIPT_MAP.items())
