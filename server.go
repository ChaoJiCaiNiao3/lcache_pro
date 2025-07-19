package lcache_pro

import (
	"context"
	"fmt"
	"net"

	pb "github.com/ChaoJiCaiNiao3/lcache_pro/pb"
	"github.com/ChaoJiCaiNiao3/lcache_pro/registry"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type Server struct {
	pb.UnimplementedLcacheProServer
	svcName      string        //服务名称
	selfAddr     string        //本节点地址
	clientPicker *ClientPicker //节点选择器，用来返回到底要哪个节点
	groups       *Group        //缓存组，用来管理缓存
	stopCh       chan struct{} //停止信号
}

func NewServer(selfAddr string, svcName string) *Server {
	stopCh := make(chan struct{})
	return &Server{
		svcName:  svcName,
		selfAddr: selfAddr,
		stopCh:   stopCh,
	}
}

func (s *Server) Start() error {
	if s.clientPicker == nil {
		return fmt.Errorf("clientPicker is nil")
	}
	if s.groups == nil {
		return fmt.Errorf("groups is nil")
	}
	//注册服务
	registry.Register(s.svcName, s.selfAddr, s.stopCh)

	//发起grpc监听
	lis, err := net.Listen("tcp", s.selfAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterLcacheProServer(grpcServer, s)
	//健康注册
	healthServer := health.NewServer()
	healthServer.SetServingStatus(s.svcName, healthpb.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus(s.svcName, healthpb.HealthCheckResponse_NOT_SERVING)
	//启动监听
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logrus.Errorf("failed to serve: %v", err)
		}
	}()

	return nil
}

func (s *Server) Stop() error {
	close(s.stopCh)
	return nil
}

func RegisterPeersToServer(picker *ClientPicker, server *Server) {
	server.clientPicker = picker
}

func RegisterGroupToServer(server *Server, group *Group) {
	server.groups = group
}

func (s *Server) Get(ctx context.Context, req *pb.Request) (*pb.ResponseForGet, error) {
	peer, ok, self := s.clientPicker.PickPeer(req.Key)
	if !ok || peer == "" {
		return nil, fmt.Errorf("failed to pick peer")
	}
	if self {
		value, ok := s.groups.cache.Get(req.Key)
		if !ok {
			return &pb.ResponseForGet{Value: nil}, nil
		}
		return &pb.ResponseForGet{Value: value.(ByteView)}, nil
	} else {
		//从哈希环对应节点拿
		resp, err := s.clientPicker.grpcCli[peer].Get(context.Background(), req)
		if err != nil {
			return nil, fmt.Errorf("failed to get: %v", err)
		}
		return resp, nil
	}
}

func (s *Server) Set(ctx context.Context, req *pb.Request) (*pb.ResponseForGet, error) {
	// TODO: 实现 Set 逻辑
	return nil, fmt.Errorf("Set not implemented")
}

func (s *Server) Delete(ctx context.Context, req *pb.Request) (*pb.ResponseForDelete, error) {
	// TODO: 实现 Delete 逻辑
	return nil, fmt.Errorf("Delete not implemented")
}
