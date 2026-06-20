# nsmith — multi-language contract compiler

`nsmith` is Nanvil’s Foundry-`forge`-style compiler suite. It compiles Neo N3 smart contracts written in **Go**, **Python**, **Java**, or **C#** into `.nef` and `.manifest.json` artifacts for deployment with `ncast deploy`.

| Tool | Role |
|------|------|
| `nanvil` | Local dev node (Anvil) |
| `ncast` | RPC client / deploy (Cast) |
| **`nsmith`** | Compile contracts (Forge) |

## Quick start

```bash
make build          # builds nanvil, ncast, and nsmith
./bin/nsmith compile ./contract.go --name MyContract
./bin/ncast deploy --wif <wif> -i MyContract.nef -m MyContract.manifest.json
```

Compile all four supported languages from the repo examples:

```bash
./scripts/test-nsmith-examples.sh
```

## Repository examples

Ready-to-compile sample contracts live under `integration/testcontracts/examples/`:

| Language | Path | Smoke-test return value |
|----------|------|-------------------------|
| Go | `examples/go/` | `"nsmith-go-ok"` |
| Python | `examples/python/` | `"nsmith-python-ok"` |
| C# | `examples/csharp/` | `"nsmith-csharp-ok"` |
| Java | `examples/java/` | `"nsmith-java-ok"` |

```bash
# Go (no extra toolchain install)
./bin/nsmith compile --lang go --out /tmp/go integration/testcontracts/examples/go

# Python (requires: nsmith install --lang python)
./bin/nsmith compile --lang python --out /tmp/py integration/testcontracts/examples/python

# C# (requires: nsmith install --lang csharp, dotnet SDK)
./bin/nsmith compile --lang csharp --out /tmp/cs integration/testcontracts/examples/csharp

# Java (requires: JDK 17+; uses ./gradlew in the example project)
./bin/nsmith compile --lang java --out /tmp/java integration/testcontracts/examples/java
```

The Java example includes a Gradle wrapper. Projects from `nsmith init --lang java` need `gradle wrapper` (or copy the wrapper from the example).

## Supported languages

| Language | Compiler | Host requirements |
|----------|----------|-------------------|
| Go | Embedded `pkg/compiler` | None (ships with nsmith) |
| Python | [neo3-boa](https://github.com/CityOfZion/neo3-boa) | Python 3.13+ |
| C# | [Neo.Compiler.CSharp](https://www.nuget.org/packages/Neo.Compiler.CSharp) (`nccs`) | .NET SDK (and matching runtime for `nccs`) |
| Java | [neow3j](https://github.com/neow3j/neow3j) Gradle plugin | JDK 17+ and `./gradlew` or system `gradle` |

Go contracts use the same compiler as neo-go. Other languages are invoked via managed toolchains under `~/.nanvil/toolchains/` (override with `NANVIL_TOOLCHAINS`).

`nsmith` auto-detects `DOTNET_ROOT` and `JAVA_HOME` when common system paths are present (for example `/usr/share/dotnet` and `/usr/lib/jvm/java-17-openjdk`).

## Commands

### `nsmith compile [path]`

Auto-detects language from project markers (`.go` + interop imports, `neo3-boa`, `.csproj`, `build.gradle` with neow3j).

**Put flags before the path** — `nsmith compile --lang python ./contract.py` (not `./contract.py --lang python`).

```bash
nsmith compile ./contracts/main.go
nsmith compile --lang python ./contract.py
nsmith compile --out ./build/MyContract --config contract.yml ./main.go
nsmith compile --json ./main.go
```

Flags:

- `--lang` — force `go`, `python`, `java`, or `csharp`
- `--out` — output prefix (without extension); absolute paths write outside the source tree
- `--name` — manifest contract name
- `--config` — Go contract YAML (`contract.yml`)
- `--debug` — emit debug artifacts where supported

### `nsmith build [path]`

Same as `compile`, with optional deploy:

```bash
nsmith build ./main.go --deploy --wif <wif> --rpc http://127.0.0.1:8545
```

### `nsmith init <name> --lang <lang>`

Scaffold a minimal project:

```bash
nsmith init MyToken --lang go
nsmith init PyToken --lang python
nsmith init CsToken --lang csharp
nsmith init JavaToken --lang java
```

Java scaffolds include `build.gradle` with `neow3jCompiler { className = ... }`. Run `gradle wrapper` in the project directory (or copy from `integration/testcontracts/examples/java/`).

### Toolchain management

```bash
nsmith install --lang python      # pin + install neo3-boa in managed venv
nsmith install --all
nsmith update --lang python       # refresh to latest PyPI version
nsmith toolchain list             # installed vs latest versions
nsmith doctor --all               # check host runtimes and installs
```

Versions are **pinned on install** and only change when you run `nsmith update`.

## Toolchain cache

```
~/.nanvil/toolchains/
  manifest.json
  python/venv/          # neo3-boa
  dotnet/tools/         # nccs
  java/                 # version pins for neow3j plugin
```

## Typical workflows

### Go (no extra install)

```bash
nsmith compile ./main.go --config contract.yml --out ./out/contract
ncast deploy --wif $WIF -i ./out/contract.nef -m ./out/contract.manifest.json
```

### Python

```bash
nsmith install --lang python
nsmith init Hello --lang python
cd Hello
nsmith compile --lang python contract.py
```

Python scaffolds use neo3-boa 1.5+ (`from boa3.sc.compiletime import public`).

### C#

```bash
nsmith install --lang csharp
nsmith init Token --lang csharp
cd Token
nsmith compile --lang csharp .
```

`nsmith compile` invokes `nccs` on the contract `.cs` file. You can pass a project directory or `.csproj`; artifacts land in `bin/sc/` beside the source.

### Java

```bash
nsmith init Token --lang java
cd Token
gradle wrapper    # once, if not copied from repo example
nsmith compile --lang java .
```

Gradle runs `neow3jCompile`; output is under `build/neow3j/`.

## Integration tests

Optional Python integration test (requires Python 3.13+ and network for PyPI):

```bash
NSMITH_INTEGRATION=1 go test -tags=nsmith_integration ./pkg/nsmith/compiler/...
```

Compile all language examples in CI or locally:

```bash
./scripts/test-nsmith-examples.sh
```
