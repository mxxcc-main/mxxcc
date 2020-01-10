// Copyright 2016 The go-ccmchain Authors
// This file is part of the go-ccmchain library.
//
// The go-ccmchain library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ccmchain library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ccmchain library. If not, see <http://www.gnu.org/licenses/>.

// Package les implements the Light Ccmchain Subprotocol.
package les

import (
	"fmt"
	"sync"
	"time"

	"github.com/ccmchain/go-ccmchain/accounts"
	"github.com/ccmchain/go-ccmchain/accounts/abi/bind"
	"github.com/ccmchain/go-ccmchain/common"
	"github.com/ccmchain/go-ccmchain/common/hexutil"
	"github.com/ccmchain/go-ccmchain/common/mclock"
	"github.com/ccmchain/go-ccmchain/consensus"
	"github.com/ccmchain/go-ccmchain/core"
	"github.com/ccmchain/go-ccmchain/core/bloombits"
	"github.com/ccmchain/go-ccmchain/core/rawdb"
	"github.com/ccmchain/go-ccmchain/core/types"
	"github.com/ccmchain/go-ccmchain/ccm"
	"github.com/ccmchain/go-ccmchain/ccm/downloader"
	"github.com/ccmchain/go-ccmchain/ccm/filters"
	"github.com/ccmchain/go-ccmchain/ccm/gasprice"
	"github.com/ccmchain/go-ccmchain/event"
	"github.com/ccmchain/go-ccmchain/internal/ccmapi"
	"github.com/ccmchain/go-ccmchain/light"
	"github.com/ccmchain/go-ccmchain/log"
	"github.com/ccmchain/go-ccmchain/node"
	"github.com/ccmchain/go-ccmchain/p2p"
	"github.com/ccmchain/go-ccmchain/p2p/discv5"
	"github.com/ccmchain/go-ccmchain/params"
	"github.com/ccmchain/go-ccmchain/rpc"
)

type LightCcmchain struct {
	lesCommons

	odr         *LesOdr
	chainConfig *params.ChainConfig
	// Channel for shutting down the service
	shutdownChan chan bool

	// Handlers
	peers      *peerSet
	txPool     *light.TxPool
	blockchain *light.LightChain
	serverPool *serverPool
	reqDist    *requestDistributor
	retriever  *retrieveManager
	relay      *lesTxRelay

	bloomRequests chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
	bloomIndexer  *core.ChainIndexer

	ApiBackend *LesApiBackend

	eventMux       *event.TypeMux
	engine         consensus.Engine
	accountManager *accounts.Manager

	networkId     uint64
	netRPCService *ccmapi.PublicNetAPI

	wg sync.WaitGroup
}

