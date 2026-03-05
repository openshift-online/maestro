package output

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/openshift-online/maestro/pkg/api/openapi"
)

// TablePrinter handles table formatting
type TablePrinter struct {
	writer *tabwriter.Writer
}

// NewTablePrinter creates a new table printer
func NewTablePrinter(w io.Writer) *TablePrinter {
	return &TablePrinter{
		writer: tabwriter.NewWriter(w, 0, 0, 3, ' ', 0),
	}
}

// Flush flushes the table output
func (p *TablePrinter) Flush() error {
	return p.writer.Flush()
}

// PrintResourceBundleList prints a list of resource bundles as a table
func PrintResourceBundleList(w io.Writer, bundles []openapi.ResourceBundle) (err error) {
	printer := NewTablePrinter(w)
	defer func() {
		if flushErr := printer.Flush(); err == nil && flushErr != nil {
			err = flushErr
		}
	}()

	// Print header
	fmt.Fprintln(printer.writer, "ID\tNAME\tCONSUMER\tVERSION\tCREATED\tSTATUS")

	// Print rows
	for _, bundle := range bundles {
		id := getStringPtr(bundle.Id)
		name := getStringPtr(bundle.Name)
		consumer := getStringPtr(bundle.ConsumerName)
		version := fmt.Sprintf("%d", getInt32Ptr(bundle.Version))
		created := formatTime(bundle.CreatedAt)
		status := getStatusFromMap(bundle.Status)

		fmt.Fprintf(printer.writer, "%s\t%s\t%s\t%s\t%s\t%s\n",
			id, name, consumer, version, created, status)
	}

	return nil
}

// PrintResourceBundle prints a single resource bundle as a table
func PrintResourceBundle(w io.Writer, bundle *openapi.ResourceBundle) (err error) {
	if bundle == nil {
		return fmt.Errorf("resource bundle is required")
	}

	printer := NewTablePrinter(w)
	defer func() {
		if flushErr := printer.Flush(); err == nil && flushErr != nil {
			err = flushErr
		}
	}()

	fmt.Fprintln(printer.writer, "FIELD\tVALUE")
	fmt.Fprintf(printer.writer, "ID\t%s\n", getStringPtr(bundle.Id))
	fmt.Fprintf(printer.writer, "Name\t%s\n", getStringPtr(bundle.Name))
	fmt.Fprintf(printer.writer, "Consumer\t%s\n", getStringPtr(bundle.ConsumerName))
	fmt.Fprintf(printer.writer, "Version\t%d\n", getInt32Ptr(bundle.Version))
	fmt.Fprintf(printer.writer, "Created\t%s\n", formatTime(bundle.CreatedAt))
	fmt.Fprintf(printer.writer, "Updated\t%s\n", formatTime(bundle.UpdatedAt))
	fmt.Fprintf(printer.writer, "Status\t%s\n", getStatusFromMap(bundle.Status))

	return nil
}

// PrintConsumerList prints a list of consumers as a table
func PrintConsumerList(w io.Writer, consumers []openapi.Consumer) (err error) {
	printer := NewTablePrinter(w)
	defer func() {
		if flushErr := printer.Flush(); err == nil && flushErr != nil {
			err = flushErr
		}
	}()

	// Print header
	fmt.Fprintln(printer.writer, "ID\tNAME\tLABELS\tCREATED")

	// Print rows
	for _, consumer := range consumers {
		id := getStringPtr(consumer.Id)
		name := getStringPtr(consumer.Name)
		labels := formatLabels(consumer.Labels)
		created := formatTime(consumer.CreatedAt)

		fmt.Fprintf(printer.writer, "%s\t%s\t%s\t%s\n",
			id, name, labels, created)
	}

	return nil
}

