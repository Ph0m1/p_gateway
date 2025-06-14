package config

import (
	"strings"
	"testing"
	"time"
)

func TestConfig_rejectInvalidVersion(t *testing.T) {
	subject := ServiceConfig{}
	err := subject.Init()
	if err == nil || strings.Index(err.Error(), "Unsupported version: 0") != 0 {
		t.Error("Error expected. Got", err.Error())
	}
}

func TestConfig_rejectInvalidEndpoints(t *testing.T) {
	samples := []string{
		"/__debug",
		"/__debug/",
		"/__debug/foo",
		"/__debug/foo/bar",
	}

	for _, e := range samples {
		subject := ServiceConfig{
			Version:   1,
			Endpoints: []*EndpointConfig{&EndpointConfig{Endpoint: e}},
		}
		err := subject.Init()
		if err == nil || !strings.HasPrefix(err.Error(), "ERROR: the endpoint url path [") {
			t.Error("Error expected processing", e)
		}
	}
}

func TestConfig_cleanHosts(t *testing.T) {
	samples := []string{
		"supu",
		"127.0.0.1",
		"https://supu.local/",
		"http://127.0.0.1",
		"supu_42.local:8080/",
		"http://127.0.0.1:8080",
	}

	expected := []string{
		"http://supu",
		"http://127.0.0.1",
		"https://supu.local",
		"http://127.0.0.1",
		"http://supu_42.local:8080",
		"http://127.0.0.1:8080",
	}

	subject := ServiceConfig{}
	result := subject.cleanHosts(samples)
	for i := range result {
		if expected[i] != result[i] {
			t.Errorf("want: %s, have: %s\n", expected[i], result[i])
		}
	}
}

func TestConfig_cleanPath(t *testing.T) {
	samples := []string{
		"supu/{tupu}",
		"/supu/{tupu}",
		"/supu.local/",
		"supu_supu.txt",
		"supu_42.local?a=8080",
		"supu/supu/supu?a=1&b=2",
		"debug/supu/supu?a=1&b=2",
	}

	expected := []string{
		"/supu/{tupu}",
		"/supu/{tupu}",
		"/supu.local/",
		"/supu_supu.txt",
		"/supu_42.local?a=8080",
		"/supu/supu/supu?a=1&b=2",
		"/debug/supu/supu?a=1&b=2",
	}

	subject := ServiceConfig{}

	for i := range samples {
		if have := subject.cleanPath(samples[i]); expected[i] != have {
			t.Errorf("want: %s, have: %s\n", expected[i], have)
		}
	}
}

func TestConfig_getEndpointPath(t *testing.T) {
	samples := []string{
		"supu/{tupu}",
		"/supu/{tupu}",
		"/supu.local/",
		"supu/{tupu}/{supu}?a={s}&b=2",
	}

	expected := []string{
		"supu/:tupu",
		"/supu/:tupu",
		"/supu.local/",
		"supu/:tupu/:supu?a={s}&b=2",
	}

	subject := ServiceConfig{}

	for i := range samples {
		params := subject.extractPlaceHoldersFromURLTemplate(samples[i], endpointURLKeysPattern)
		if have := subject.getEndpointPath(samples[i], params); expected[i] != have {
			t.Errorf("want: %s, have: %s\n", expected[i], have)
		}
	}
}

func TestConfig_initBackendURLMappings_ok(t *testing.T) {
	samples := []string{
		"supu/{tupu}",
		"/supu/{tupu1}",
		"/supu.local/",
		"/supu/{tupu_56}/{supu-5t6}?a={foo}&b={foo}",
	}

	expected := []string{
		"/supu/{{.Tupu}}",
		"/supu/{{.Tupu1}}",
		"/supu.local/",
		"/supu/{{.Tupu_56}}/{{.Supu-5t6}}?a={{.Foo}}&b={{.Foo}}",
	}

	backend := Backend{}
	endpoint := EndpointConfig{Backend: []*Backend{&backend}}
	subject := ServiceConfig{Endpoints: []*EndpointConfig{&endpoint}}

	inputSet := map[string]interface{}{
		"tupu":     nil,
		"tupu1":    nil,
		"tupu_56":  nil,
		"supu-5t6": nil,
		"foo":      nil,
	}

	for i := range samples {
		backend.URLPattern = samples[i]
		if err := subject.initBackendURLMappings(0, 0, inputSet); err != nil {
			t.Error(err)
		}
		if backend.URLPattern != expected[i] {
			t.Errorf("want: %s, have: %s\n", expected[i], backend.URLPattern)
		}
	}
}

func TestConfig_initBackendURLMappings_tooManyOutput(t *testing.T) {
	backend := Backend{URLPattern: "/supu/{tupu_56}/{supu-5t6}?a={foo}&b={foo}"}
	endpoint := EndpointConfig{Backend: []*Backend{&backend}}
	subject := ServiceConfig{Endpoints: []*EndpointConfig{&endpoint}}

	inputSet := map[string]interface{}{
		"tupu": nil,
	}

	err := subject.initBackendURLMappings(0, 0, inputSet)
	if err == nil || !strings.HasPrefix(err.Error(), "Too many output params!") {
		t.Errorf("Error expected: %v\n", err.Error())
	}
}

