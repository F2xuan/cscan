package main

import (
	"flag"
	"fmt"

	"cscan/pkg"
	"cscan/rpc/task/internal/config"
	"cscan/rpc/task/internal/server"
	"cscan/rpc/task/internal/svc"
	"cscan/rpc/task/pb"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/task.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	logx.MustSetup(c.Log)
	logx.DisableStat()

	// 安装日志过滤器（RPC服务端不需要过滤HTTP access log，但保留接口一致性）
	originalWriter := logx.Reset()
	filteredWriter := pkg.NewFilteredLogWriter(originalWriter, []string{})
	logx.SetWriter(filteredWriter)

	fmt.Println(`
   ______ _____  ______          _   _ 
  / ____/ ____|/ __ \ \        / / | \ | |
 | |   | (___ | |  | \ \  /\  / /|  \| |
 | |    \___ \| |  | |\ \/  \/ / | .  |
 | |________) | |__| | \  /\  /  | |\  |
  \_____|_____/ \____/   \/  \/   |_| \_| 
                  RPC SERVICE            `)
	fmt.Println("---------------------------------------------------------")
	logx.Info("CScan Task RPC Service Starting")
	fmt.Println("---------------------------------------------------------")
	ctx, err := svc.NewServiceContext(c)
	if err != nil {
		logx.Errorf("Failed to initialize service: %v", err)
		return
	}

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterTaskServiceServer(grpcServer, server.NewTaskServiceServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})

	s.AddOptions(
		grpc.MaxRecvMsgSize(50*1024*1024), // 50MB
		grpc.MaxSendMsgSize(50*1024*1024), // 50MB
	)
	defer s.Stop()

	logx.Infof("RPC Server listening at %s", c.ListenOn)
	s.Start()
}
