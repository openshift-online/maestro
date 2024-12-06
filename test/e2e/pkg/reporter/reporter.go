package reporter

import (
	"encoding/xml"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2/types"
)

type JUnitTestSuite struct {
	XMLName xml.Name `xml:"testsuite"`
	// Name maps onto the description of the test suite - maps onto Report.SuiteDescription
	Name string `xml:"name,attr"`
	// Package maps onto the absolute path to the test suite - maps onto Report.SuitePath
	Package string `xml:"package,attr"`
	// Tests maps onto the total number of specs in the test suite (this includes any suite nodes such as BeforeSuite)
	Tests int `xml:"tests,attr"`
	// Disabled maps onto specs that are pending
	Disabled int `xml:"disabled,attr"`
	// Skiped maps onto specs that are skipped
	Skipped int `xml:"skipped,attr"`
	// Errors maps onto specs that panicked or were interrupted
	Errors int `xml:"errors,attr"`
	// Failures maps onto specs that failed
	Failures int `xml:"failures,attr"`
	// Time is the time in seconds to execute all the test suite - maps onto Report.RunTime
	Time float64 `xml:"time,attr"`
	// Timestamp is the ISO 8601 formatted start-time of the suite - maps onto Report.StartTime
	Timestamp string `xml:"timestamp,attr"`
	// TestCases capture the individual specs
	TestCases []JUnitTestCase `xml:"testcase"`
}

type JUnitTestCase struct {
	// Name maps onto the full text of the spec - equivalent to "[SpecReport.LeafNodeType] SpecReport.FullText()"
	Name string `xml:"name,attr"`
	// Classname maps onto the name of the test suite - equivalent to Report.SuiteDescription
	Classname string `xml:"classname,attr"`
	// Status maps onto the string representation of SpecReport.State
	Status string `xml:"status,attr"`
	// Time is the time in seconds to execute the spec - maps onto SpecReport.RunTime
	Time float64 `xml:"time,attr"`
	// Skipped is populated with a message if the test was skipped or pending
	Skipped *JUnitSkipped `xml:"skipped,omitempty"`
	// Error is populated if the test panicked or was interrupted
	Error *JUnitError `xml:"error,omitempty"`
	// Failure is populated if the test failed
	Failure *JUnitFailure `xml:"failure,omitempty"`
	// SystemOut maps onto any captured stdout/stderr output - maps onto SpecReport.CapturedStdOutErr
	SystemOut string `xml:"system-out,omitempty"`
	// SystemOut maps onto any captured GinkgoWriter output - maps onto SpecReport.CapturedGinkgoWriterOutput
	SystemErr string `xml:"system-err,omitempty"`
}

type JUnitSkipped struct {
	// Message maps onto "pending" if the test was marked pending, "skipped" if the test was marked skipped, and "skipped - REASON" if the user called Skip(REASON)
	Message string `xml:"message,attr"`
}

type JUnitError struct {
	// Message maps onto the panic/exception thrown - equivalent to SpecReport.Failure.ForwardedPanic - or to "interrupted"
	Message string `xml:"message,attr"`
	// Type is one of "panicked" or "interrupted"
	Type string `xml:"type,attr"`
	// Description maps onto the captured stack trace for a panic, or the failure message for an interrupt which will include the dump of running goroutines
	Description string `xml:",chardata"`
}

type JUnitFailure struct {
	// Message maps onto the failure message - equivalent to SpecReport.Failure.Message
	Message string `xml:"message,attr"`
	// Type is "failed"
	Type string `xml:"type,attr"`
	// Description maps onto the location and stack trace of the failure
	Description string `xml:",chardata"`
}

func GenerateJUnitReport(report types.Report, dst string) error {
	suite := JUnitTestSuite{
		Name:      report.SuiteDescription,
		Package:   report.SuitePath,
		Time:      report.RunTime.Seconds(),
		Timestamp: report.StartTime.Format("2006-01-02T15:04:05"),
	}
	for _, spec := range report.SpecReports {
		if spec.FullText() != "" {
			name := spec.LeafNodeText
			labels := spec.Labels()
			if len(labels) > 0 {
				name = name + " [" + strings.Join(labels, ", ") + "]"
			}

			test := JUnitTestCase{
				Name:      name,
				Classname: report.SuiteDescription,
				Status:    spec.State.String(),
				Time:      spec.RunTime.Seconds(),
				SystemOut: systemOutForUnstructureReporters(spec),
				SystemErr: spec.CapturedGinkgoWriterOutput,
			}

			suite.Tests += 1

			switch spec.State {
			case types.SpecStateSkipped:
				message := "skipped"
				if spec.Failure.Message != "" {
					message += " - " + spec.Failure.Message
				}
				test.Skipped = &JUnitSkipped{Message: message}
				suite.Skipped += 1
			case types.SpecStatePending:
				test.Skipped = &JUnitSkipped{Message: "pending"}
				suite.Disabled += 1
			case types.SpecStateFailed:
				test.Failure = &JUnitFailure{
					Message:     spec.Failure.Message,
					Type:        "failed",
					Description: fmt.Sprintf("%s\n%s", spec.Failure.Location.String(), spec.Failure.Location.FullStackTrace),
				}
				suite.Failures += 1
			case types.SpecStateInterrupted:
				test.Error = &JUnitError{
					Message:     "interrupted",
					Type:        "interrupted",
					Description: spec.Failure.Message,
				}
				suite.Errors += 1
			case types.SpecStateAborted:
				test.Failure = &JUnitFailure{
					Message:     spec.Failure.Message,
					Type:        "aborted",
					Description: fmt.Sprintf("%s\n%s", spec.Failure.Location.String(), spec.Failure.Location.FullStackTrace),
				}
				suite.Errors += 1
			case types.SpecStatePanicked:
				test.Error = &JUnitError{
					Message:     spec.Failure.ForwardedPanic,
					Type:        "panicked",
					Description: fmt.Sprintf("%s\n%s", spec.Failure.Location.String(), spec.Failure.Location.FullStackTrace),
				}
				suite.Errors += 1
			}

			suite.TestCases = append(suite.TestCases, test)
		}
	}

	junitReport := []JUnitTestSuite{suite}

	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, err = f.WriteString(xml.Header)
	if err != nil {
		return err
	}
	encoder := xml.NewEncoder(f)
	encoder.Indent("  ", "    ")
	err = encoder.Encode(junitReport)
	if err != nil {
		return err
	}

	return f.Close()
}

func systemOutForUnstructureReporters(spec types.SpecReport) string {
	systemOut := spec.CapturedStdOutErr
	if len(spec.ReportEntries) > 0 {
		systemOut += "\nReport Entries:\n"
		for i, entry := range spec.ReportEntries {
			systemOut += fmt.Sprintf("%s\n%s\n%s\n", entry.Name, entry.Location, entry.Time.Format(time.RFC3339Nano))
			if representation := entry.StringRepresentation(); representation != "" {
				systemOut += representation + "\n"
			}
			if i+1 < len(spec.ReportEntries) {
				systemOut += "--\n"
			}
		}
	}
	return systemOut
}
