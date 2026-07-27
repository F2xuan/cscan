package container

import (
	"encoding/json"
	"io"
	"net/http"
)

// parseBody 解析 JSON 请求体
func parseBody(r *http.Request, v interface{}) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, v)
}
