package config

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/ph0m1/porta/encoding"
)

const (
	BracketsRouterPatternBuilder = iota
	ColonRouterPatternBuilder
)

const (
	GET    string = "GET"
	POST   string = "POST"
	PUT    string = "PUT"
	DELETE string = "DELETE"
	NONE   string = ""
)

var RoutingPattern = ColonRouterPatternBuilder

type HTTPMethod string

// ServiceConfig defines the service
type ServiceConfig struct {
	// set of endpoint definitions
	Endpoints []*EndpointConfig `mapstructure:"endpoints"`
	// default timeout
	Timeout time.Duration `mapstructure:"timeout"`
	// default TTL for GET
	CacheTTL time.Duration `mapstructure:"cache_ttl"`
	// default set of hosts
	Host []string `mapstructure:"host"`
	// port to bind service
	Port int `mapstructure:"port"`
	// version code of the configuration
	Version int `mapstructure:"version"`

	// run in Debug Mode
	Debug bool
}

// EndpointConfig defines the configuration of a single endpoint to be exposed by service
type EndpointConfig struct {
	// url pattern to be registered and exposed to the world
	Endpoint string `mapstructure:"endpoint"`
	// HTTP method of the endpoint (GET, POST, PUT, etc)
	Method string `mapstructure:"method"`
	// set of definitions of the backends to be linked to this endpoint
	Backend []*Backend `mapstructure:"backend"`
	// number of concurrent calls this endpoint must send to the backends
	ConcurrentCalls int `mapstructure:"concurrent_calls"`
	// timeout of this endpoint
	Timeout time.Duration `mapstructure:"timeout"`
	// duration of cache header
	CacheTTL time.Duration `mapstructure:"cache_ttl"`
	// list of query string params to be extracted from the URI
	QueryString []string `mapstructure:"querystring_params"`
}

// Backend defines how to connect to the backend service and how to process the received response
type Backend struct {
	// the name of the group the response should be moved to
	Group string `mapstructure:"group"`
	// HTTP method of the request to send to the backend
	Method string `mapstructure:"method"`
	// Set of hosts of the API
	Host []string `mapstructure:"host"`
	// URL pattern to use to locate the resource to be consumed
	URLPattern string `mapstructure:"url_pattern"`
	// set of response fields to remove
	Blacklist []string `mapstructure:"blacklist"`
	// set of response fields to allow
	Whitelist []string `mapstructure:"whitelist"`
	// map of response fields to renamed and their new names
	Mapping map[string]string `mapstructure:"mapping"`
	// the encoding format
	Encoding string `mapstructure:"encoding"`
	// name of the field to extract to the root
	Target string `mapstructure:"target"`

	// list of keys to be replaced in the URLPattern
	URLKeys []string
	// number of concurrent calls this endpoint must send to the API
	ConcurrentCalls int
	// timeout of this backend
	Timeout time.Duration
	// decoder to use in order to parse the received response from the API
	Decoder encoding.Decoder
}

var (
	simpleURLKeysPattern   = regexp.MustCompile(`\{([a-zA-Z\-_0-9]+)\}`)
	endpointURLKeysPattern = regexp.MustCompile(`/\{([a-zA-Z\-_0-9]+)\}`)
	errInvalidHost         = errors.New("invalid host")
	hostPattern            = regexp.MustCompile(`(https?://)?([a-zA-Z0-9\._\-]+)(:[0-9]{2,6})?/?`)
	debugPattern           = "^[^/]|/__debug(/.*)?$"
	defaultPort            = 8080
)

