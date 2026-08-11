#!/bin/sh

# RPC 服务启动脚本：替换 task.yaml 中的环境变量占位符
envsubst '${CSCAN_MONGO_URI} ${CSCAN_REDIS_PASSWORD}' \
    < /app/etc/task.yaml.template > /app/etc/task.yaml

# 执行传入的命令
exec "$@"
