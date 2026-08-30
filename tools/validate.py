#!/usr/bin/env python3
"""Render every manifest combination and check the result.

This is the engine's conventions in executable form: what render() does here
is what odyssey-cli must do.
"""
import itertools
import pathlib
import re
import shutil
import subprocess
import sys
import tempfile

import yaml

ROOT = pathlib.Path(__file__).resolve().parent.parent
FRAG = ROOT / "fragments"

DUMMY = {
    "PROJECT": "demo",
    "STATE_BUCKET": "demo-tfstate",
    "AWS_REGION": "ap-southeast-1",
    "CLOUDFLARE_ACCOUNT_ID": "abc123",
    "ZONE_NAME": "example.com",
    "R2_LOCATION": "apac",
    "CUSTOM_DOMAIN": "demo.example.com",
    "WORKER_NAME": "demo",
    "PREPROD_ENV": "preprod",
    "PRD_ENV": "prd",
    "PREPROD_URL": "https://preprod.example.com",
    "PRD_URL": "https://example.com",
}

# {{ENV}} in a deploy workflow means that workflow's environment.
WORKFLOW_ENV = {
    "3-deploy.yml": "prd",
    "3-deploy-preprod.yml": "preprod",
    "4-deploy-prd.yml": "prd",
}


def subst(text, envs, env=None):
    text = text.replace("{{ENV_LIST}}", "[" + ", ".join(f'"{e}"' for e in envs) + "]")
    if env is not None:
        text = text.replace("{{ENV}}", env)
    for k, v in DUMMY.items():
        text = text.replace("{{" + k + "}}", v)
    return text


def fill_marker(text, name, body):
    """Replace the marker's body (None keeps the default), strip the marker lines."""
    pat = re.compile(rf"[ \t]*# >>> {name}\n(.*?)[ \t]*# <<< {name}\n", re.S)
    m = pat.search(text)
    if not m:
        return text
    return text[: m.start()] + (m.group(1) if body is None else body) + text[m.end() :]


def render(layout, envs, stack, arch, provider):
    out = pathlib.Path(tempfile.mkdtemp(prefix=f"odyssey-{layout}-{stack}-"))
    wf_dir = out / ".github" / "workflows"
    wf_dir.mkdir(parents=True)

    # always
    shutil.copytree(FRAG / "workflows" / "scripts", out / ".github" / "scripts")
    makefile = [(FRAG / "makefile" / "base.mk").read_text()]

    # workflows: ci + the layout's deploy set, markers filled or emptied
    ci_infra = (FRAG / "workflows" / "ci" / "infra.yml").read_text() if provider else ""
    deploy_infra = (
        (FRAG / "infra" / "providers" / provider / "workflows" / "deploy.yml").read_text()
        if provider
        else ""
    )
    sources = sorted((FRAG / "workflows" / "ci").glob("[0-9]*.yml")) + sorted(
        (FRAG / "workflows" / "deploy" / layout).glob("*.yml")
    )
    for src in sources:
        text = src.read_text()
        text = fill_marker(text, "infra", ci_infra if src.parent.name == "ci" else deploy_infra)
        text = fill_marker(text, "deploy", None)
        text = subst(text, envs, WORKFLOW_ENV.get(src.name))
        (wf_dir / src.name).write_text(text)

    # stack
    shutil.copytree(FRAG / "stacks" / stack / ".github", out / ".github", dirs_exist_ok=True)
    for extra in ("files",):
        d = FRAG / "stacks" / stack / extra
        if d.is_dir():
            shutil.copytree(d, out, dirs_exist_ok=True)
    makefile.append((FRAG / "makefile" / "stack" / stack / "main.mk").read_text())

    # architecture
    arch_dir = FRAG / "makefile" / "deploy" / arch
    makefile.append((arch_dir / "main.mk").read_text())
    if (arch_dir / "scripts").is_dir():
        shutil.copytree(arch_dir / "scripts", out / "scripts")
    if (arch_dir / "files").is_dir():
        shutil.copytree(arch_dir / "files", out, dirs_exist_ok=True)

    # infra
    if provider:
        makefile.append((FRAG / "makefile" / "infra" / "terraform" / "main.mk").read_text())
        infra = out / "infra"
        infra.mkdir()
        shutil.copy(FRAG / "infra" / ".gitignore", infra / ".gitignore")
        pdir = FRAG / "infra" / "providers" / provider
        for f in pdir.iterdir():
            if f.name == "workflows":
                continue
            if f.is_dir():
                shutil.copytree(f, infra / f.name)
            else:
                shutil.copy(f, infra / f.name)
        adir = FRAG / "infra" / "architecture" / arch
        if adir.is_dir():
            for f in adir.glob("*.tf"):
                if f.name == "variables.tf":
                    with open(infra / "variables.tf", "a") as v:
                        v.write("\n" + f.read_text())
                else:
                    shutil.copy(f, infra / f.name)
            for f in adir.glob("config/*"):
                with open(infra / "config" / f.name, "a") as c:
                    c.write("\n" + f.read_text())
        # {{ENV}} in a filename expands the file once per environment
        for f in list(infra.rglob("*{{ENV}}*")):
            text = f.read_text()
            for env in envs:
                f.with_name(f.name.replace("{{ENV}}", env)).write_text(subst(text, envs, env))
            f.unlink()
    (out / "Makefile").write_text("\n".join(makefile))

    # copy means verbatim + substitution: one pass over everything written
    for f in out.rglob("*"):
        if f.is_file():
            try:
                text = f.read_text()
            except UnicodeDecodeError:
                continue
            if "{{" in text:
                f.write_text(subst(text, envs))
    return out


