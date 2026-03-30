package dotenv

import (
	"bufio"
	"os"
	"strings"
)

// Load reads a .env file and sets any key that is not already set in the
// process environment. Shell environment always wins over the file.
//
// Supported line formats:
//
//	KEY=value
//	KEY="value with spaces"
//	KEY='value'
//	# comment
//	(blank lines ignored)
//
// Returns silently if the file does not exist.
func Load(path string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		parseLine(scanner.Text())
	}
	return scanner.Err()
}

func parseLine(raw string) {
	line := strings.TrimSpace(raw)
	if line == "" || strings.HasPrefix(line, "#") {
		return
	}
	key, val, found := strings.Cut(line, "=")
	if !found {
		return
	}
	key = strings.TrimSpace(key)
	val = strings.TrimSpace(val)
	val = stripQuotes(val)
	if _, exists := os.LookupEnv(key); !exists {
		_ = os.Setenv(key, val)
	}
}

func stripQuotes(val string) string {
	if len(val) >= 2 {
		if (val[0] == '"' && val[len(val)-1] == '"') ||
			(val[0] == '\'' && val[len(val)-1] == '\'') {
			return val[1 : len(val)-1]
		}
	}
	return val
}