// PrintConsumer prints a single consumer as a table
func PrintConsumer(w io.Writer, consumer *openapi.Consumer) (err error) {
	if consumer == nil {
		return fmt.Errorf("consumer is required")
	}

	printer := NewTablePrinter(w)
	defer func() {
		if flushErr := printer.Flush(); err == nil && flushErr != nil {
			err = flushErr
		}
	}()

	fmt.Fprintln(printer.writer, "FIELD\tVALUE")
	fmt.Fprintf(printer.writer, "ID\t%s\n", getStringPtr(consumer.Id))
	fmt.Fprintf(printer.writer, "Name\t%s\n", getStringPtr(consumer.Name))
	fmt.Fprintf(printer.writer, "Labels\t%s\n", formatLabels(consumer.Labels))
	fmt.Fprintf(printer.writer, "Created\t%s\n", formatTime(consumer.CreatedAt))
	fmt.Fprintf(printer.writer, "Updated\t%s\n", formatTime(consumer.UpdatedAt))

	return nil
}

// Helper functions

func getStringPtr(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func getInt32Ptr(ptr *int32) int32 {
	if ptr == nil {
		return 0
	}
	return *ptr
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func formatLabels(labels *map[string]string) string {
	if labels == nil || len(*labels) == 0 {
		return ""
	}

	var parts []string
	for k, v := range *labels {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ",")
}

// PrintResourceBundleStatus prints the status field of a resource bundle as a table
func PrintResourceBundleStatus(w io.Writer, bundleID string, status map[string]interface{}) (err error) {
	printer := NewTablePrinter(w)
	defer func() {
		if flushErr := printer.Flush(); err == nil && flushErr != nil {
			err = flushErr
		}
	}()

	fmt.Fprintln(printer.writer, "FIELD\tVALUE")
	fmt.Fprintf(printer.writer, "ID\t%s\n", bundleID)
	fmt.Fprintf(printer.writer, "Status\t%s\n", getStatusFromMap(status))

	// Print conditions if available
	conditionsInterface, ok := status["conditions"]
	if ok {
		conditions, ok := conditionsInterface.([]interface{})
		if ok && len(conditions) > 0 {
			fmt.Fprintln(printer.writer, "")
			fmt.Fprintln(printer.writer, "Conditions:")
			for _, condInterface := range conditions {
				cond, ok := condInterface.(map[string]interface{})
				if !ok {
					continue
				}

				condType, _ := cond["type"].(string)
				condStatus, _ := cond["status"].(string)
				reason, _ := cond["reason"].(string)
				message, _ := cond["message"].(string)
				lastTransitionTime, _ := cond["lastTransitionTime"].(string)

				fmt.Fprintf(printer.writer, "  Type\t%s\n", condType)
				fmt.Fprintf(printer.writer, "  Status\t%s\n", condStatus)
				if reason != "" {
					fmt.Fprintf(printer.writer, "  Reason\t%s\n", reason)
				}
				if message != "" {
					fmt.Fprintf(printer.writer, "  Message\t%s\n", message)
				}
				if lastTransitionTime != "" {
					fmt.Fprintf(printer.writer, "  LastTransitionTime\t%s\n", lastTransitionTime)
				}
				fmt.Fprintln(printer.writer, "")
			}
		}
	}

	return nil
}

func getStatusFromMap(status map[string]interface{}) string {
	if len(status) == 0 {
		return "Unknown"
	}

	// Extract conditions from the status map
	conditionsInterface, ok := status["conditions"]
	if !ok {
		return "Unknown"
	}

	conditions, ok := conditionsInterface.([]interface{})
	if !ok || len(conditions) == 0 {
		return "Unknown"
	}

	// Find the Applied condition
	for _, condInterface := range conditions {
		cond, ok := condInterface.(map[string]interface{})
		if !ok {
			continue
		}

		condType, _ := cond["type"].(string)
		if condType == "Applied" {
			condStatus, _ := cond["status"].(string)
			if condStatus == "True" {
				return "Applied"
			}
			return "Pending"
		}
	}

	return "Unknown"
}
