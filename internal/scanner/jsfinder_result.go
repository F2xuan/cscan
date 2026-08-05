package scanner

// JSFinderResult JSFinder 扫描后独立的返回结果，不再混入漏洞 Vulnerability
type JSFinderResult struct {
	Authority        string   `json:"authority"`
	Host             string   `json:"host"`
	Port             int      `json:"port"`
	URL              string   `json:"url"`
	Severity         string   `json:"severity"`
	VulName          string   `json:"vul_name"`
	Result           string   `json:"result"`
	Tags             []string `json:"tags"`
	MatcherName      string   `json:"matcher_name"`
	ExtractedResults []string `json:"extracted_results"`
	CurlCommand      string   `json:"curl_command"`
	Request          string   `json:"request"`
	Response         string   `json:"response"`
}
