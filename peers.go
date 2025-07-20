package lcache_pro

import (
	"context"
	"fmt"
	"time"

	"github.com/ChaoJiCaiNiao3/lcache_pro/consistenthash"
	pb "github.com/ChaoJiCaiNiao3/lcache_pro/pb"
	"github.com/ChaoJiCaiNiao3/lcache_pro/registry"
	"github.com/sirupsen/logrus"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// PeerPicker 定义了peer选择器的接口
type PeerPicker interface {
	PickPeer(key string) (peer Peer, ok bool, self bool)
	Close() error
}

// Peer 定义了缓存节点的接口
type Peer interface {
	Get(group string, key string) ([]byte, error)
	Set(ctx context.Context, group string, key string, value []byte) error
	Delete(group string, key string) (bool, error)
	Close() error
}

type ClientPicker struct {
	selfAddr string
	svcName  string
	etcdCli  *clientv3.Client
	consHash *consistenthash.Map
	ctx      context.Context
	cancel   context.CancelFunc
	grpcCli  map[string]pb.LcacheProClient
}

// PickerOption 定义配置选项
type PickerOption func(*ClientPicker)

func NewClientPicker(selfAddr string, svcName string, consHash *consistenthash.Map, opts ...PickerOption) (*ClientPicker, error) {
	ctx, cancel := context.WithCancel(context.Background())
	if consHash == nil {
		consHash = consistenthash.NewConsistentHash()
	}
	picker := &ClientPicker{
		selfAddr: selfAddr,
		svcName:  svcName,
		etcdCli:  nil,
		consHash: consHash,
		ctx:      ctx,
		cancel:   cancel,
	}
	for _, opt := range opts {
		opt(picker)
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   registry.DefaultConfig.Endpoints,
		DialTimeout: registry.DefaultConfig.DialTimeout,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create etcd client: %v", err)
	}
	picker.etcdCli = cli

	go picker.startServiceDiscovery()
	return picker, nil
}

func (p *ClientPicker) Close() error {
	//上下文关闭grpc连接
	p.cancel()
	p.etcdCli.Close()
	return nil
}

func (p *ClientPicker) startServiceDiscovery() error {
	// 先进行全量更新
	if err := p.fetchAllServices(); err != nil {
		return err
	}

	// 启动增量更新
	go p.watchServiceChanges()
	return nil
}

func (p *ClientPicker) fetchAllServices() error {
	ctx, cancel := context.WithTimeout(p.ctx, 3*time.Second)
	defer cancel()

	resp, err := p.etcdCli.Get(ctx, "/services/"+p.svcName, clientv3.WithPrefix())
	if err != nil {
		return fmt.Errorf("failed to get all services: %v", err)
	}

	for _, kv := range resp.Kvs {
		addr := string(kv.Value)
		if addr != "" {
			p.consHash.Add(addr)
			//连接grpc
			grpcCli, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return fmt.Errorf("failed to create grpc client: %v", err)
			}
			go func() {
				<-p.ctx.Done()
				grpcCli.Close()
			}()
			p.grpcCli[addr] = pb.NewLcacheProClient(grpcCli)
			logrus.Infof("added service: %s", addr)
		}
	}
	return nil
}

func (p *ClientPicker) watchServiceChanges() error {
	watchCh := p.etcdCli.Watch(p.ctx, "/services/"+p.svcName, clientv3.WithPrefix())

	for {
		select {
		case <-p.ctx.Done():
			return fmt.Errorf("picker context done")
		case resp, ok := <-watchCh:
			if !ok {
				return fmt.Errorf("watch channel closed")
			}
			p.handleWatchEvents(resp.Events)
		}
	}
}

func (p *ClientPicker) handleWatchEvents(events []*clientv3.Event) {
	for _, ev := range events {
		switch ev.Type {
		case clientv3.EventTypePut:
			addr := string(ev.Kv.Value)
			if addr != "" {
				//连接grpc
				grpcCli, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
				if err != nil {
					logrus.Errorf("failed to create grpc client: %v", err)
					continue
				}
				go func() {
					<-p.ctx.Done()
					grpcCli.Close()
				}()
				p.grpcCli[addr] = pb.NewLcacheProClient(grpcCli)
				p.consHash.Add(addr)
				logrus.Infof("added service: %s", addr)
			}
		case clientv3.EventTypeDelete:
			addr := string(ev.Kv.Key)
			if addr != "" {
				delete(p.grpcCli, addr)
				p.consHash.Remove(addr)
				logrus.Infof("removed service: %s", addr)
			}
		}
	}
}

func (p *ClientPicker) PickPeer(key string) (string, bool, bool) {
	peer := p.consHash.Get(key)
	if peer == "" {
		return "", false, false
	}
	return peer, true, peer == p.selfAddr
}
