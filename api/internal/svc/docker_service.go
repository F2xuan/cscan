package svc

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"cscan/api/internal/config"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/zeromicro/go-zero/core/logx"
)

// DockerService 封装 Docker SDK,提供 cscan 相关容器发现与日志读取能力
type DockerService struct {
	cli        *client.Client
	prefix     string
	registry   string
	extraNames map[string]struct{}
}

// ContainerLogLine 单行容器日志
type ContainerLogLine struct {
	Stream string `json:"stream"`
	TS     string `json:"ts"`
	Line   string `json:"line"`
}

// ContainerInfo 对外暴露的容器摘要
type ContainerInfo struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Image  string            `json:"image"`
	State  string            `json:"state"`
	Status string            `json:"status"`
	Ports  []types.Port      `json:"ports"`
	Labels map[string]string `json:"labels,omitempty"`
}

// NewDockerService 构造 DockerService;若 Docker 守护不可达,返回错误
func NewDockerService(cfg config.DockerConfig) (*DockerService, error) {
	opts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}
	if cfg.Host != "" {
		opts = []client.Opt{client.WithHost(cfg.Host), client.WithAPIVersionNegotiation()}
	}
	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := cli.Ping(ctx); err != nil {
		_ = cli.Close()
		return nil, fmt.Errorf("ping docker daemon: %w", err)
	}
	prefix := cfg.ContainerPrefix
	if prefix == "" {
		prefix = "cscan"
	}
	extras := make(map[string]struct{}, len(cfg.ExtraNames))
	for _, n := range cfg.ExtraNames {
		if n != "" {
			extras[n] = struct{}{}
		}
	}
	return &DockerService{cli: cli, prefix: prefix, registry: cfg.ImageRegistry, extraNames: extras}, nil
}

func (s *DockerService) Close() {
	if s != nil && s.cli != nil {
		_ = s.cli.Close()
	}
}

// isCscanContainer 根据名称前缀/镜像仓库/额外名单判断
func (s *DockerService) isCscanContainer(name string, image string) bool {
	if _, ok := s.extraNames[name]; ok {
		return true
	}
	if strings.HasPrefix(name, s.prefix+"-") || strings.HasPrefix(name, s.prefix+"_") {
		return true
	}
	if s.registry != "" && strings.HasPrefix(image, s.registry) {
		return true
	}
	return false
}

// swarmReplicaSuffix 匹配 Docker Swarm 任务容器名后缀：.<replica>.<task_id>
// 例如 cscan_api.1.a1b2c3d4e5f6 -> 匹配 ".1.a1b2c3d4e5f6"
// 非 swarm 普通容器名（如 cscan_api）不匹配，保持原样。
var swarmReplicaSuffix = regexp.MustCompile(`\.\d+\.[0-9a-f]{8,}$`)

// normalizeContainerName 剥离 Docker Swarm 任务容器名中的副本后缀，
// 将 cscan_api.1.a1b2c3d4e5f6 归一化为 cscan_api，方便日志聚合与展示。
// 普通容器名原样返回。
func normalizeContainerName(name string) string {
	return swarmReplicaSuffix.ReplaceAllString(name, "")
}

// ListCscanContainers 列出所有 cscan 相关容器(含已停止)
func (s *DockerService) ListCscanContainers(ctx context.Context) ([]ContainerInfo, error) {
	list, err := s.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}
	out := make([]ContainerInfo, 0, len(list))
	for _, c := range list {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		name = normalizeContainerName(name)
		if !s.isCscanContainer(name, c.Image) {
			continue
		}
		out = append(out, ContainerInfo{
			ID:     c.ID,
			Name:   name,
			Image:  c.Image,
			State:  c.State,
			Status: c.Status,
			Ports:  c.Ports,
			Labels: c.Labels,
		})
	}
	return out, nil
}

// FetchLogs 一次性拉取最近 N 行日志
func (s *DockerService) FetchLogs(ctx context.Context, name, tail, since string) ([]ContainerLogLine, error) {
	if tail == "" {
		tail = "1000"
	}
	reader, err := s.cli.ContainerLogs(ctx, name, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     false,
		Tail:       tail,
		Since:      since,
		Timestamps: true,
	})
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return parseLogStream(reader), nil
}

