package mock

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// MockDefinition defines a mock server from YAML.
type MockDefinition struct {
	Name      string  `yaml:"name"`
	Namespace string  `yaml:"namespace"`
	Port      int     `yaml:"port"`
	Routes    []Route `yaml:"routes"`
}

// Route defines a single mock route.
type Route struct {
	Method   string        `yaml:"method"`
	Path     string        `yaml:"path"`
	Response RouteResponse `yaml:"response"`
}

// RouteResponse defines the response for a mock route.
type RouteResponse struct {
	Status  int               `yaml:"status"`
	Headers map[string]string `yaml:"headers"`
	Body    string            `yaml:"body"`
	Delay   string            `yaml:"delay"` // e.g., "500ms"
}

// LoadDefinition reads a mock definition YAML file.
func LoadDefinition(path string) (*MockDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading mock file: %w", err)
	}
	var def MockDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parsing mock file: %w", err)
	}
	if def.Name == "" {
		return nil, fmt.Errorf("mock name is required")
	}
	if def.Port == 0 {
		def.Port = 8080
	}
	if def.Namespace == "" {
		def.Namespace = "default"
	}
	return &def, nil
}

type jsonRoute struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
	DelayMs int               `json:"delay_ms"`
}

// GenerateRoutesJSON serializes the mock routes to a JSON string
// suitable for embedding in the Python server script.
func GenerateRoutesJSON(def *MockDefinition) (string, error) {
	var routes []jsonRoute
	for _, r := range def.Routes {
		delayMs := 0
		if r.Response.Delay != "" {
			if d, err := time.ParseDuration(r.Response.Delay); err == nil {
				delayMs = int(d.Milliseconds())
			}
		}
		status := r.Response.Status
		if status == 0 {
			status = 200
		}
		routes = append(routes, jsonRoute{
			Method:  strings.ToUpper(r.Method),
			Path:    r.Path,
			Status:  status,
			Headers: r.Response.Headers,
			Body:    r.Response.Body,
			DelayMs: delayMs,
		})
	}

	data, err := json.Marshal(routes)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GeneratePythonServer generates a Python script that serves the mock routes.
func GeneratePythonServer(def *MockDefinition) string {
	routesJSON, _ := GenerateRoutesJSON(def)

	return fmt.Sprintf(`import json, http.server, time, re

ROUTES = json.loads('%s')
PORT = %d

class MockHandler(http.server.BaseHTTPRequestHandler):
    def do_request(self):
        for route in ROUTES:
            method = route.get('method', '*')
            path = route.get('path', '/')
            if method != '*' and method != self.command:
                continue
            pattern = re.sub(r':(\w+)', r'(?P<\1>[^/]+)', path)
            if re.fullmatch(pattern, self.path.split('?')[0]):
                delay = route.get('delay_ms', 0)
                if delay > 0:
                    time.sleep(delay / 1000.0)
                status = route.get('status', 200)
                self.send_response(status)
                for k, v in route.get('headers', {}).items():
                    self.send_header(k, v)
                if 'Content-Type' not in route.get('headers', {}):
                    self.send_header('Content-Type', 'application/json')
                self.end_headers()
                body = route.get('body', '')
                self.wfile.write(body.encode())
                return
        self.send_response(404)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(b'{"error": "no matching route"}')

    def do_GET(self): self.do_request()
    def do_POST(self): self.do_request()
    def do_PUT(self): self.do_request()
    def do_DELETE(self): self.do_request()
    def do_PATCH(self): self.do_request()
    def log_message(self, format, *args): pass

print(f'Mock server listening on port {PORT}')
http.server.HTTPServer(('', PORT), MockHandler).serve_forever()
`, strings.ReplaceAll(routesJSON, `'`, `\'`), def.Port)
}

// MockInfo contains information about a running mock.
type MockInfo struct {
	Name      string
	Namespace string
	Port      int
	Routes    int
	PodName   string
}
