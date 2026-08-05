package worker

import "cscan/internal/scanner"

// ToAssetDocument transforms a scanner.Asset into an AssetDocument
// Used to consolidate mapping logic between result_sink.go and worker.go
func ToAssetDocument(asset *scanner.Asset) AssetDocument {
	doc := AssetDocument{
		Authority:  asset.Authority,
		Host:       asset.Host,
		Port:       int32(asset.Port),
		Category:   asset.Category,
		Service:    asset.Service,
		Title:      asset.Title,
		App:        asset.App,
		HttpStatus: asset.HttpStatus,
		HttpHeader: asset.HttpHeader,
		HttpBody:   asset.HttpBody,
		Cert:       asset.Cert,
		IconHash:   asset.IconHash,
		IconData:   asset.IconData,
		Screenshot: asset.Screenshot,
		Server:     asset.Server,
		Banner:     asset.Banner,
		IsHttp:     asset.IsHTTP,
		Cname:      asset.CName,
		IsCdn:      asset.IsCDN,
		IsCloud:    asset.IsCloud,
		Source:     asset.Source,
	}

	for _, ip := range asset.IPV4 {
		doc.Ipv4 = append(doc.Ipv4, IPV4Info{IP: ip.IP, Location: ip.Location})
	}

	for _, ip := range asset.IPV6 {
		doc.Ipv6 = append(doc.Ipv6, IPV6Info{IP: ip.IP, Location: ip.Location})
	}

	return doc
}

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
