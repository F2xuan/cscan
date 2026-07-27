package swagger

// SwaggerUIHTML 是 swagger-ui 的最小内嵌 HTML 页面：
// 从 CDN 加载 swagger-ui-bundle，并将 specUrl 注入为后端的 /swagger/doc.json。
// 采用 CDN 加载避免在 Go 二进制内打包静态资源；生产环境可通过反向代理替换为本地资源。
const SwaggerUIHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>CSCAN API 文档</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.18.2/swagger-ui.css" />
  <style>
    html, body { margin: 0; padding: 0; height: 100%; }
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, "PingFang SC", "Microsoft YaHei", sans-serif; }
    #swagger-ui { height: 100vh; }
    .topbar { display: none; }
    .swagger-ui .info { margin: 24px 0 8px; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.18.2/swagger-ui-bundle.js" charset="UTF-8"></script>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.18.2/swagger-ui-standalone-preset.js" charset="UTF-8"></script>
  <script>
    window.onload = function () {
      window.ui = SwaggerUIBundle({
        url: "/swagger/doc.json",
        dom_id: "#swagger-ui",
        deepLinking: true,
        presets: [SwaggerUIBundle.presets.apis],
        plugins: [SwaggerUIBundle.plugins.DownloadUrl],
        layout: "BaseLayout",
        docExpansion: "list",
        defaultModelsExpandDepth: 3,
        defaultModelExpandDepth: 3,
        defaultModelRendering: "model",
        displayRequestDuration: true,
        operationsSorter: "alpha",
        tagsSorter: "alpha",
        filter: true,
        persistAuthorization: true,
        requestSnippetsEnabled: false,
        tryItOutEnabled: true,
        withCredentials: false
      });
    };
  </script>
</body>
</html>`