func New(ctx *node.ServiceContext, config *ccm.Config) (*LightCcmchain, error) {
	chainDb, err := ctx.OpenDatabase("lightchaindata", config.DatabaseCache, config.DatabaseHandles, "ccm/db/chaindata/")
	if err != nil {
		return nil, err
	}
	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlock(chainDb, config.Genesis)
	if _, isCompat := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !isCompat {
		return nil, genesisErr
	}
	log.Info("Initialised chain configuration", "config", chainConfig)

	peers := newPeerSet()
	quitSync := make(chan struct{})

	lccm := &LightCcmchain{
		lesCommons: lesCommons{
			chainDb: chainDb,
			config:  config,
			iConfig: light.DefaultClientIndexerConfig,
		},
		chainConfig:    chainConfig,
		eventMux:       ctx.EventMux,
		peers:          peers,
		reqDist:        newRequestDistributor(peers, quitSync, &mclock.System{}),
		accountManager: ctx.AccountManager,
		engine:         ccm.CreateConsensusEngine(ctx, chainConfig, &config.Ethash, nil, false, chainDb),
		shutdownChan:   make(chan bool),
		networkId:      config.NetworkId,
		bloomRequests:  make(chan chan *bloombits.Retrieval),
		bloomIndexer:   ccm.NewBloomIndexer(chainDb, params.BloomBitsBlocksClient, params.HelperTrieConfirmations),
	}
	lccm.serverPool = newServerPool(chainDb, quitSync, &lccm.wg, lccm.config.UltraLightServers)
	lccm.retriever = newRetrieveManager(peers, lccm.reqDist, lccm.serverPool)
	lccm.relay = newLesTxRelay(peers, lccm.retriever)

	lccm.odr = NewLesOdr(chainDb, light.DefaultClientIndexerConfig, lccm.retriever)
	lccm.chtIndexer = light.NewChtIndexer(chainDb, lccm.odr, params.CHTFrequency, params.HelperTrieConfirmations)
	lccm.bloomTrieIndexer = light.NewBloomTrieIndexer(chainDb, lccm.odr, params.BloomBitsBlocksClient, params.BloomTrieFrequency)
	lccm.odr.SetIndexers(lccm.chtIndexer, lccm.bloomTrieIndexer, lccm.bloomIndexer)

	checkpoint := config.Checkpoint
	if checkpoint == nil {
		checkpoint = params.TrustedCheckpoints[genesisHash]
	}
	// Note: NewLightChain adds the trusted checkpoint so it needs an ODR with
	// indexers already set but not started yet
	if lccm.blockchain, err = light.NewLightChain(lccm.odr, lccm.chainConfig, lccm.engine, checkpoint); err != nil {
		return nil, err
	}
	// Note: AddChildIndexer starts the update process for the child
	lccm.bloomIndexer.AddChildIndexer(lccm.bloomTrieIndexer)
	lccm.chtIndexer.Start(lccm.blockchain)
	lccm.bloomIndexer.Start(lccm.blockchain)

	// Rewind the chain in case of an incompatible config upgrade.
	if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
		log.Warn("Rewinding chain to upgrade configuration", "err", compat)
		lccm.blockchain.SetHead(compat.RewindTo)
		rawdb.WriteChainConfig(chainDb, genesisHash, chainConfig)
	}

	lccm.txPool = light.NewTxPool(lccm.chainConfig, lccm.blockchain, lccm.relay)
	lccm.ApiBackend = &LesApiBackend{ctx.ExtRPCEnabled(), lccm, nil}

	gpoParams := config.GPO
	if gpoParams.Default == nil {
		gpoParams.Default = config.Miner.GasPrice
	}
	lccm.ApiBackend.gpo = gasprice.NewOracle(lccm.ApiBackend, gpoParams)

	oracle := config.CheckpointOracle
	if oracle == nil {
		oracle = params.CheckpointOracles[genesisHash]
	}
	registrar := newCheckpointOracle(oracle, lccm.getLocalCheckpoint)
	if lccm.protocolManager, err = NewProtocolManager(lccm.chainConfig, checkpoint, light.DefaultClientIndexerConfig, config.UltraLightServers, config.UltraLightFraction, true, config.NetworkId, lccm.eventMux, lccm.peers, lccm.blockchain, nil, chainDb, lccm.odr, lccm.serverPool, registrar, quitSync, &lccm.wg, nil); err != nil {
		return nil, err
	}
	if lccm.protocolManager.ulc != nil {
		log.Warn("Ultra light client is enabled", "servers", len(config.UltraLightServers), "fraction", config.UltraLightFraction)
		lccm.blockchain.DisableCheckFreq()
	}
	return lccm, nil
}

func lesTopic(genesisHash common.Hash, protocolVersion uint) discv5.Topic {
	var name string
	switch protocolVersion {
	case lpv2:
		name = "LES2"
	default:
		panic(nil)
	}
	return discv5.Topic(name + "@" + common.Bytes2Hex(genesisHash.Bytes()[0:8]))
}

type LightDummyAPI struct{}

// Ccmchainbase is the address that mining rewards will be send to
func (s *LightDummyAPI) Ccmchainbase() (common.Address, error) {
	return common.Address{}, fmt.Errorf("mining is not supported in light mode")
}

