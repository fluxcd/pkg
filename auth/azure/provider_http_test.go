/*
Copyright 2026 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package azure_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/azure"
)

type registryRoundTripper func(*http.Request) (*http.Response, error)

func (f registryRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestProvider_RegistryHTTPClient(t *testing.T) {
	type contextKey struct{}
	ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), contextKey{}, "caller"), time.Second)
	defer cancel()
	exp := time.Now().Add(time.Hour).Truncate(time.Second)
	token := "eyJhbGciOiJIUzI1NiJ9." + base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, exp.Unix()))) + ".c2lnbmF0dXJl"
	calls := 0
	client := &http.Client{Timeout: 30 * time.Second, Transport: registryRoundTripper(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.Method != http.MethodPost || req.URL.Scheme != "https" || req.URL.Host != "test.azurecr.io:8443" || req.URL.Path != "/oauth2/exchange" {
			t.Fatalf("unexpected exchange request: %s %s", req.Method, req.URL)
		}
		if req.Context().Value(contextKey{}) != "caller" {
			t.Fatal("request lost caller context")
		}
		deadline, ok := req.Context().Deadline()
		wantDeadline, _ := ctx.Deadline()
		if !ok || !deadline.Equal(wantDeadline) {
			t.Fatal("request lost caller deadline")
		}
		if err := req.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if req.Form.Get("grant_type") != "access_token" || req.Form.Get("service") != "test.azurecr.io" || req.Form.Get("access_token") != "access-token" {
			t.Fatalf("unexpected exchange form: %v", req.Form)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"refresh_token":%q}`, token))), Request: req}, nil
	})}
	creds, err := (azure.Provider{}).NewArtifactRegistryCredentials(ctx, "test.azurecr.io:8443", &azure.Token{AccessToken: azcore.AccessToken{Token: "access-token"}},
		auth.WithArtifactRegistryHTTPClient(client), auth.WithProxyURL(url.URL{Scheme: "http", Host: "unused.invalid:8080"}))
	if err != nil {
		t.Fatal(err)
	}
	config, err := creds.Authenticator.Authorization()
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || config.Username != "00000000-0000-0000-0000-000000000000" || config.Password != token || !creds.ExpiresAt.Equal(exp) {
		t.Fatalf("unexpected exchange result: calls=%d, expires=%v", calls, creds.ExpiresAt)
	}
	if client.Timeout != 30*time.Second {
		t.Fatal("caller client was modified")
	}
}

func TestProvider_RegistryHTTPClientRedirect(t *testing.T) {
	var targetCalls atomic.Int32
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetCalls.Add(1); w.WriteHeader(http.StatusOK) }))
	defer target.Close()
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	blocked := errors.New("redirect is outside the allowed registry")
	client := source.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return blocked }
	ctx := runtime.WithRetryOptions(context.Background(), policy.RetryOptions{MaxRetries: -1})
	_, err := (azure.Provider{}).NewArtifactRegistryCredentials(ctx, strings.TrimPrefix(source.URL, "https://"), &azure.Token{}, auth.WithArtifactRegistryHTTPClient(client))
	if !errors.Is(err, blocked) {
		t.Fatalf("expected caller redirect error, got %v", err)
	}
	if got := targetCalls.Load(); got != 0 {
		t.Fatalf("redirect target was contacted %d times", got)
	}
}

func TestProvider_RegistryHTTPClientProxy(t *testing.T) {
	var proxyConnections atomic.Int32
	proxy := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadGateway) }))
	proxy.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			proxyConnections.Add(1)
		}
	}
	proxy.Start()
	defer proxy.Close()
	proxyURL, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	blocked := errors.New("proxy address is not allowed")
	var dialAttempts atomic.Int32
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyURL(proxyURL)
	transport.DialContext = func(_ context.Context, _, address string) (net.Conn, error) {
		dialAttempts.Add(1)
		if address != proxyURL.Host {
			t.Errorf("dialed %s, expected proxy %s", address, proxyURL.Host)
		}
		return nil, blocked
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	ctx := runtime.WithRetryOptions(context.Background(), policy.RetryOptions{MaxRetries: -1})
	_, err = (azure.Provider{}).NewArtifactRegistryCredentials(ctx, "test.azurecr.io", &azure.Token{}, auth.WithArtifactRegistryHTTPClient(client))
	if !errors.Is(err, blocked) {
		t.Fatalf("expected caller proxy error, got %v", err)
	}
	if dialAttempts.Load() == 0 || proxyConnections.Load() != 0 {
		t.Fatalf("dial attempts=%d, proxy connections=%d", dialAttempts.Load(), proxyConnections.Load())
	}
}

func TestProvider_RegistryHTTPClientCancellation(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(fmt.Sprintf("deadline=%t", deadline), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			want := error(context.Canceled)
			if deadline {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
				want = context.DeadlineExceeded
			}
			defer cancel()
			calls := 0
			client := &http.Client{Transport: registryRoundTripper(func(req *http.Request) (*http.Response, error) {
				calls++
				if !deadline {
					cancel()
				}
				<-req.Context().Done()
				return nil, req.Context().Err()
			})}
			_, err := (azure.Provider{}).NewArtifactRegistryCredentials(ctx, "test.azurecr.io", &azure.Token{}, auth.WithArtifactRegistryHTTPClient(client))
			if !errors.Is(err, want) || calls != 1 {
				t.Fatalf("calls=%d, error=%v, expected %v", calls, err, want)
			}
		})
	}
}

func TestProvider_RegistryHTTPClientPreservesManagedIdentity(t *testing.T) {
	var identityCalls atomic.Int32
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identityCalls.Add(1)
		if r.Header.Get("X-IDENTITY-HEADER") != "identity-header" {
			t.Error("missing identity header")
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"access_token": "managed-token", "expires_on": fmt.Sprint(time.Now().Add(time.Hour).Unix()), "token_type": "Bearer", "resource": "https://management.azure.com"}); err != nil {
			t.Error(err)
		}
	}))
	defer identity.Close()
	for _, key := range []string{"AZURE_CLIENT_ID", "AZURE_CLIENT_SECRET", "AZURE_TENANT_ID", "AZURE_FEDERATED_TOKEN_FILE", "AZURE_AUTHORITY_HOST", "MSI_ENDPOINT", "MSI_SECRET", "IDENTITY_SERVER_THUMBPRINT"} {
		t.Setenv(key, "")
	}
	t.Setenv("AZURE_CLIENT_ID", "00000000-0000-0000-0000-000000000001")
	t.Setenv("IDENTITY_ENDPOINT", identity.URL)
	t.Setenv("IDENTITY_HEADER", "identity-header")
	registryCalls := 0
	client := &http.Client{Transport: registryRoundTripper(func(*http.Request) (*http.Response, error) {
		registryCalls++
		return nil, errors.New("registry transport cannot contact identity service")
	})}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	token, err := (azure.Provider{}).NewControllerToken(ctx, auth.WithScopes("https://management.azure.com/.default"), auth.WithArtifactRegistryHTTPClient(client))
	if err != nil {
		t.Fatal(err)
	}
	if token.(*azure.Token).Token != "managed-token" || identityCalls.Load() != 1 || registryCalls != 0 {
		t.Fatalf("identity calls=%d, registry calls=%d", identityCalls.Load(), registryCalls)
	}
}

func TestProvider_RegistryHTTPClientInvalidResponse(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"refresh_token":null}`,
		`{"refresh_token":""}`,
		`{"refresh_token":"invalid"}`,
		`{"refresh_token":"eyJhbGciOiJIUzI1NiJ9.e30.c2lnbmF0dXJl"}`,
	} {
		t.Run(body, func(t *testing.T) {
			client := &http.Client{Transport: registryRoundTripper(func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
			})}
			creds, err := (azure.Provider{}).NewArtifactRegistryCredentials(context.Background(), "test.azurecr.io", &azure.Token{}, auth.WithArtifactRegistryHTTPClient(client))
			if err == nil || creds != nil {
				t.Fatalf("expected invalid response error, got credentials=%v, error=%v", creds, err)
			}
		})
	}
}

func TestProvider_RegistryHTTPClientTimeout(t *testing.T) {
	client := &http.Client{Timeout: 20 * time.Millisecond, Transport: registryRoundTripper(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})}
	ctx := runtime.WithRetryOptions(context.Background(), policy.RetryOptions{MaxRetries: -1})
	_, err := (azure.Provider{}).NewArtifactRegistryCredentials(ctx, "test.azurecr.io", &azure.Token{}, auth.WithArtifactRegistryHTTPClient(client))
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("expected caller client timeout, got %v", err)
	}
}

func TestProvider_RegistryHTTPClientAllowedProxy(t *testing.T) {
	token := "eyJhbGciOiJIUzI1NiJ9." + base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, time.Now().Add(time.Hour).Unix()))) + ".c2lnbmF0dXJl"
	registry := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"refresh_token": token}); err != nil {
			t.Error(err)
		}
	}))
	defer registry.Close()
	registryHost := strings.TrimPrefix(registry.URL, "https://")
	var proxyConnections atomic.Int32
	proxyDone := make(chan struct{})
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(proxyDone)
		if r.Method != http.MethodConnect || r.Host != registryHost {
			t.Errorf("unexpected proxy request: %s %s", r.Method, r.Host)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		upstream, err := (&net.Dialer{}).DialContext(r.Context(), "tcp", registryHost)
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer upstream.Close()
		conn, rw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		if _, err := rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
			t.Error(err)
			return
		}
		if err := rw.Flush(); err != nil {
			t.Error(err)
			return
		}
		proxyConnections.Add(1)
		copied := make(chan error, 1)
		go func() { _, err := io.Copy(conn, upstream); copied <- err }()
		if _, err := io.Copy(upstream, rw); err != nil {
			t.Error(err)
		}
		if err := upstream.Close(); err != nil {
			t.Error(err)
		}
		if err := <-copied; err != nil && !errors.Is(err, net.ErrClosed) {
			t.Error(err)
		}
	}))
	defer proxy.Close()
	proxyURL, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := registry.Client()
	transport := client.Transport.(*http.Transport)
	transport.Proxy = http.ProxyURL(proxyURL)
	defer func() {
		transport.CloseIdleConnections()
		select {
		case <-proxyDone:
		case <-time.After(5 * time.Second):
			t.Error("proxy tunnel did not close")
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	creds, err := (azure.Provider{}).NewArtifactRegistryCredentials(ctx, registryHost, &azure.Token{}, auth.WithArtifactRegistryHTTPClient(client))
	if err != nil {
		t.Fatal(err)
	}
	config, err := creds.Authenticator.Authorization()
	if err != nil {
		t.Fatal(err)
	}
	if config.Password != token || proxyConnections.Load() != 1 {
		t.Fatalf("unexpected exchange result or proxy connections=%d", proxyConnections.Load())
	}
}
