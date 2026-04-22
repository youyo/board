package main

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/cli"
)

var version = "dev"

func main() {
	rootCmd := cli.NewRootCmd(version)

	if err := rootCmd.Execute(); err != nil {
		var apiErr *boardapi.APIError
		if errors.As(err, &apiErr) {
			result := map[string]interface{}{
				"error":       true,
				"code":        string(apiErr.Code),
				"status_code": apiErr.StatusCode,
				"message":     apiErr.Message,
			}
			if hint := apiErr.Hint(); hint != "" {
				result["hint"] = hint
			}
			if apiErr.RetryAfter > 0 {
				result["retry_after_seconds"] = int(apiErr.RetryAfter.Seconds())
			}
			_ = json.NewEncoder(os.Stderr).Encode(result)
		} else {
			_ = json.NewEncoder(os.Stderr).Encode(map[string]interface{}{
				"error":   true,
				"message": err.Error(),
			})
		}
		os.Exit(1)
	}
}
