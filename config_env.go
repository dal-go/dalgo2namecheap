package namecheap

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const namecheapAPIFile = ".namecheap-api"

// ConfigFromEnv resolves NameCheap credentials from environment variables
// or from ~/.namecheap-api file (per-key priority: env var first, then file).
// Returns options for WithAPIUser and WithAPIKey only; no client-IP option is included.
func ConfigFromEnv() ([]Option, error) {
	apiUser := os.Getenv("NAMECHEAP_API_USER")
	apiKey := os.Getenv("NAMECHEAP_API_KEY")

	// If either credential is missing, try reading the file
	if apiUser == "" || apiKey == "" {
		fileVals, err := readNamecheapAPIFile()
		if err != nil {
			return nil, fmt.Errorf("namecheap: failed to read credentials file: %w", err)
		}
		if apiUser == "" {
			apiUser = fileVals["NAMECHEAP_API_USER"]
		}
		if apiKey == "" {
			apiKey = fileVals["NAMECHEAP_API_KEY"]
		}
	}

	if apiUser == "" {
		return nil, fmt.Errorf("namecheap: NAMECHEAP_API_USER is not set")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("namecheap: NAMECHEAP_API_KEY is not set")
	}

	return []Option{WithAPIUser(apiUser), WithAPIKey(apiKey)}, nil
}

// readNamecheapAPIFile reads ~/.namecheap-api and returns key-value pairs.
// Returns an error if the file exists but cannot be read.
// Returns an empty map if the file does not exist.
func readNamecheapAPIFile() (map[string]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	path := filepath.Join(home, namecheapAPIFile)

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	defer f.Close()

	result := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue // not a KEY=value line
		}
		key := line[:idx]
		val := line[idx+1:]
		// Strip optional surrounding double quotes
		if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
			val = val[1 : len(val)-1]
		}
		result[key] = val
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading credentials file: %w", err)
	}
	return result, nil
}
