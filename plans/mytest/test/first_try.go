package test

import (
	"context"
	"fmt"

	ds "github.com/ipfs/go-datastore"

	bserv "github.com/ipfs/go-blockservice"
	syncds "github.com/ipfs/go-datastore/sync"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	dag "github.com/ipfs/go-merkledag"
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	crypto "github.com/libp2p/go-libp2p-crypto"
	core "github.com/textileio/go-threads/core/service"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/crypto/symmetric"
	tstore "github.com/textileio/go-threads/logstore/lstoremem"
	"github.com/textileio/go-threads/service"
	"github.com/textileio/go-threads/util"
)

func FirstTry(runenv *runtime.RunEnv) error {
	runenv.Message("Starting peer")

	api, addrinfo, err := makeService()
	if err != nil {
		return fmt.Errorf("making service: %s", err)
	}

	ctx := context.Background()
	watcher, writer := sync.MustWatcherWriter(ctx, runenv)
	defer watcher.Close()
	defer writer.Close()

	seq, err := writer.Write(ctx, sync.PeerSubtree, addrinfo)
	if err != nil {
		return fmt.Errorf("failed to write peer subtree in sync service: %w", err)
	}
	runenv.Message("My seq is %d and peer id %s", seq, addrinfo.ID)

	if seq == 1 {
		id := thread.NewIDV1(thread.Raw, 32)
		fk, err := symmetric.CreateKey()
		if err != nil {
			return fmt.Errorf("creating follower symkey: %s", err)
		}
		rk, err := symmetric.CreateKey()
		if err != nil {
			return fmt.Errorf("creating read symkey: %s", err)
		}
		info, err := api.CreateThread(ctx, id, core.FollowKey(fk), core.ReadKey(rk))
		if err != nil {
			return fmt.Errorf("creating thread: %s", err)
		}
		runenv.Message("Created thread %s", info.ID)
	}

	return nil
}

func makeService() (core.Service, *peer.AddrInfo, error) {
	sk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("generating ed25519 key: %s", err)
	}
	addr := util.MustParseAddr("/ip4/0.0.0.0/tcp/0")

	h, err := libp2p.New(
		context.Background(),
		libp2p.ListenAddrs(addr),
		libp2p.Identity(sk),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("creating libp2p host: %s", err)
	}
	bs := bstore.NewBlockstore(syncds.MutexWrap(ds.NewMapDatastore()))
	bsrv := bserv.New(bs, offline.Exchange(bs))
	ts, err := service.NewService(
		context.Background(),
		h,
		bsrv.Blockstore(),
		dag.NewDAGService(bsrv),
		tstore.NewLogstore(),
		service.Config{
			Debug: true,
		})
	if err != nil {
		return nil, nil, fmt.Errorf("creating service: %s", err)
	}

	return ts, host.InfoFromHost(h), nil
}
