package initscaffold

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nspcc-dev/neo-go/pkg/nsmith/toolchain"
)

// Options for project scaffolding.
type Options struct {
	Name string
	Lang string
	Dir  string
}

// Create writes a minimal contract project for the given language.
func Create(opts Options) (string, error) {
	dir := opts.Dir
	if dir == "" {
		dir = opts.Name
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	switch opts.Lang {
	case "go":
		return dir, writeGo(dir, opts.Name)
	case "python":
		return dir, writePython(dir, opts.Name)
	case "csharp":
		return dir, writeCSharp(dir, opts.Name)
	case "java":
		return dir, writeJava(dir, opts.Name)
	default:
		return "", fmt.Errorf("unsupported language %q", opts.Lang)
	}
}

func writeGo(dir, name string) error {
	mainGo := fmt.Sprintf(`package main

import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"

// %s Neo smart contract.
func _deploy(_ any, isUpdate bool) {
	if !isUpdate {
		runtime.Log("%s deployed")
	}
}

// GetValue returns a constant for smoke tests.
func GetValue() string {
	return "ok"
}
`, name, name)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(mainGo), 0o644); err != nil {
		return err
	}
	conf := fmt.Sprintf(`name: %s
`, name)
	return os.WriteFile(filepath.Join(dir, "contract.yml"), []byte(conf), 0o644)
}

func writePython(dir, name string) error {
	contract := fmt.Sprintf(`"""%s — neo3-boa contract scaffold."""

from boa3.sc.compiletime import public


@public
def get_value() -> str:
    return "ok"
`, name)
	if err := os.WriteFile(filepath.Join(dir, "contract.py"), []byte(contract), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("neo3-boa\n"), 0o644)
}

func writeCSharp(dir, name string) error {
	csproj := fmt.Sprintf(`<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
    <Nullable>enable</Nullable>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Neo.SmartContract.Framework" Version="3.9.1" />
  </ItemGroup>
  <Target Name="PostBuild" AfterTargets="PostBuildEvent">
    <Exec Command="nccs $(ProjectPath)" />
  </Target>
</Project>
`)
	contract := fmt.Sprintf(`using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Services;

namespace %s
{
    public class Contract : SmartContract
    {
        public static void _deploy(object data, bool update)
        {
            if (!update)
            {
                Runtime.Log("%s deployed");
            }
        }

        public static string GetValue() => "ok";
    }
}
`, name, name)
	if err := os.WriteFile(filepath.Join(dir, name+".csproj"), []byte(csproj), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "Contract.cs"), []byte(contract), 0o644)
}

func writeJava(dir, name string) error {
	m, err := toolchain.NewManager()
	if err != nil {
		return err
	}
	man, err := m.LoadManifest()
	if err != nil {
		return err
	}
	pluginVer := man.Versions["java"]
	if pluginVer == "" {
		pluginVer = "3.24.0"
	}
	pkg := "io.nanvil.contract"
	className := "Contract"
	buildGradle := fmt.Sprintf(`plugins {
    id 'java'
    id 'io.neow3j.gradle-plugin' version '%s'
}

group '%s'
version '1.0-SNAPSHOT'

java {
    sourceCompatibility = JavaVersion.VERSION_1_8
    targetCompatibility = JavaVersion.VERSION_1_8
}

repositories {
    mavenCentral()
}

dependencies {
    implementation 'io.neow3j:devpack:%s'
}

neow3jCompiler {
    className = '%s.%s'
}
`, pluginVer, pkg, pluginVer, pkg, className)

	contract := fmt.Sprintf(`package %s;

import io.neow3j.devpack.annotations.DisplayName;

@DisplayName("%s")
public class %s {

    public static String getValue() {
        return "ok";
    }
}
`, pkg, name, className)

	if err := os.WriteFile(filepath.Join(dir, "build.gradle"), []byte(buildGradle), 0o644); err != nil {
		return err
	}
	srcDir := filepath.Join(dir, "src", "main", "java", "io", "nanvil", "contract")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(srcDir, className+".java"), []byte(contract), 0o644); err != nil {
		return err
	}
	settings := "rootProject.name = '" + name + "'\n"
	return os.WriteFile(filepath.Join(dir, "settings.gradle"), []byte(settings), 0o644)
}
