package result

import (
	"encoding/xml"
	"time"
)

// JUnitXMLReport represents the JUnit XML format.
type JUnitXMLReport struct {
	XMLName    xml.Name         `xml:"testsuites"`
	Name       string           `xml:"name,attr"`
	Tests      int              `xml:"tests,attr"`
	Failures   int              `xml:"failures,attr"`
	Errors     int              `xml:"errors,attr"`
	Time       float64          `xml:"time,attr"`
	TestSuites []JUnitTestSuite `xml:"testsuite"`
}

// JUnitTestSuite represents a test suite in JUnit XML.
type JUnitTestSuite struct {
	Name     string          `xml:"name,attr"`
	Tests    int             `xml:"tests,attr"`
	Failures int             `xml:"failures,attr"`
	Errors   int             `xml:"errors,attr"`
	Time     float64         `xml:"time,attr"`
	Cases    []JUnitTestCase `xml:"testcase"`
}

// JUnitTestCase represents a test case in JUnit XML.
type JUnitTestCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Time      float64       `xml:"time,attr"`
	Failure   *JUnitFailure `xml:"failure,omitempty"`
	Error     *JUnitFailure `xml:"error,omitempty"`
	Skipped   *struct{}     `xml:"skipped,omitempty"`
}

// JUnitFailure represents a failure or error in JUnit XML.
type JUnitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Text    string `xml:",chardata"`
}

// ToJUnitXML converts Report to JUnit XML format bytes.
func (r *Report) ToJUnitXML() ([]byte, error) {
	totalDuration := r.EndTime.Sub(r.StartTime)
	suite := JUnitTestSuite{
		Name:     "X-Mate Tests",
		Tests:    r.TotalCases,
		Failures: r.FailedCases,
		Errors:   r.ErrorCases,
		Time:     totalDuration.Seconds(),
	}

	for _, cr := range r.Results {
		tc := JUnitTestCase{
			Name:      cr.Name,
			ClassName: "xmate",
			Time:      cr.Duration.Seconds(),
		}
		switch cr.Status {
		case Failed:
			msg := buildFailureMessage(cr.Steps)
			tc.Failure = &JUnitFailure{
				Message: "test failed",
				Type:    "failure",
				Text:    msg,
			}
		case Error:
			msg := buildFailureMessage(cr.Steps)
			if msg == "" {
				msg = "configuration error: no steps executed"
			}
			tc.Error = &JUnitFailure{
				Message: "test error",
				Type:    "error",
				Text:    msg,
			}
		case Skipped:
			tc.Skipped = &struct{}{}
		}
		suite.Cases = append(suite.Cases, tc)
	}

	report := JUnitXMLReport{
		Name:       "X-Mate",
		Tests:      r.TotalCases,
		Failures:   r.FailedCases,
		Errors:     r.ErrorCases,
		Time:       totalDuration.Seconds(),
		TestSuites: []JUnitTestSuite{suite},
	}

	return xml.MarshalIndent(report, "", "  ")
}

// buildFailureMessage constructs a failure message from step results.
func buildFailureMessage(steps []StepResult) string {
	msg := ""
	for _, s := range steps {
		if !s.Pass {
			msg += "[" + s.Phase + "] " + s.Desc + " (" + s.Type + "): " + s.Message + "\n"
		}
	}
	return msg
}

// DurationToSeconds converts a time.Duration to seconds as a float64,
// suitable for JUnit XML time attributes.
func DurationToSeconds(d time.Duration) float64 {
	return d.Seconds()
}
