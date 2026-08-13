#!/bin/sh
# API 容器启动入口。
# 配置注入对齐 12-factor：JWT / Mongo / Redis 全部由 Go 配置层经环境变量读取
# （api/internal/config/config.go LoadSecretFromEnv），无需 envsubst 渲染模板，
# 也避免密码特殊字符（@:/ 等）在模板替换时破坏连接串。

# JWT 密钥持久化目录（挂载到 volume，保证重启后 token 不失效）
JWT_SECRET_FILE="/app/data/.jwt_secret"
mkdir -p /app/data

if [ -n "$CSCAN_JWT_SECRET" ]; then
    # 用户通过环境变量指定了密钥，最高优先级（多副本部署必须显式指定同一密钥）
    echo "[entrypoint] Using CSCAN_JWT_SECRET from environment variable"
    echo -n "$CSCAN_JWT_SECRET" > "$JWT_SECRET_FILE" 2>/dev/null || true
elif [ -f "$JWT_SECRET_FILE" ]; then
    # 从持久化文件恢复（单实例重启保持 token 有效）
    export CSCAN_JWT_SECRET="$(cat "$JWT_SECRET_FILE")"
    echo "[entrypoint] Loaded CSCAN_JWT_SECRET from persistent storage"
else
    # 首次启动且未指定：生成随机密钥并持久化（仅适合单实例；多副本请显式指定）
    CSCAN_JWT_SECRET="$(head -c 48 /dev/urandom | base64 | tr -d '\n/+=' | head -c 64)"
    echo -n "$CSCAN_JWT_SECRET" > "$JWT_SECRET_FILE"
    chmod 600 "$JWT_SECRET_FILE"
    export CSCAN_JWT_SECRET
    echo "[entrypoint] Generated new CSCAN_JWT_SECRET and saved to persistent storage (single-instance only)"
fi

exec "$@"