// StreamLogs 阻塞推送实时日志事件,直到 ctx 取消或容器停止
func (s *DockerService) StreamLogs(ctx context.Context, name, tail, since string, onLine func(ContainerLogLine), onEnd func(string)) error {
	if tail == "" {
		tail = "1000"
	}
	reader, err := s.cli.ContainerLogs(ctx, name, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       tail,
		Since:      since,
		Timestamps: true,
	})
	if err != nil {
		return err
	}
	defer reader.Close()

	// 使用 Docker 官方 stdcopy.StdCopy 进行多路复用解码(修复 H2):
	//   - 原手写实现逐字节读取、不校验 stream-type、1MiB 截断后会丢失同行的剩余字节;
	//   - StdCopy 内部以 io.ReadFull 读满帧体, 与 SDK 行为对齐;
	//   - 按 stream 路由到 lineWriter,onLine 按整行回调,跨帧拼接由 bufio 切分逻辑保证。
	out := newLineWriter("stdout", onLine)
	errW := newLineWriter("stderr", onLine)
	if _, copyErr := stdcopy.StdCopy(out, errW, reader); copyErr != nil {
		if ctx.Err() != nil {
			return nil
		}
		if copyErr == io.EOF {
			if onEnd != nil {
				onEnd("container stopped or stream closed")
			}
			return nil
		}
		logx.Errorf("[ContainerLogs] stdcopy err: %v", copyErr)
		return copyErr
	}
	if onEnd != nil {
		onEnd("container stopped or stream closed")
	}
	out.flush()
	errW.flush()
	return nil
}

// splitDockerTimestamp 拆分 docker Timestamps:true 格式: `2026-07-24T10:11:12.345678901Z <body>`
func splitDockerTimestamp(line string) (ts string, body string) {
	idx := strings.IndexByte(line, ' ')
	if idx <= 0 {
		return "", line
	}
	return line[:idx], line[idx+1:]
}

// parseLogStream 从静态日志流同步解析所有行
func parseLogStream(r io.Reader) []ContainerLogLine {
	out := make([]ContainerLogLine, 0, 256)
	collect := func(line ContainerLogLine) { out = append(out, line) }
	w := newLineWriter("stdout", collect)
	errW := newLineWriter("stderr", collect)
	if _, err := stdcopy.StdCopy(w, errW, r); err != nil && err != io.EOF {
		logx.Errorf("[ContainerLogs] parseLogStream stdcopy err: %v", err)
	}
	w.flush()
	errW.flush()
	return out
}

// lineWriter 按 \n 切分 stdcopy 输出,按整行回调 onLine。
// stdcopy.StdCopy 保证写到 writer 的内容限于单个 stream,但一次 Write 可能是半行、多行、或跨帧延续,
// 因此内部用 bufio 风格的 carry 缓冲跨帧拼接,遇到 \n 触发回调。
//
// 1MiB 上限保护:单个超长行(无 \n)在达到 1MiB 时强制 flush,避免内存膨胀;
// 这与原"丢掉剩余字节"的 bug 不同,StdCopy 写入是按帧体的,1MiB 后下一帧仍被正确解码。
const maxLineBytes = 1 << 20

type lineWriter struct {
	stream string
	onLine func(ContainerLogLine)
	carry  []byte
	mu     sync.Mutex
}

func newLineWriter(stream string, onLine func(ContainerLogLine)) *lineWriter {
	return &lineWriter{stream: stream, onLine: onLine}
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	wrote := 0
	for {
		nl := indexByte(p, '\n')
		if nl < 0 {
			w.carry = append(w.carry, p...)
			wrote += len(p)
			if len(w.carry) >= maxLineBytes {
				w.flushLocked()
			}
			return wrote, nil
		}
		w.carry = append(w.carry, p[:nl]...)
		w.flushLocked()
		wrote += nl + 1
		p = p[nl+1:]
	}
}

func (w *lineWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.flushLocked()
}

func (w *lineWriter) flushLocked() {
	if len(w.carry) == 0 {
		return
	}
	line := string(w.carry)
	w.carry = w.carry[:0]
	ts, body := splitDockerTimestamp(line)
	if w.onLine != nil {
		w.onLine(ContainerLogLine{Stream: w.stream, TS: ts, Line: body})
	}
}

// indexByte 复刻 strings.IndexByte,避免在 hot 路径引入额外分配
func indexByte(p []byte, b byte) int {
	for i, c := range p {
		if c == b {
			return i
		}
	}
	return -1
}
