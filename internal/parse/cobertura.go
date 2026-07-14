package parse

import (
	"bytes"
	"encoding/xml"
	"fmt"

	"github.com/JaydenCJ/covdrift/internal/model"
)

// Cobertura XML shape, reduced to what line-coverage diffing needs. Only
// the class-level <lines> element is read: cobertura duplicates method
// lines under <methods>, and counting both would double-instrument files.
type xmlCoverage struct {
	XMLName  xml.Name     `xml:"coverage"`
	Packages []xmlPackage `xml:"packages>package"`
}

type xmlPackage struct {
	Classes []xmlClass `xml:"classes>class"`
}

type xmlClass struct {
	Filename string    `xml:"filename,attr"`
	Lines    []xmlLine `xml:"lines>line"`
}

type xmlLine struct {
	Number int   `xml:"number,attr"`
	Hits   int64 `xml:"hits,attr"`
}

// Cobertura parses a cobertura XML report (coverage.py, pytest-cov, JaCoCo
// exports, gcovr --cobertura, Istanbul's cobertura reporter, …). Multiple
// <class> entries for the same filename — one class per Java/Python class
// in the same source file — are merged by summing per-line counts.
func Cobertura(data []byte) (*model.Report, error) {
	var doc xmlCoverage
	dec := xml.NewDecoder(bytes.NewReader(data))
	// Cobertura files routinely carry a DOCTYPE pointing at a DTD URL;
	// treat it as inert markup rather than fetching anything.
	dec.Strict = true
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("not a valid cobertura XML report: %v", err)
	}
	if doc.XMLName.Local != "coverage" {
		return nil, fmt.Errorf("not a cobertura report: root element is <%s>, want <coverage>", doc.XMLName.Local)
	}

	report := model.NewReport(FormatCobertura)
	for _, pkg := range doc.Packages {
		for _, class := range pkg.Classes {
			if class.Filename == "" {
				return nil, fmt.Errorf("cobertura: <class> element without a filename attribute")
			}
			file := report.File(class.Filename)
			for _, line := range class.Lines {
				if line.Number < 1 {
					return nil, fmt.Errorf("cobertura: %s: invalid line number %d", class.Filename, line.Number)
				}
				if line.Hits < 0 {
					return nil, fmt.Errorf("cobertura: %s:%d: negative hit count %d", class.Filename, line.Number, line.Hits)
				}
				if line.Hits == 0 {
					file.MarkSeen(line.Number)
				} else {
					file.AddHits(line.Number, line.Hits)
				}
			}
		}
	}
	return report, nil
}
