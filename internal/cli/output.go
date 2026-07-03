package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
)

// printJSON prints data as formatted JSON
func printJSON(data interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// table creates a tabwriter for aligned column output
func table() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
}

// printRow prints a tab-separated row
func printRow(w *tabwriter.Writer, values ...interface{}) {
	for i, v := range values {
		if i > 0 {
			fmt.Fprint(w, "\t")
		}
		fmt.Fprint(w, v)
	}
	fmt.Fprintln(w)
}
