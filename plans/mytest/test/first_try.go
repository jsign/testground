package test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/mr-tron/base58"
	"github.com/multiformats/go-multiaddr"

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

var empty = ""

var threadSubtree = &sync.Subtree{
	GroupKey: "thread",
	KeyFunc: func(payload interface{}) string {
		return payload.(string)
	},
	PayloadType: reflect.TypeOf(&empty),
}

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
		if err := <-watcher.Barrier(ctx, sync.State("ready"), int64(runenv.RunParams.TestInstanceCount-1)); err != nil {
			return fmt.Errorf("waiting for barrier: %s", err)
		}
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
		addr := addrinfo.Addrs[2].String() + "/p2p/" + api.Host().ID().String() + "/thread/" + info.ID.String()
		bfk := base58.Encode(info.FollowKey.Bytes())
		brk := base58.Encode(info.ReadKey.Bytes())
		inv := fmt.Sprintf("%s:%s:%s", addr, bfk, brk)
		runenv.Message("Created thread %s", inv)
		if _, err = writer.Write(ctx, threadSubtree, &inv); err != nil {
			return fmt.Errorf("writing to threadSubtree: %s", err)
		}
		time.Sleep(time.Second * 30)
	} else {
		ch := make(chan *string)
		if err := watcher.Subscribe(ctx, threadSubtree, ch); err != nil {
			return fmt.Errorf("subscribing to threadSubtree: %s", err)
		}
		if _, err := writer.SignalEntry(ctx, "ready"); err != nil {
			return fmt.Errorf("signalling entry: %s", err)
		}
		inv := <-ch
		runenv.Message("I noticed there's a new thread: %s", *inv)
		splits := strings.Split(*inv, ":")
		fk, err := base58.Decode(splits[1])
		if err != nil {
			return fmt.Errorf("decoding followkey: %s", err)
		}
		rk, err := base58.Decode(splits[2])
		if err != nil {
			return fmt.Errorf("decoding readkey: %s", err)
		}
		rfk, err := symmetric.NewKey(fk)
		if err != nil {
			return fmt.Errorf("creating sk of follow key: %s", err)
		}
		rrk, err := symmetric.NewKey(rk)
		if err != nil {
			return fmt.Errorf("creating sk of read key: %s", err)
		}
		maddr := multiaddr.StringCast(splits[0])
		snfo, err := api.AddThread(ctx, maddr, core.FollowKey(rfk), core.ReadKey(rrk))
		if err != nil {
			return fmt.Errorf("adding thread: %s", err)
		}
		runenv.Message("Added thread correctly: %v", snfo.Logs)
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
