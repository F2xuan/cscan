package worker

import "cscan/internal/scanner"

// ToVulDocument transforms a scanner.Vulnerability into a VulDocument
// Used to consolidate mapping logic between result_sink.go and worker.go
func ToVulDocument(vul *scanner.Vulnerability, taskId string) VulDocument {
	doc := VulDocument{
		Authority: vul.Authority,
		Host:      vul.Host,
		Port:      int32(vul.Port),
		Url:       vul.Url,
		PocFile:   vul.PocFile,
		Source:    vul.Source,
		Severity:  vul.Severity,
		Result:    vul.Result,
		Extra:     vul.Extra,
		TaskId:    taskId,
	}

	if vul.VulName != "" {
		name := vul.VulName
		doc.VulName = &name
	}
	if len(vul.Tags) > 0 {
		doc.Tags = vul.Tags
	}

	if vul.CvssScore > 0 {
		score := vul.CvssScore
		doc.CvssScore = &score
	}
	if vul.CveId != "" {
		cve := vul.CveId
		doc.CveId = &cve
	}
	if vul.CweId != "" {
		cwe := vul.CweId
		doc.CweId = &cwe
	}
	if vul.Remediation != "" {
		rem := vul.Remediation
		doc.Remediation = &rem
	}
	if len(vul.References) > 0 {
		doc.References = vul.References
	}

	if vul.MatcherName != "" {
		mn := vul.MatcherName
		doc.MatcherName = &mn
	}
	if len(vul.ExtractedResults) > 0 {
		doc.ExtractedResults = vul.ExtractedResults
	}
	if vul.CurlCommand != "" {
		cmd := vul.CurlCommand
		doc.CurlCommand = &cmd
	}
	if vul.Request != "" {
		req := vul.Request
		doc.Request = &req
	}
	if vul.Response != "" {
		res := vul.Response
		doc.Response = &res
	}

	doc.ResponseTruncated = &vul.ResponseTruncated

	return doc
}
