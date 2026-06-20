package toolchain

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Resolver resolves latest toolchain package versions.
type Resolver struct {
	Client *http.Client
}

func NewResolver() *Resolver {
	return &Resolver{Client: &http.Client{Timeout: 30 * time.Second}}
}

// LatestNeo3Boa returns the latest neo3-boa version from PyPI.
func (r *Resolver) LatestNeo3Boa(ctx context.Context) (string, error) {
	return r.pypiLatest(ctx, "neo3-boa")
}

// LatestNCCS returns the latest Neo.Compiler.CSharp version from NuGet.
func (r *Resolver) LatestNCCS(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.nuget.org/v3-flatcontainer/neo.compiler.csharp/index.json", nil)
	if err != nil {
		return "", err
	}
	resp, err := r.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("nuget: HTTP %d", resp.StatusCode)
	}
	var out struct {
		Versions []string `json:"versions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Versions) == 0 {
		return "", fmt.Errorf("nuget: no versions for Neo.Compiler.CSharp")
	}
	return out.Versions[len(out.Versions)-1], nil
}

// LatestNeow3jPlugin returns a recent neow3j Gradle plugin version from Maven Central search.
func (r *Resolver) LatestNeow3jPlugin(ctx context.Context) (string, error) {
	url := "https://search.maven.org/solrsearch/select?q=g:io.neow3j+AND+a:gradle-plugin&rows=1&wt=json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := r.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("maven: HTTP %d", resp.StatusCode)
	}
	var out struct {
		Response struct {
			Docs []struct {
				LatestVersion string `json:"latestVersion"`
			} `json:"docs"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Response.Docs) == 0 || out.Response.Docs[0].LatestVersion == "" {
		return "3.24.0", nil // fallback pin
	}
	return out.Response.Docs[0].LatestVersion, nil
}

func (r *Resolver) pypiLatest(ctx context.Context, pkg string) (string, error) {
	url := fmt.Sprintf("https://pypi.org/pypi/%s/json", pkg)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := r.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("pypi: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var out struct {
		Info struct {
			Version string `json:"version"`
		} `json:"info"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if out.Info.Version == "" {
		return "", fmt.Errorf("pypi: empty version for %s", pkg)
	}
	return strings.TrimSpace(out.Info.Version), nil
}
