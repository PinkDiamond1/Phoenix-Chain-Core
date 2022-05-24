package adapters

import (
	"errors"
	"fmt"
	"math"
	"net"
	"sync"

	"Phoenix-Chain-Core/libs/event"
	"Phoenix-Chain-Core/libs/log"
	"Phoenix-Chain-Core/ethereum/node"
	"Phoenix-Chain-Core/ethereum/p2p"
	"Phoenix-Chain-Core/ethereum/p2p/discover"
	"Phoenix-Chain-Core/ethereum/p2p/simulations/pipes"
	"Phoenix-Chain-Core/libs/rpc"

	"github.com/gorilla/websocket"
)

// SimAdapter is a NodeAdapter which creates in-memory simulation nodes and
// connects them using net.Pipe
type SimAdapter struct {
	pipe     func() (net.Conn, net.Conn, error)
	mtx      sync.RWMutex
	nodes    map[discover.NodeID]*SimNode
	services map[string]ServiceFunc
}

// NewSimAdapter creates a SimAdapter which is capable of running in-memory
// simulation nodes running any of the given services (the services to run on a
// particular node are passed to the NewNode function in the NodeConfig)
// the adapter uses a net.Pipe for in-memory simulated network connections
func NewSimAdapter(services map[string]ServiceFunc) *SimAdapter {
	return &SimAdapter{
		pipe:     pipes.NetPipe,
		nodes:    make(map[discover.NodeID]*SimNode),
		services: services,
	}
}

func NewTCPAdapter(services map[string]ServiceFunc) *SimAdapter {
	return &SimAdapter{
		pipe:     pipes.TCPPipe,
		nodes:    make(map[discover.NodeID]*SimNode),
		services: services,
	}
}

// Name returns the name of the adapter for logging purposes
func (s *SimAdapter) Name() string {
	return "sim-adapter"
}

// NewNode returns a new SimNode using the given config
func (s *SimAdapter) NewNode(config *NodeConfig) (Node, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	// check a node with the ID doesn't already exist
	id := config.ID
	if _, exists := s.nodes[id]; exists {
		return nil, fmt.Errorf("node already exists: %s", id)
	}

	// check the services are valid
	if len(config.Services) == 0 {
		return nil, errors.New("node must have at least one service")
	}
	for _, service := range config.Services {
		if _, exists := s.services[service]; !exists {
			return nil, fmt.Errorf("unknown node service %q", service)
		}
	}

	n, err := node.New(&node.Config{
		P2P: p2p.Config{
			PrivateKey:      config.PrivateKey,
			MaxPeers:        math.MaxInt32,
			NoDiscovery:     true,
			Dialer:          s,
			EnableMsgEvents: config.EnableMsgEvents,
		},
		NoUSB:  true,
		Logger: log.New("node.id", id.String()),
	})
	if err != nil {
		return nil, err
	}

	simNode := &SimNode{
		ID:      id,
		config:  config,
		node:    n,
		adapter: s,
		running: make(map[string]node.Service),
	}
	s.nodes[id] = simNode
	return simNode, nil
}

// Dial implements the p2p.NodeDialer interface by connecting to the node using
// an in-memory net.Pipe
func (s *SimAdapter) Dial(dest *discover.Node) (conn net.Conn, err error) {
	node, ok := s.GetNode(dest.ID)
	if !ok {
		return nil, fmt.Errorf("unknown node: %s", dest.ID)
	}
	srv := node.Server()
	if srv == nil {
		return nil, fmt.Errorf("node not running: %s", dest.ID)
	}
	// SimAdapter.pipe is net.Pipe (NewSimAdapter)
	pipe1, pipe2, err := s.pipe()
	if err != nil {
		return nil, err
	}
	// this is simulated 'listening'
	// asynchronously call the dialed destintion node's p2p server
	// to set up connection on the 'listening' side
	go srv.SetupConn(pipe1, 0, nil)
	return pipe2, nil
}

// DialRPC implements the RPCDialer interface by creating an in-memory RPC
// client of the given node
func (s *SimAdapter) DialRPC(id discover.NodeID) (*rpc.Client, error) {
	node, ok := s.GetNode(id)
	if !ok {
		return nil, fmt.Errorf("unknown node: %s", id)
	}
	handler, err := node.node.RPCHandler()
	if err != nil {
		return nil, err
	}
	return rpc.DialInProc(handler), nil
}

// GetNode returns the node with the given ID if it exists
func (s *SimAdapter) GetNode(id discover.NodeID) (*SimNode, bool) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	node, ok := s.nodes[id]
	return node, ok
}

// SimNode is an in-memory simulation node which connects to other nodes using
// net.Pipe (see SimAdapter.Dial), running devp2p protocols directly over that
// pipe
type SimNode struct {
	lock         sync.RWMutex
	ID           discover.NodeID
	config       *NodeConfig
	adapter      *SimAdapter
	node         *node.Node
	running      map[string]node.Service
	client       *rpc.Client
	registerOnce sync.Once
}

// Close closes the underlaying node.Node to release
// acquired resources.
func (sn *SimNode) Close() error {
	return sn.node.Close()
}

