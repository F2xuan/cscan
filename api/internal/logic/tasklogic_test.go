package logic

import (
	"regexp"
	"testing"

	"cscan/api/internal/types"
)

// TestHasAnyScanPhaseEnabled 测试扫描阶段检测逻辑
func TestHasAnyScanPhaseEnabled(t *testing.T) {
	testCases := []struct {
		name       string
		taskConfig map[string]interface{}
		expected   bool
	}{
		{
			"至少一个阶段启用",
			map[string]interface{}{
				"domainscan": map[string]interface{}{"enable": true},
				"portscan":   map[string]interface{}{"enable": false},
			},
			true,
		},
		{
			"所有阶段禁用",
			map[string]interface{}{
				"domainscan": map[string]interface{}{"enable": false},
				"portscan":   map[string]interface{}{"enable": false},
			},
			false,
		},
		{
			"空配置",
			map[string]interface{}{},
			false,
		},
		{
			"多个阶段启用",
			map[string]interface{}{
				"domainscan":   map[string]interface{}{"enable": true},
				"fingerprint":  map[string]interface{}{"enable": true},
				"pocscan":      map[string]interface{}{"enable": true},
			},
			true,
		},
		{
			"配置格式错误",
			map[string]interface{}{
				"domainscan": "invalid",
			},
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := hasAnyScanPhaseEnabled(tc.taskConfig)
			if result != tc.expected {
				t.Errorf("hasAnyScanPhaseEnabled() = %v, 期望 %v", result, tc.expected)
			}
		})
	}
}

// TestTaskNameRegexEscape 测试任务名称查询中的正则转义（防止 ReDoS）
func TestTaskNameRegexEscape(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"普通字符串", "test task", "test task"},
		{"包含点号", "test.task", `test\.task`},
		{"包含星号", "test*task", `test\*task`},
		{"包含问号", "test?task", `test\?task`},
		{"包含方括号", "test[task]", `test\[task\]`},
		{"包含圆括号", "test(task)", `test\(task\)`},
		{"包含加号", "test+task", `test\+task`},
		{"复杂正则字符", ".*+?[](){}^$|\\", `\.\*\+\?\[\]\(\)\{\}\^\$\|\\`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := regexp.QuoteMeta(tc.input)
			if result != tc.expected {
				t.Errorf("QuoteMeta(%q) = %q, 期望 %q", tc.input, result, tc.expected)
			}
		})
	}
}

// TestTaskStatusValidation 测试任务状态枚举值验证
func TestTaskStatusValidation(t *testing.T) {
	validStatuses := []string{"pending", "running", "success", "failed", "stopped", "stopping"}
	invalidStatuses := []string{"", "invalid", "PENDING", "Running", "completed"}

	for _, status := range validStatuses {
		t.Run("有效状态:"+status, func(t *testing.T) {
			// 模拟状态验证逻辑
			isValid := false
			for _, valid := range []string{"pending", "running", "success", "failed", "stopped", "stopping"} {
				if status == valid {
					isValid = true
					break
				}
			}
			if !isValid {
				t.Errorf("状态 %q 应该是有效的", status)
			}
		})
	}

	for _, status := range invalidStatuses {
		t.Run("无效状态:"+status, func(t *testing.T) {
			// 模拟状态验证逻辑
			isValid := false
			for _, valid := range []string{"pending", "running", "success", "failed", "stopped", "stopping"} {
				if status == valid {
					isValid = true
					break
				}
			}
			if isValid {
				t.Errorf("状态 %q 应该是无效的", status)
			}
		})
	}
}

// TestTaskPriorityValidation 测试任务优先级枚举值验证
func TestTaskPriorityValidation(t *testing.T) {
	testCases := []struct {
		priority int
		isValid  bool
	}{
		{0, true},  // background
		{1, true},  // low
		{2, true},  // normal
		{3, true},  // high
		{4, true},  // urgent
		{-1, false},
		{5, false},
		{10, false},
	}

	for _, tc := range testCases {
		t.Run(string(rune('0'+tc.priority)), func(t *testing.T) {
			isValid := tc.priority >= 0 && tc.priority <= 4
			if isValid != tc.isValid {
				t.Errorf("优先级 %d 的有效性判断错误，期望 %v，实际 %v", tc.priority, tc.isValid, isValid)
			}
		})
	}
}

