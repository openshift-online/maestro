package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

const (
	FlagOutput = "output"
)

// Format represents the output format
type Format string

// TODO support yaml format
const (
	FormatJSON  Format = "json"
	FormatTable Format = "table"
)

// AddFormatFlag adds the --output flag to a command
func AddFormatFlag(cmd *cobra.Command) {
	cmd.Flags().StringP(FlagOutput, "o", "table", "Output format: json or table")
}

// GetFormat parses the output format from command flags
func GetFormat(cmd *cobra.Command) (Format, error) {
	formatStr, err := cmd.Flags().GetString(FlagOutput)
	if err != nil {
		return "", err
	}

	switch formatStr {
	case "json":
		return FormatJSON, nil
	case "table":
		return FormatTable, nil
	default:
		return "", fmt.Errorf("invalid output format: %s (must be json or table)", formatStr)
	}
}

// PrintJSON outputs data as JSON
func PrintJSON(w io.Writer, data interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