// Addr returns the node's discovery address
func (sn *SimNode) Addr() []byte {
	return []byte(sn.Node().String())
}

// Node returns a discover.Node representing the SimNode
func (sn *SimNode) Node() *discover.Node {
	return discover.NewNode(sn.ID, net.IP{127, 0, 0, 1}, 16789, 16789)
}

// Client returns an rpc.Client which can be used to communicate with the
// underlying services (it is set once the node has started)
func (sn *SimNode) Client() (*rpc.Client, error) {
	sn.lock.RLock()
	defer sn.lock.RUnlock()
	if sn.client == nil {
		return nil, errors.New("node not started")
	}
	return sn.client, nil
}

// ServeRPC serves RPC requests over the given connection by creating an
// in-memory client to the node's RPC server
func (sn *SimNode) ServeRPC(conn *websocket.Conn) error {
	handler, err := sn.node.RPCHandler()
	if err != nil {
		return err
	}
	codec := rpc.NewFuncCodec(conn, conn.WriteJSON, conn.ReadJSON)
	handler.ServeCodec(codec, 0)
	return nil
}

// Snapshots creates snapshots of the services by calling the
// simulation_snapshot RPC method
func (sn *SimNode) Snapshots() (map[string][]byte, error) {
	sn.lock.RLock()
	services := make(map[string]node.Service, len(sn.running))
	for name, service := range sn.running {
		services[name] = service
	}
	sn.lock.RUnlock()
	if len(services) == 0 {
		return nil, errors.New("no running services")
	}
	snapshots := make(map[string][]byte)
	for name, service := range services {
		if s, ok := service.(interface {
			Snapshot() ([]byte, error)
		}); ok {
			snap, err := s.Snapshot()
			if err != nil {
				return nil, err
			}
			snapshots[name] = snap
		}
	}
	return snapshots, nil
}

// Start registers the services and starts the underlying devp2p node
func (sn *SimNode) Start(snapshots map[string][]byte) error {
	newService := func(name string) func(ctx *node.ServiceContext) (node.Service, error) {
		return func(nodeCtx *node.ServiceContext) (node.Service, error) {
			ctx := &ServiceContext{
				RPCDialer:   sn.adapter,
				NodeContext: nodeCtx,
				Config:      sn.config,
			}
			if snapshots != nil {
				ctx.Snapshot = snapshots[name]
			}
			serviceFunc := sn.adapter.services[name]
			service, err := serviceFunc(ctx)
			if err != nil {
				return nil, err
			}
			sn.running[name] = service
			return service, nil
		}
	}

	// ensure we only register the services once in the case of the node
	// being stopped and then started again
	var regErr error
	sn.registerOnce.Do(func() {
		for _, name := range sn.config.Services {
			if err := sn.node.Register(newService(name)); err != nil {
				regErr = err
				break
			}
		}
	})
	if regErr != nil {
		return regErr
	}

	if err := sn.node.Start(); err != nil {
		return err
	}

	// create an in-process RPC client
	handler, err := sn.node.RPCHandler()
	if err != nil {
		return err
	}

	sn.lock.Lock()
	sn.client = rpc.DialInProc(handler)
	sn.lock.Unlock()

	return nil
}

// Stop closes the RPC client and stops the underlying devp2p node
func (sn *SimNode) Stop() error {
	sn.lock.Lock()
	if sn.client != nil {
		sn.client.Close()
		sn.client = nil
	}
	sn.lock.Unlock()
	return sn.node.Stop()
}

// Service returns a running service by name
func (sn *SimNode) Service(name string) node.Service {
	sn.lock.RLock()
	defer sn.lock.RUnlock()
	return sn.running[name]
}

// Services returns a copy of the underlying services
func (sn *SimNode) Services() []node.Service {
	sn.lock.RLock()
	defer sn.lock.RUnlock()
	services := make([]node.Service, 0, len(sn.running))
	for _, service := range sn.running {
		services = append(services, service)
	}
	return services
}

// ServiceMap returns a map by names of the underlying services
func (sn *SimNode) ServiceMap() map[string]node.Service {
	sn.lock.RLock()
	defer sn.lock.RUnlock()
	services := make(map[string]node.Service, len(sn.running))
	for name, service := range sn.running {
		services[name] = service
	}
	return services
}

// Server returns the underlying p2p.Server
func (sn *SimNode) Server() *p2p.Server {
	return sn.node.Server()
}

// SubscribeEvents subscribes the given channel to peer events from the
// underlying p2p.Server
func (sn *SimNode) SubscribeEvents(ch chan *p2p.PeerEvent) event.Subscription {
	srv := sn.Server()
	if srv == nil {
		panic("node not running")
	}
	return srv.SubscribeEvents(ch)
}

// NodeInfo returns information about the node
func (sn *SimNode) NodeInfo() *p2p.NodeInfo {
	server := sn.Server()
	if server == nil {
		return &p2p.NodeInfo{
			ID:    sn.ID.String(),
			Enode: sn.Node().String(),
		}
	}
	return server.NodeInfo()
}