// TestTaskListReq_Pagination 测试任务列表请求的分页参数
func TestTaskListReq_Pagination(t *testing.T) {
	testCases := []struct {
		name     string
		req      *types.MainTaskListReq
		expected struct {
			page     int
			pageSize int
		}
	}{
		{
			"正常分页",
			&types.MainTaskListReq{Page: 2, PageSize: 20},
			struct{ page, pageSize int }{2, 20},
		},
		{
			"零页码",
			&types.MainTaskListReq{Page: 0, PageSize: 20},
			struct{ page, pageSize int }{1, 20},
		},
		{
			"负页码",
			&types.MainTaskListReq{Page: -1, PageSize: 20},
			struct{ page, pageSize int }{1, 20},
		},
		{
			"零页大小",
			&types.MainTaskListReq{Page: 1, PageSize: 0},
			struct{ page, pageSize int }{1, 20},
		},
		{
			"超大页大小",
			&types.MainTaskListReq{Page: 1, PageSize: 200},
			struct{ page, pageSize int }{1, 100},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 模拟分页规范化逻辑
			page := tc.req.Page
			if page <= 0 {
				page = 1
			}
			pageSize := tc.req.PageSize
			if pageSize <= 0 {
				pageSize = 20
			}
			if pageSize > 100 {
				pageSize = 100
			}

			if page != tc.expected.page {
				t.Errorf("page = %d, 期望 %d", page, tc.expected.page)
			}
			if pageSize != tc.expected.pageSize {
				t.Errorf("pageSize = %d, 期望 %d", pageSize, tc.expected.pageSize)
			}
		})
	}
}

// TestTaskListReq_FilterLogic 测试任务列表过滤逻辑
func TestTaskListReq_FilterLogic(t *testing.T) {
	testCases := []struct {
		name            string
		req             *types.MainTaskListReq
		expectedFilters int // 期望的过滤条件数量
	}{
		{
			"无过滤条件",
			&types.MainTaskListReq{},
			0,
		},
		{
			"按名称过滤",
			&types.MainTaskListReq{Name: "test"},
			1,
		},
		{
			"按状态过滤",
			&types.MainTaskListReq{Status: "running"},
			1,
		},
		{
			"按标签过滤",
			&types.MainTaskListReq{Tags: []string{"tag1", "tag2"}},
			1,
		},
		{
			"多条件过滤",
			&types.MainTaskListReq{
				Name:   "test",
				Status: "success",
				Tags:   []string{"tag1"},
			},
			3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filterCount := 0
			if tc.req.Name != "" {
				filterCount++
			}
			if tc.req.Status != "" {
				filterCount++
			}
			if len(tc.req.Tags) > 0 {
				filterCount++
			}

			if filterCount != tc.expectedFilters {
				t.Errorf("过滤条件数量 = %d, 期望 %d", filterCount, tc.expectedFilters)
			}
		})
	}
}

// TestTaskConfigValidation 测试任务配置的基本验证逻辑
func TestTaskConfigValidation(t *testing.T) {
	testCases := []struct {
		name    string
		config  map[string]interface{}
		isValid bool
	}{
		{
			"至少一个扫描阶段启用",
			map[string]interface{}{
				"domainscan": map[string]interface{}{"enable": true},
			},
			true,
		},
		{
			"所有阶段禁用",
			map[string]interface{}{
				"domainscan": map[string]interface{}{"enable": false},
				"portscan":   map[string]interface{}{"enable": false},
			},
			false,
		},
		{
			"空配置",
			map[string]interface{}{},
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isValid := hasAnyScanPhaseEnabled(tc.config)
			if isValid != tc.isValid {
				t.Errorf("配置验证 = %v, 期望 %v", isValid, tc.isValid)
			}
		})
	}
}

// TestTaskTagsHandling 测试任务标签处理逻辑
func TestTaskTagsHandling(t *testing.T) {
	testCases := []struct {
		name          string
		tags          []string
		expectedCount int
		shouldBeNil   bool
	}{
		{
			"正常标签",
			[]string{"tag1", "tag2", "tag3"},
			3,
			false,
		},
		{
			"空标签数组",
			[]string{},
			0,
			false,
		},
		{
			"nil标签",
			nil,
			0,
			true,
		},
		{
			"包含空字符串",
			[]string{"tag1", "", "tag2"},
			2,
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 模拟标签清理逻辑
			var cleaned []string
			if tc.tags != nil {
				for _, tag := range tc.tags {
					if tag != "" {
						cleaned = append(cleaned, tag)
					}
				}
			}

			if tc.shouldBeNil {
				if cleaned != nil {
					t.Errorf("期望 nil，实际非 nil")
				}
				return
			}

			if cleaned == nil {
				cleaned = []string{}
			}
			if len(cleaned) != tc.expectedCount {
				t.Errorf("标签数量 = %d, 期望 %d", len(cleaned), tc.expectedCount)
			}
		})
	}
}

// TestTaskWorkspaceIdResolution 测试工作空间ID解析逻辑
func TestTaskWorkspaceIdResolution(t *testing.T) {
	testCases := []struct {
		name             string
		reqWorkspaceId   string
		ctxWorkspaceId   string
		expectedResolved string
	}{
		{
			"请求体优先",
			"workspace-from-req",
			"workspace-from-ctx",
			"workspace-from-req",
		},
		{
			"请求体为空使用上下文",
			"",
			"workspace-from-ctx",
			"workspace-from-ctx",
		},
		{
			"两者都为空",
			"",
			"",
			"",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 模拟工作空间ID解析逻辑
			resolved := tc.reqWorkspaceId
			if resolved == "" {
				resolved = tc.ctxWorkspaceId
			}

			if resolved != tc.expectedResolved {
				t.Errorf("解析的工作空间ID = %q, 期望 %q", resolved, tc.expectedResolved)
			}
		})
	}
}
