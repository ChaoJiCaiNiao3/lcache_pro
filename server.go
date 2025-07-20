package lcache_pro

import (
	"bytes"
	"context"
	"fmt"
	"net"

	"github.com/ChaoJiCaiNiao3/lcache_pro/database"
	pb "github.com/ChaoJiCaiNiao3/lcache_pro/pb"
	"github.com/ChaoJiCaiNiao3/lcache_pro/registry"
	"github.com/ChaoJiCaiNiao3/lcache_pro/singleflight"
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
	singleflight *singleflight.Group
	redis        *database.Redis
	grpcServer   *grpc.Server
}

func NewServer(selfAddr string, svcName string) *Server {
	stopCh := make(chan struct{})
	redis := database.NewRedis(database.DefaultConfig)
	return &Server{
		svcName:      svcName,
		selfAddr:     selfAddr,
		stopCh:       stopCh,
		singleflight: &singleflight.Group{},
		redis:        redis,
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

func (s *Server) Close() error {
	close(s.stopCh)
	s.clientPicker.Close()
	s.groups.Close()
	s.redis.RedisClient.Close()
	s.grpcServer.GracefulStop()
	return nil
}

func RegisterPeersToServer(picker *ClientPicker, server *Server) {
	server.clientPicker = picker
}

func RegisterGroupToServer(group *Group, server *Server) {
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
			//没找到，单飞从数据库拿
			value, err := s.singleflight.Do(req.Key, func() (interface{}, error) {
				value, err := s.redis.Get(req.Key)
				if err != nil {
					logrus.Errorf("failed to get: %v", err)
					return nil, fmt.Errorf("failed to get: %v", err)
				}
				return ByteView(value), nil
			})
			if err != nil {
				return nil, fmt.Errorf("failed to get: %v", err)
			}
			s.groups.cache.Set(req.Key, value.(ByteView))
			return &pb.ResponseForGet{Value: value.(ByteView)}, nil
		}
		return &pb.ResponseForGet{Value: value.(ByteView)}, nil
	} else {
		//单飞从远程节点拿
		value, err := s.singleflight.Do(req.Key, func() (interface{}, error) {
			resp, err := s.clientPicker.grpcCli[peer].Get(context.Background(), req)
			if err != nil {
				return nil, fmt.Errorf("failed to get: %v", err)
			}
			return resp, nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get: %v", err)
		}
		return value.(*pb.ResponseForGet), nil
	}
}

func (s *Server) Set(ctx context.Context, req *pb.Request) (*pb.ResponseForGet, error) {
	//先寻找节点
	peer, ok, self := s.clientPicker.PickPeer(req.Key)
	if !ok || peer == "" {
		return nil, fmt.Errorf("failed to pick peer")
	}
	if self {
		s.redis.Set(req.Key, string(req.Value))
		s.groups.cache.Delete(req.Key)
	} else {
		s.clientPicker.grpcCli[peer].Set(context.Background(), req)
	}
	return &pb.ResponseForGet{Value: ByteView(req.Value)}, nil
}

func (s *Server) Delete(ctx context.Context, req *pb.Request) (*pb.ResponseForDelete, error) {
	peer, ok, self := s.clientPicker.PickPeer(req.Key)
	if !ok || peer == "" {
		return nil, fmt.Errorf("failed to pick peer")
	}
	if self {
		s.redis.Delete(req.Key)
		s.groups.cache.Delete(req.Key)
	} else {
		s.clientPicker.grpcCli[peer].Delete(context.Background(), req)
	}
	return &pb.ResponseForDelete{Value: true}, nil
}

// 设置热点数据时使用的函数
func (s *Server) SetToCache(key string, value ByteView) error {
	return s.groups.cache.Set(key, value)
}

// 封装grpc服务
func (s *Server) SetToCacheAndRedis(key string, value ByteView) error {
	flag, err := s.Set(context.Background(), &pb.Request{Key: key, Value: value})
	if err != nil || bytes.Equal(flag.Value, []byte{}) {
		return fmt.Errorf("failed to set: %v", err)
	}
	return nil
}

func (s *Server) DeleteCacheAndRedis(key string) error {
	flag, err := s.Delete(context.Background(), &pb.Request{Key: key})
	if err != nil || !flag.Value {
		return fmt.Errorf("failed to delete: %v", err)
	}
	return nil
}

func (s *Server) GetFromCacheAndRedis(key string) (ByteView, error) {
	value, err := s.Get(context.Background(), &pb.Request{Key: key})
	if err != nil {
		return nil, fmt.Errorf("failed to get: %v", err)
	}
	return value.Value, nil
}