def check(out, provider):
    def run(*cmd, cwd=out):
        r = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True)
        if r.returncode != 0:
            raise SystemExit(f"FAIL in {out}: {' '.join(cmd)}\n{r.stdout}{r.stderr}")

    for wf in (out / ".github" / "workflows").iterdir():
        yaml.safe_load(wf.read_text())
        assert "# >>>" not in wf.read_text(), f"unfilled marker in {wf}"
        infra_calls = wf.read_text().count("make infra ENV")
        assert infra_calls <= 1, f"duplicated make infra in {wf}"
        if not provider:
            for word in ("terraform", "make infra"):
                assert word not in wf.read_text(), f"infra residue in {wf}"

    # odyssey tokens are {{UPPER}}; ${{ }} is GitHub's own interpolation
    token = re.compile(r"(?<!\$)\{\{[A-Z0-9_]+\}\}")
    for f in out.rglob("*"):
        if f.is_file():
            try:
                m = token.search(f.read_text())
            except UnicodeDecodeError:
                continue
            assert not m, f"unsubstituted token {m.group()} in {f}"

    for sh in out.rglob("*.sh"):
        run("bash", "-n", str(sh))

    run("make", "-n", "ci")
    run("make", "-n", "deploy")
    if provider:
        run("make", "-n", "infra", "ENV=prd")
        run("terraform", "-chdir=infra", "fmt", "-check", "-recursive")
        run("terraform", "-chdir=infra", "init", "-backend=false", "-input=false")
        run("terraform", "-chdir=infra", "validate")


def main():
    manifest = yaml.safe_load((ROOT / "manifest.yml").read_text())
    combos = []
    for stack, archs in manifest["stacks"].items():
        for arch in archs:
            spec = manifest["architectures"][arch]
            combos.append((stack, arch, spec["provider"]))
            if spec.get("infra") == "optional":
                combos.append((stack, arch, None))

    for (layout, envs), (stack, arch, provider) in itertools.product(
        manifest["environments"].items(), combos
    ):
        out = render(layout, envs, stack, arch, provider)
        check(out, provider)
        print(f"ok  {layout:6} {stack:15} {arch:17} {provider or '-'}")
        shutil.rmtree(out)

    print("all combinations render and validate")


if __name__ == "__main__":
    main()