func (s *ServiceConfig) Init() error {
	if s.Version != 1 {
		return fmt.Errorf("Unsupported version: %d\n", s.Version)
	}
	if s.Port == 0 {
		s.Port = defaultPort
	}
	s.Host = s.cleanHosts(s.Host)
	for i, e := range s.Endpoints {
		e.Endpoint = s.cleanPath(e.Endpoint)

		if err := e.validate(); err != nil {
			return err
		}

		inputParams := s.extractPlaceHoldersFromURLTemplate(e.Endpoint, endpointURLKeysPattern)
		inputSet := map[string]interface{}{}
		for ip := range inputParams {
			inputSet[inputParams[ip]] = nil
		}
		e.Endpoint = s.getEndpointPath(e.Endpoint, inputParams)

		s.initEndpointDefaults(i)

		for j, b := range e.Backend {
			s.initBackendDefaults(i, j)
			b.Method = strings.ToTitle(b.Method)

			if err := s.initBackendURLMappings(i, j, inputSet); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *ServiceConfig) extractPlaceHoldersFromURLTemplate(subject string, pattern *regexp.Regexp) []string {
	matches := pattern.FindAllStringSubmatch(subject, -1)
	keys := make([]string, len(matches))
	for k, v := range matches {
		keys[k] = v[1]
	}
	return keys
}

func (s *ServiceConfig) initEndpointDefaults(e int) {
	endpoint := s.Endpoints[e]
	if endpoint.Method == NONE {
		endpoint.Method = GET
	} else {
		endpoint.Method = strings.ToTitle(endpoint.Method)
	}
	if s.CacheTTL != 0 && endpoint.CacheTTL == 0 {
		endpoint.CacheTTL = s.CacheTTL
	}
	if s.Timeout != 0 && endpoint.Timeout == 0 {
		endpoint.Timeout = s.Timeout
	}
	if endpoint.ConcurrentCalls == 0 {
		endpoint.ConcurrentCalls = 1
	}
}

func (s *ServiceConfig) initBackendDefaults(e, b int) {
	endpoint := s.Endpoints[e]
	backend := endpoint.Backend[b]
	if len(backend.Host) == 0 {
		backend.Host = s.Host
	} else {
		backend.Host = s.cleanHosts(backend.Host)
	}
	if backend.Method == NONE {
		backend.Method = endpoint.Method
	}
	backend.Timeout = endpoint.Timeout
	backend.ConcurrentCalls = endpoint.ConcurrentCalls

	switch strings.ToLower(backend.Encoding) {
	case "xml":
		backend.Decoder = encoding.XMLDecoder
	case "json":
		backend.Decoder = encoding.JSONDecoder
	case "toml":
		backend.Decoder = encoding.TOMLDecoder
	case "yaml":
		backend.Decoder = encoding.YAMLDecoder
	default:
		backend.Decoder = encoding.YAMLDecoder
	}
}

func (s *ServiceConfig) initBackendURLMappings(e, b int, inputParams map[string]interface{}) error {
	backend := s.Endpoints[e].Backend[b]
	backend.URLPattern = s.cleanPath(backend.URLPattern)

	outputParams := s.extractPlaceHoldersFromURLTemplate(backend.URLPattern, simpleURLKeysPattern)

	outputSet := map[string]interface{}{}
	for op := range outputParams {
		outputSet[outputParams[op]] = nil
	}

	if len(outputSet) > len(inputParams) {
		return fmt.Errorf("Too many output params! input: %v, output: %v\n", outputSet, outputParams)
	}

	tmp := backend.URLPattern
	backend.URLKeys = make([]string, len(outputParams))
	for o := range outputParams {
		if _, ok := inputParams[outputParams[o]]; !ok {
			return fmt.Errorf("Undefined output param [%s]! input: %v, output: %v\n", outputParams[o], inputParams, outputParams)
		}
		tmp = strings.Replace(tmp, "{"+outputParams[o]+"}", "{{."+strings.Title(outputParams[o])+"}}", -1)
		backend.URLKeys = append(backend.URLKeys, strings.Title(outputParams[o]))
	}
	backend.URLPattern = tmp
	return nil
}

func (s *ServiceConfig) cleanHosts(hosts []string) []string {
	cleaned := []string{}
	for i := range hosts {
		cleaned = append(cleaned, s.cleanHost(hosts[i]))
	}
	return cleaned
}

func (s *ServiceConfig) cleanHost(host string) string {
	matches := hostPattern.FindAllStringSubmatch(host, -1)
	if len(matches) != 1 {
		panic(errInvalidHost)
	}
	keys := matches[0][1:]
	if keys[0] == "" {
		keys[0] = "http://"
	}
	return strings.Join(keys, "")
}

func (s *ServiceConfig) cleanPath(path string) string {
	return "/" + strings.TrimPrefix(path, "/")
}

func (s *ServiceConfig) getEndpointPath(path string, params []string) string {
	result := path

	if RoutingPattern == ColonRouterPatternBuilder {
		for p := range params {
			result = strings.Replace(result, "/{"+params[p]+"}", "/:"+params[p], -1)
		}
	}
	return result
}

func (e *EndpointConfig) validate() error {
	matched, err := regexp.MatchString(debugPattern, e.Endpoint)
	if err != nil {
		log.Printf("ERROR: parsing the endpoint url [%s]: %s. Ignoring\n", e.Endpoint, err.Error())
		return err
	}
	if matched {
		return fmt.Errorf("ERROR: the endpoint url path [%s] is not a valid one!!! Ignoring\n", e.Endpoint)
	}

	if len(e.Backend) == 0 {
		return fmt.Errorf("WARNING: the [%s] endpoint has 0 backends defined! Ignoring\n", e.Endpoint)
	}

	return nil
}