func TestConfig_initBackendURLMappings_undefinedOutput(t *testing.T) {
	backend := Backend{URLPattern: "/supu/{tupu_56}/{supu-5t6}?a={foo}&b={foo}"}
	endpoint := EndpointConfig{Backend: []*Backend{&backend}}
	subject := ServiceConfig{Endpoints: []*EndpointConfig{&endpoint}}

	inputSet := map[string]interface{}{
		"tupu": nil,
		"supu": nil,
		"foo":  nil,
	}

	err := subject.initBackendURLMappings(0, 0, inputSet)
	if err == nil || !strings.HasPrefix(err.Error(), "Undefined output param [") {
		t.Errorf("Error expected: %v\n", err.Error())
	}
}

func TestConfig_init(t *testing.T) {
	supuBackend := Backend{
		URLPattern: "/__debug/supu",
	}
	supuEndpoint := EndpointConfig{
		Endpoint: "/supu",
		Method:   "post",
		Timeout:  1500 * time.Millisecond,
		CacheTTL: 6 * time.Hour,
		Backend:  []*Backend{&supuBackend},
	}

	githubBackend := Backend{
		URLPattern: "/",
		Host:       []string{"https://api.github.com"},
		Whitelist:  []string{"authorizations_url", "code_search_url"},
	}
	githubEndpoint := EndpointConfig{
		Endpoint: "/github",
		Timeout:  1500 * time.Millisecond,
		CacheTTL: 6 * time.Hour,
		Backend:  []*Backend{&githubBackend},
	}

	userBackend := Backend{
		URLPattern: "/users/{user}",
		Host:       []string{"https://jsonplaceholder.typicode.com"},
		Mapping:    map[string]string{"email": "personal_email"},
	}
	postBackend := Backend{
		URLPattern: "/posts/{user}",
		Host:       []string{"https://jsonplaceholder.typicode.com"},
		Group:      "posts",
		Encoding:   "xml",
	}
	userEndpoint := EndpointConfig{
		Endpoint: "/users/{user}",
		Backend:  []*Backend{&userBackend, &postBackend},
	}

	subject := ServiceConfig{
		Version:   1,
		Timeout:   5 * time.Second,
		CacheTTL:  30 * time.Minute,
		Host:      []string{"http://127.0.0.1:8080"},
		Endpoints: []*EndpointConfig{&supuEndpoint, &githubEndpoint, &userEndpoint},
	}

	if err := subject.Init(); err != nil {
		t.Error("Error at the configuration init:", err.Error())
	}

	if len(supuBackend.Host) != 1 || supuBackend.Host[0] != subject.Host[0] {
		t.Error("Default hosts not applied to the supu backend", supuBackend.Host)
	}

	for level, method := range map[string]string{
		"userBackend":  userBackend.Method,
		"postBackend":  postBackend.Method,
		"userEndpoint": userEndpoint.Method,
	} {
		if method != "GET" {
			t.Errorf("Default method not applied at %s. Get: %s", level, method)
		}
	}

	if supuBackend.Method != "POST" {
		t.Error("supuBackend method not sanitized")
	}

	if userBackend.Timeout != subject.Timeout {
		t.Error("default timeout not applied to the userBackend")
	}

	if userEndpoint.CacheTTL != subject.CacheTTL {
		t.Error("default CacheTTL not applied to the userEndpoint")
	}
}

func TestConfig_initKONoBackends(t *testing.T) {
	subject := ServiceConfig{
		Version: 1,
		Host:    []string{"http://127.0.0.1:8080"},
		Endpoints: []*EndpointConfig{
			&EndpointConfig{
				Endpoint: "/supu",
				Method:   "post",
				Backend:  []*Backend{},
			},
		},
	}

	if err := subject.Init(); err == nil || !strings.HasPrefix(err.Error(), "WARNING: the [/supu] endpoint has 0 backends defined! Ignoring") {
		t.Error("Error expected at the configuration init", err)
	}
}

func TestConfig_initKOInvalidHost(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("The init process did not panic with an invalid host: %v", r)
		}
	}()
	subject := ServiceConfig{
		Version: 1,
		Host:    []string{"http://127.0.0.1:8080", "http://127.0.0.1:8080"},
		Endpoints: []*EndpointConfig{
			&EndpointConfig{
				Endpoint: "/supu",
				Method:   "post",
				Backend:  []*Backend{},
			},
		},
	}
	subject.Init()
}

func TestConfig_initKOInvalidDebugPattern(t *testing.T) {
	dp := debugPattern
	debugPattern = "a(b"
	subject := ServiceConfig{
		Version: 1,
		Host:    []string{"http://127.0.0.1:8080"},
		Endpoints: []*EndpointConfig{
			&EndpointConfig{
				Endpoint: "/__debug/supu",
				Method:   "get",
				Backend:  []*Backend{},
			},
		},
	}

	if err := subject.Init(); err == nil || !strings.HasPrefix(err.Error(), "error parsing regexp: missing closing ): `a(b`") {
		t.Error("Error expected at the configuration init!", err)
	}

	debugPattern = dp
}
