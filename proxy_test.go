package main

import (
	"strings"
	"testing"
)

func TestBuildLuxpowerCommandAddsProxyEnv(t *testing.T) {
	original := luxpowerProxyURL
	defer func() { luxpowerProxyURL = original }()

	luxpowerProxyURL = "http://localhost:8888"

	cmd := buildLuxpowerCommand()
	if len(cmd.Env) == 0 {
		t.Fatalf("expected command env to be set when proxy configured")
	}

	var httpProxy, httpsProxy string
	for _, env := range cmd.Env {
		if strings.HasPrefix(env, "HTTP_PROXY=") {
			httpProxy = strings.TrimPrefix(env, "HTTP_PROXY=")
		}
		if strings.HasPrefix(env, "HTTPS_PROXY=") {
			httpsProxy = strings.TrimPrefix(env, "HTTPS_PROXY=")
		}
	}

	if httpProxy != luxpowerProxyURL {
		t.Fatalf("expected HTTP_PROXY %q, got %q", luxpowerProxyURL, httpProxy)
	}
	if httpsProxy != luxpowerProxyURL {
		t.Fatalf("expected HTTPS_PROXY %q, got %q", luxpowerProxyURL, httpsProxy)
	}
}

func TestBuildLuxpowerCommandWithoutProxyLeavesEnvUntouched(t *testing.T) {
	original := luxpowerProxyURL
	defer func() { luxpowerProxyURL = original }()

	luxpowerProxyURL = ""

	cmd := buildLuxpowerCommand()
	for _, env := range cmd.Env {
		if strings.HasPrefix(env, "HTTP_PROXY=") || strings.HasPrefix(env, "HTTPS_PROXY=") {
			t.Fatalf("unexpected proxy variables present: %q", env)
		}
	}
}
