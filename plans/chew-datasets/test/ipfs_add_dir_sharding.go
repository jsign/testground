package test

import (
	"context"
	"time"

	config "github.com/ipfs/go-ipfs-config"
	utils "github.com/ipfs/testground/plans/chew-datasets/utils"
	"github.com/ipfs/testground/sdk/iptb"
	"github.com/ipfs/testground/sdk/runtime"
)

// IpfsAddDirSharding IPFS Add Directory Sharding Test
type IpfsAddDirSharding struct{}

func (t *IpfsAddDirSharding) AcceptFiles() bool {
	return false
}

func (t *IpfsAddDirSharding) AcceptDirs() bool {
	return true
}

func (t *IpfsAddDirSharding) AddRepoOptions() iptb.AddRepoOptions {
	return func(cfg *config.Config) error {
		cfg.Experimental.ShardingEnabled = true
		return nil
	}
}

func (t *IpfsAddDirSharding) Execute(ctx context.Context, runenv *runtime.RunEnv, cfg *utils.TestCaseOptions) error {
	if cfg.IpfsInstance != nil {
		runenv.RecordMessage("Running against the Core API")

		err := cfg.ForEachPath(runenv, func(path string, size int64, isDir bool) (string, error) {
			unixfsFile, err := utils.ConvertToUnixfs(path, isDir)
			if err != nil {
				return "", err
			}

			tstarted := time.Now()
			cidFile, err := cfg.IpfsInstance.Unixfs().Add(ctx, unixfsFile)
			if err != nil {
				return "", err
			}
			runenv.RecordMetric(utils.MakeTimeToAddMetric(size, "coreapi"), float64(time.Since(tstarted)/time.Millisecond))

			return cidFile.String(), nil
		})

		if err != nil {
			return err
		}
	}

	if cfg.IpfsDaemon != nil {
		runenv.RecordMessage("Running against the Daemon (IPTB)")

		err := cfg.ForEachPath(runenv, func(path string, size int64, isDir bool) (string, error) {
			tstarted := time.Now()
			cid, err := cfg.IpfsDaemon.AddDir(path)
			runenv.RecordMetric(utils.MakeTimeToAddMetric(size, "daemon"), float64(time.Since(tstarted)/time.Millisecond))
			return cid, err
		})

		if err != nil {
			return err
		}
	}
	return nil
}
