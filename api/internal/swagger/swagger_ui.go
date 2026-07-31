package swagger

// SwaggerUIHTML 是 swagger-ui 的内嵌 HTML 页面：
// 从 CDN 加载 swagger-ui-dist，并将 specUrl 注入为后端的 /swagger/doc.json。
// 采用 CDN 加载避免在 Go 二进制内打包静态资源；生产环境可通过反向代理替换为本地资源。
const SwaggerUIHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>CSCAN API 文档</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.20.0/swagger-ui.css" />
  <link rel="icon" type="image/svg+xml" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>📖</text></svg>">
  <style>
    :root {
      --cscan-primary: #2563eb;
      --cscan-primary-dark: #1d4ed8;
      --cscan-bg: #f8fafc;
      --cscan-surface: #ffffff;
      --cscan-border: #e2e8f0;
      --cscan-text: #1e293b;
      --cscan-text-secondary: #64748b;
      --cscan-success: #10b981;
      --cscan-warning: #f59e0b;
      --cscan-danger: #ef4444;
    }

    @media (prefers-color-scheme: dark) {
      :root {
        --cscan-bg: #0f172a;
        --cscan-surface: #1e293b;
        --cscan-border: #334155;
        --cscan-text: #e2e8f0;
        --cscan-text-secondary: #94a3b8;
      }
    }

    * { box-sizing: border-box; }

    html, body {
      margin: 0;
      padding: 0;
      height: 100%;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial,
                   "PingFang SC", "Microsoft YaHei", "Noto Sans SC", sans-serif;
      background: var(--cscan-bg);
      color: var(--cscan-text);
      -webkit-font-smoothing: antialiased;
    }

    /* 顶部导航栏 */
    .cscan-header {
      background: linear-gradient(135deg, var(--cscan-primary) 0%, #7c3aed 100%);
      color: white;
      padding: 16px 24px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      box-shadow: 0 2px 8px rgba(0,0,0,0.15);
      position: sticky;
      top: 0;
      z-index: 100;
    }

    .cscan-header-left {
      display: flex;
      align-items: center;
      gap: 12px;
    }

    .cscan-logo {
      font-size: 24px;
      font-weight: 700;
      letter-spacing: -0.5px;
      display: flex;
      align-items: center;
      gap: 8px;
    }

    .cscan-logo-badge {
      background: rgba(255,255,255,0.2);
      padding: 2px 10px;
      border-radius: 12px;
      font-size: 12px;
      font-weight: 600;
      backdrop-filter: blur(4px);
    }

    .cscan-header-right {
      display: flex;
      align-items: center;
      gap: 16px;
      font-size: 13px;
      opacity: 0.9;
    }

    .cscan-header-right a {
      color: white;
      text-decoration: none;
      opacity: 0.85;
      transition: opacity 0.2s;
    }

    .cscan-header-right a:hover {
      opacity: 1;
      text-decoration: underline;
    }

    /* Swagger UI 容器 */
    #swagger-ui {
      min-height: calc(100vh - 60px);
    }

    /* 覆盖 Swagger UI 默认样式 */
    .swagger-ui .topbar { display: none !important; }

    .swagger-ui .info {
      margin: 24px 0 16px;
      padding: 20px 24px;
      background: var(--cscan-surface);
      border-radius: 12px;
      border: 1px solid var(--cscan-border);
      box-shadow: 0 1px 3px rgba(0,0,0,0.05);
    }

    .swagger-ui .info .title {
      font-size: 28px;
      font-weight: 700;
      color: var(--cscan-text);
      margin: 0 0 8px;
    }

    .swagger-ui .info p {
      color: var(--cscan-text-secondary);
      line-height: 1.6;
    }

    /* 操作块美化 */
    .swagger-ui .opblock {
      border-radius: 10px;
      border: 1px solid var(--cscan-border);
      box-shadow: 0 1px 3px rgba(0,0,0,0.04);
      margin-bottom: 8px;
      overflow: hidden;
      transition: box-shadow 0.2s;
    }

    .swagger-ui .opblock:hover {
      box-shadow: 0 4px 12px rgba(0,0,0,0.08);
    }

    .swagger-ui .opblock .opblock-summary {
      padding: 12px 16px;
      border-bottom: none;
    }

    .swagger-ui .opblock .opblock-summary-method {
      border-radius: 6px;
      font-weight: 700;
      font-size: 12px;
      min-width: 70px;
      text-align: center;
      padding: 6px 0;
    }

    .swagger-ui .opblock.opblock-post { border-color: #bfdbfe; background: rgba(37,99,235,0.02); }
    .swagger-ui .opblock.opblock-post .opblock-summary-method { background: var(--cscan-primary); }
    .swagger-ui .opblock.opblock-get { border-color: #bbf7d0; background: rgba(16,185,129,0.02); }
    .swagger-ui .opblock.opblock-get .opblock-summary-method { background: var(--cscan-success); }
    .swagger-ui .opblock.opblock-delete { border-color: #fecaca; background: rgba(239,68,68,0.02); }
    .swagger-ui .opblock.opblock-delete .opblock-summary-method { background: var(--cscan-danger); }
    .swagger-ui .opblock.opblock-put { border-color: #fef3c7; background: rgba(245,158,11,0.02); }
    .swagger-ui .opblock.opblock-put .opblock-summary-method { background: var(--cscan-warning); }

    .swagger-ui .opblock-summary-path {
      font-family: "JetBrains Mono", "Fira Code", "Cascadia Code", Consolas, monospace;
      font-size: 14px;
      font-weight: 600;
    }

    .swagger-ui .opblock-summary-description {
      font-size: 13px;
      color: var(--cscan-text-secondary);
    }

    /* 标签分组美化 */
    .swagger-ui .opblock-tag {
      font-size: 18px;
      font-weight: 700;
      color: var(--cscan-text);
      border-bottom: 2px solid var(--cscan-primary);
      padding: 16px 0 8px;
      margin-top: 24px;
    }

    .swagger-ui .opblock-tag small {
      font-size: 13px;
      font-weight: 400;
      color: var(--cscan-text-secondary);
    }

    /* 参数表格 */
    .swagger-ui table thead tr td,
    .swagger-ui table thead tr th {
      border-bottom: 2px solid var(--cscan-border);
      font-weight: 600;
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      color: var(--cscan-text-secondary);
    }

    .swagger-ui .parameters-col_description input,
    .swagger-ui .parameters-col_description textarea,
    .swagger-ui .parameters-col_description select {
      border: 1px solid var(--cscan-border);
      border-radius: 6px;
      padding: 8px 12px;
      font-size: 13px;
      transition: border-color 0.2s;
    }

    .swagger-ui .parameters-col_description input:focus,
    .swagger-ui .parameters-col_description textarea:focus {
      border-color: var(--cscan-primary);
      outline: none;
      box-shadow: 0 0 0 3px rgba(37,99,235,0.1);
    }

    /* 按钮美化 */
    .swagger-ui .btn {
      border-radius: 6px;
      font-weight: 600;
      font-size: 13px;
      padding: 8px 20px;
      transition: all 0.2s;
      border: none;
      cursor: pointer;
    }

    .swagger-ui .btn.execute {
      background: var(--cscan-primary);
      color: white;
    }

    .swagger-ui .btn.execute:hover {
      background: var(--cscan-primary-dark);
    }

    .swagger-ui .btn.authorize {
      border: 2px solid var(--cscan-primary);
      color: var(--cscan-primary);
      border-radius: 8px;
      padding: 8px 20px;
    }

    .swagger-ui .btn.authorize:hover {
      background: var(--cscan-primary);
      color: white;
    }

    /* 响应区域 */
    .swagger-ui .responses-inner {
      padding: 16px;
    }

    .swagger-ui .response-col_status {
      font-weight: 700;
      font-family: "JetBrains Mono", monospace;
    }

    .swagger-ui .highlight-code {
      border-radius: 8px;
      font-size: 12px;
    }

    /* Model 区域 */
    .swagger-ui .model-box {
      background: var(--cscan-surface);
      border: 1px solid var(--cscan-border);
      border-radius: 8px;
      padding: 12px;
    }

    /* Schema 链接 */
    .swagger-ui a.nostyle,
    .swagger-ui a.nostyle:visited {
      color: var(--cscan-primary);
      text-decoration: none;
    }

    .swagger-ui a.nostyle:hover {
      text-decoration: underline;
    }

    /* 授权弹窗 */
    .swagger-ui .dialog-ux .modal-ux {
      border-radius: 12px;
      border: 1px solid var(--cscan-border);
      box-shadow: 0 20px 60px rgba(0,0,0,0.15);
    }

    /* 暗色模式适配 */
    @media (prefers-color-scheme: dark) {
      .swagger-ui {
        color: var(--cscan-text);
      }
      .swagger-ui .opblock {
        background: var(--cscan-surface);
      }
      .swagger-ui .opblock .opblock-section-header {
        background: rgba(255,255,255,0.03);
      }
      .swagger-ui table tbody tr td {
        border-bottom: 1px solid var(--cscan-border);
      }
      .swagger-ui input,
      .swagger-ui textarea,
      .swagger-ui select {
        background: #0f172a;
        color: var(--cscan-text);
        border-color: var(--cscan-border);
      }
      .swagger-ui .model-box {
        background: #0f172a;
      }
      .swagger-ui section.models {
        border-color: var(--cscan-border);
      }
      .swagger-ui section.models h4 {
        color: var(--cscan-text);
      }
    }

    /* 加载动画 */
    .swagger-loading {
      display: flex;
      align-items: center;
      justify-content: center;
      height: 400px;
      color: var(--cscan-text-secondary);
      font-size: 15px;
    }

    .swagger-loading::after {
      content: "";
      width: 24px;
      height: 24px;
      margin-left: 12px;
      border: 3px solid var(--cscan-border);
      border-top-color: var(--cscan-primary);
      border-radius: 50%;
      animation: spin 0.8s linear infinite;
    }

    @keyframes spin {
      to { transform: rotate(360deg); }
    }

    /* 隐藏多余的空 Parameters 区域：
       - 当接口有 Request Body 且 Parameters 中显示 "No parameters" 时隐藏
       - 当 Parameters 表格为空行时隐藏 */
    .swagger-ui .opblock-body .parameters-container .table-container {
      position: relative;
    }
    .swagger-ui .parameters-container:has(.no-parameters) {
      display: none !important;
    }
    .swagger-ui .opblock-body > div:not(:has(.opblock-section)):has(.parameters) > div:has(table.parameters) {
      /* 如果有 request body 且 parameters 表格只有一行（空），隐藏整个 parameters section */
    }
    /* 更精准的规则：当同一 opblock 内有 request-body 且 parameters 区域显示 No parameters 时隐藏 */
    .swagger-ui .opblock:has(.opblock-section-request-body) .parameters-container:has(.no-parameters) {
      display: none !important;
    }
    .swagger-ui .opblock:has(.opblock-section-request-body) .parameters-container table.parameters > tbody > tr:only-child > td[colspan] {
      display: none;
    }
    .swagger-ui .opblock:has(.opblock-section-request-body) .parameters-container:has(table.parameters > tbody > tr > td[colspan]) {
      display: none !important;
    }

    /* 响应式 */
    @media (max-width: 768px) {
      .cscan-header {
        padding: 12px 16px;
        flex-direction: column;
        gap: 8px;
        text-align: center;
      }
      .cscan-logo { font-size: 20px; }
      .swagger-ui .opblock-summary-path { font-size: 12px; }
    }
  </style>
</head>
<body>
  <!-- 顶部导航栏 -->
  <div class="cscan-header">
    <div class="cscan-header-left">
      <div class="cscan-logo">
        <span>📖</span>
        <span>CSCAN API</span>
        <span class="cscan-logo-badge">v1</span>
      </div>
    </div>
    <div class="cscan-header-right">
      <span>分布式网络资产扫描平台</span>
    </div>
  </div>

  <!-- Swagger UI 挂载点 -->
  <div id="swagger-ui">
    <div class="swagger-loading">正在加载 API 文档...</div>
  </div>

  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.20.0/swagger-ui-bundle.js" charset="UTF-8"></script>
  <script>
    window.onload = function () {
      // 从 localStorage 恢复上次保存的认证信息
      const authKey = 'cscan-swagger-auth';

      window.ui = SwaggerUIBundle({
        url: "/swagger/doc.json",
        dom_id: "#swagger-ui",
        deepLinking: true,
        presets: [SwaggerUIBundle.presets.apis],
        plugins: [
          SwaggerUIBundle.plugins.DownloadUrl,
          // 保存认证信息插件
          function() {
            return {
              statePlugins: {
                auth: {
                  wrapActions: {
                    authorize: function(oriAction) {
                      return function(payload) {
                        try {
                          localStorage.setItem(authKey, JSON.stringify(payload));
                        } catch(e) {}
                        return oriAction(payload);
                      }
                    },
                    logout: function(oriAction) {
                      return function(payload) {
                        try {
                          localStorage.removeItem(authKey);
                        } catch(e) {}
                        return oriAction(payload);
                      }
                    }
                  }
                }
              }
            }
          }
        ],
        layout: "BaseLayout",
        docExpansion: "list",
        defaultModelsExpandDepth: 2,
        defaultModelExpandDepth: 3,
        defaultModelRendering: "model",
        displayRequestDuration: true,
        operationsSorter: "alpha",
        tagsSorter: "alpha",
        filter: true,
        persistAuthorization: true,
        tryItOutEnabled: true,
        withCredentials: false,
        requestSnippetsEnabled: false,
        showExtensions: true,
        showCommonExtensions: true,
        // 中文语言
        onComplete: function() {
          // 自动恢复保存的认证信息
          try {
            const saved = localStorage.getItem(authKey);
            if (saved) {
              const authData = JSON.parse(saved);
              window.ui.preauthorizeApiKey("BearerAuth", 
                authData.BearerAuth?.bearer?.value || authData.BearerAuth?.value || "");
            }
          } catch(e) {}

          // 隐藏多余的空 Parameters 区域：
          // 当接口有 Request Body 且 Parameters 为空时（POST 接口全用 JSON Body）
          function hideEmptyParameters() {
            document.querySelectorAll('.opblock').forEach(function(block) {
              var hasBody = block.querySelector('.opblock-section-request-body');
              var paramsContainer = block.querySelector('.parameters-container');
              if (hasBody && paramsContainer) {
                var noParams = paramsContainer.querySelector('.no-parameters');
                var emptyColspan = paramsContainer.querySelector('td[colspan]');
                var rows = paramsContainer.querySelectorAll('table.parameters tbody tr');
                // 如果 Parameters 区域为空或只有一行提示文字，则隐藏
                if (noParams || emptyColspan || rows.length === 0) {
                  paramsContainer.style.display = 'none';
                  // 同时隐藏 Parameters 的 section header
                  var sectionHeader = paramsContainer.closest('div')?.previousElementSibling;
                  if (sectionHeader && sectionHeader.classList.contains('opblock-section-header')) {
                    // 检查标题是否包含 "Parameters"
                    var h4 = sectionHeader.querySelector('h4');
                    if (h4 && h4.textContent.trim() === 'Parameters') {
                      sectionHeader.style.display = 'none';
                    }
                  }
                }
              }
            });
          }

          // 初始执行 + DOM 变化时自动执行（处理展开/折叠操作）
          hideEmptyParameters();
          var observer = new MutationObserver(function() {
            hideEmptyParameters();
          });
          observer.observe(document.getElementById('swagger-ui'), {
            childList: true,
            subtree: true
          });
        }
      });
    };
  </script>
</body>
</html>`