// Coinbase is the address that mining rewards will be send to (alias for Ccmchainbase)
func (s *LightDummyAPI) Coinbase() (common.Address, error) {
	return common.Address{}, fmt.Errorf("mining is not supported in light mode")
}

// Hashrate returns the POW hashrate
func (s *LightDummyAPI) Hashrate() hexutil.Uint {
	return 0
}

// Mining returns an indication if this node is currently mining.
func (s *LightDummyAPI) Mining() bool {
	return false
}

// APIs returns the collection of RPC services the ccmchain package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (s *LightCcmchain) APIs() []rpc.API {
	return append(ccmapi.GetAPIs(s.ApiBackend), []rpc.API{
		{
			Namespace: "ccm",
			Version:   "1.0",
			Service:   &LightDummyAPI{},
			Public:    true,
		}, {
			Namespace: "ccm",
			Version:   "1.0",
			Service:   downloader.NewPublicDownloaderAPI(s.protocolManager.downloader, s.eventMux),
			Public:    true,
		}, {
			Namespace: "ccm",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(s.ApiBackend, true),
			Public:    true,
		}, {
			Namespace: "net",
			Version:   "1.0",
			Service:   s.netRPCService,
			Public:    true,
		}, {
			Namespace: "les",
			Version:   "1.0",
			Service:   NewPrivateLightAPI(&s.lesCommons, s.protocolManager.reg),
			Public:    false,
		},
	}...)
}

func (s *LightCcmchain) ResetWithGenesisBlock(gb *types.Block) {
	s.blockchain.ResetWithGenesisBlock(gb)
}

func (s *LightCcmchain) BlockChain() *light.LightChain      { return s.blockchain }
func (s *LightCcmchain) TxPool() *light.TxPool              { return s.txPool }
func (s *LightCcmchain) Engine() consensus.Engine           { return s.engine }
func (s *LightCcmchain) LesVersion() int                    { return int(ClientProtocolVersions[0]) }
func (s *LightCcmchain) Downloader() *downloader.Downloader { return s.protocolManager.downloader }
func (s *LightCcmchain) EventMux() *event.TypeMux           { return s.eventMux }

// Protocols implements node.Service, returning all the currently configured
// network protocols to start.
func (s *LightCcmchain) Protocols() []p2p.Protocol {
	return s.makeProtocols(ClientProtocolVersions)
}

// Start implements node.Service, starting all internal goroutines needed by the
// Ccmchain protocol implementation.
func (s *LightCcmchain) Start(srvr *p2p.Server) error {
	log.Warn("Light client mode is an experimental feature")
	s.startBloomHandlers(params.BloomBitsBlocksClient)
	s.netRPCService = ccmapi.NewPublicNetAPI(srvr, s.networkId)
	// clients are searching for the first advertised protocol in the list
	protocolVersion := AdvertiseProtocolVersions[0]
	s.serverPool.start(srvr, lesTopic(s.blockchain.Genesis().Hash(), protocolVersion))
	s.protocolManager.Start(s.config.LightPeers)
	return nil
}

// Stop implements node.Service, terminating all internal goroutines used by the
// Ccmchain protocol.
func (s *LightCcmchain) Stop() error {
	s.odr.Stop()
	s.relay.Stop()
	s.bloomIndexer.Close()
	s.chtIndexer.Close()
	s.blockchain.Stop()
	s.protocolManager.Stop()
	s.txPool.Stop()
	s.engine.Close()

	s.eventMux.Stop()

	time.Sleep(time.Millisecond * 200)
	s.chainDb.Close()
	close(s.shutdownChan)

	return nil
}

// SetClient sets the rpc client and binds the registrar contract.
func (s *LightCcmchain) SetContractBackend(backend bind.ContractBackend) {
	// Short circuit if registrar is nil
	if s.protocolManager.reg == nil {
		return
	}
	s.protocolManager.reg.start(backend)
}
