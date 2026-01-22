package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type TransformRule struct {
	Tag    string                 `json:"tag"`
	From   string                 `json:"from"`
	To     string                 `json:"to"`
	Params map[string]interface{} `json:"params"`
	Pre    string                 `json:"pre"`
	Post   string                 `json:"post"`
}

type Config struct {
	Rules []TransformRule `json:"rules"`
}

type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      int         `json:"id"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
	Error   interface{} `json:"error"`
	ID      int         `json:"id"`
}

type LLMSed struct {
	config     Config
	serverURL  string
	httpClient *http.Client
}

func NewLLMSed(configPath, serverURL string) (*LLMSed, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &LLMSed{
		config:     config,
		serverURL:  serverURL,
		httpClient: &http.Client{},
	}, nil
}

func (l *LLMSed) callRPC(endpoint string, payload interface{}) (interface{}, error) {
	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "transform",
		Params:  payload,
		ID:      1,
	}

	body, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, err
	}

	resp, err := l.httpClient.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var rpcResp JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, err
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("rpc error: %v", rpcResp.Error)
	}

	return rpcResp.Result, nil
}

func (l *LLMSed) handleProxy(w http.ResponseWriter, r *http.Request) {
	// Read incoming request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// Find matching rule (simple: just use first rule for now)
	if len(l.config.Rules) == 0 {
		http.Error(w, "no transformation rules configured", http.StatusInternalServerError)
		return
	}
	rule := l.config.Rules[0]

	// Pre-transform
	if rule.Pre != "" {
		log.Printf("Calling pre-transform: %s", rule.Pre)
		result, err := l.callRPC(rule.Pre, payload)
		if err != nil {
			http.Error(w, fmt.Sprintf("pre-transform failed: %v", err), http.StatusInternalServerError)
			return
		}
		payload = result.(map[string]interface{})
	}

	// Forward to target server
	targetBody, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "failed to marshal transformed request", http.StatusInternalServerError)
		return
	}

	targetURL := l.serverURL + r.URL.Path
	log.Printf("Forwarding to: %s", targetURL)

	targetReq, err := http.NewRequest(r.Method, targetURL, bytes.NewReader(targetBody))
	if err != nil {
		http.Error(w, "failed to create target request", http.StatusInternalServerError)
		return
	}

	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			targetReq.Header.Add(key, value)
		}
	}

	targetResp, err := l.httpClient.Do(targetReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to forward request: %v", err), http.StatusBadGateway)
		return
	}
	defer targetResp.Body.Close()

	responseBody, err := io.ReadAll(targetResp.Body)
	if err != nil {
		http.Error(w, "failed to read response", http.StatusInternalServerError)
		return
	}

	var responsePayload map[string]interface{}
	if err := json.Unmarshal(responseBody, &responsePayload); err != nil {
		http.Error(w, "invalid response from target", http.StatusBadGateway)
		return
	}

	// Post-transform
	if rule.Post != "" {
		log.Printf("Calling post-transform: %s", rule.Post)
		result, err := l.callRPC(rule.Post, responsePayload)
		if err != nil {
			http.Error(w, fmt.Sprintf("post-transform failed: %v", err), http.StatusInternalServerError)
			return
		}
		responsePayload = result.(map[string]interface{})
	}

	// Send response back
	finalBody, err := json.Marshal(responsePayload)
	if err != nil {
		http.Error(w, "failed to marshal final response", http.StatusInternalServerError)
		return
	}

	// Copy response headers
	for key, values := range targetResp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(targetResp.StatusCode)
	w.Write(finalBody)
}

func main() {
	host := flag.String("host", "0.0.0.0", "Host to bind to")
	port := flag.Int("port", 8080, "Port to listen on")
	mapFile := flag.String("map_file", "config.json", "Path to mapping configuration file")
	server := flag.String("server", "https://api.openai.com", "Target server URL")
	flag.Parse()

	// Trim trailing slash from server URL
	*server = strings.TrimSuffix(*server, "/")

	llmsed, err := NewLLMSed(*mapFile, *server)
	if err != nil {
		log.Fatalf("Failed to initialize llmsed: %v", err)
	}

	addr := fmt.Sprintf("%s:%d", *host, *port)
	log.Printf("Starting llmsed on %s, proxying to %s", addr, *server)

	http.HandleFunc("/", llmsed.handleProxy)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
