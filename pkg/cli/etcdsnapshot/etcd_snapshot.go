package etcdsnapshot

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/erikdubbelboer/gspt"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/cluster"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/etcd"
	"github.com/rancher/k3s/pkg/server"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/urfave/cli"
)

// commandSetup setups up common things needed
// for each etcd command.
func commandSetup(app *cli.Context, cfg *cmds.Server) (string, error) {
	gspt.SetProcTitle(os.Args[0])

	nodeName := app.String("node-name")
	if nodeName == "" {
		h, err := os.Hostname()
		if err != nil {
			return "", err
		}
		nodeName = h
	}

	os.Setenv("NODE_NAME", nodeName)

	return server.ResolveDataDir(cfg.DataDir)
}

func Run(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return run(app, &cmds.ServerConfig)
}

func run(app *cli.Context, cfg *cmds.Server) error {
	dataDir, err := commandSetup(app, cfg)
	if err != nil {
		return err
	}

	if len(app.Args()) > 0 {
		return cmds.ErrCommandNoArgs
	}

	var serverConfig server.Config
	serverConfig.DisableAgent = true
	serverConfig.ControlConfig.DataDir = dataDir
	serverConfig.ControlConfig.EtcdSnapshotName = cfg.EtcdSnapshotName
	serverConfig.ControlConfig.EtcdSnapshotDir = cfg.EtcdSnapshotDir
	serverConfig.ControlConfig.EtcdSnapshotRetention = 0 // disable retention check
	serverConfig.ControlConfig.EtcdS3 = cfg.EtcdS3
	serverConfig.ControlConfig.EtcdS3Endpoint = cfg.EtcdS3Endpoint
	serverConfig.ControlConfig.EtcdS3EndpointCA = cfg.EtcdS3EndpointCA
	serverConfig.ControlConfig.EtcdS3SkipSSLVerify = cfg.EtcdS3SkipSSLVerify
	serverConfig.ControlConfig.EtcdS3AccessKey = cfg.EtcdS3AccessKey
	serverConfig.ControlConfig.EtcdS3SecretKey = cfg.EtcdS3SecretKey
	serverConfig.ControlConfig.EtcdS3BucketName = cfg.EtcdS3BucketName
	serverConfig.ControlConfig.EtcdS3Region = cfg.EtcdS3Region
	serverConfig.ControlConfig.EtcdS3Folder = cfg.EtcdS3Folder
	serverConfig.ControlConfig.Runtime = &config.ControlRuntime{}
	serverConfig.ControlConfig.Runtime.ETCDServerCA = filepath.Join(dataDir, "tls", "etcd", "server-ca.crt")
	serverConfig.ControlConfig.Runtime.ClientETCDCert = filepath.Join(dataDir, "tls", "etcd", "client.crt")
	serverConfig.ControlConfig.Runtime.ClientETCDKey = filepath.Join(dataDir, "tls", "etcd", "client.key")
	serverConfig.ControlConfig.Runtime.KubeConfigAdmin = filepath.Join(dataDir, "cred", "admin.kubeconfig")

	ctx := signals.SetupSignalHandler(context.Background())
	e := etcd.NewETCD()
	e.SetControlConfig(&serverConfig.ControlConfig)

	initialized, err := e.IsInitialized(ctx, &serverConfig.ControlConfig)
	if err != nil {
		return err
	}
	if !initialized {
		return errors.New("managed etcd database has not been initialized")
	}

	cluster := cluster.New(&serverConfig.ControlConfig)

	if err := cluster.Bootstrap(ctx); err != nil {
		return err
	}

	sc, err := server.NewContext(ctx, serverConfig.ControlConfig.Runtime.KubeConfigAdmin)
	if err != nil {
		return err
	}
	serverConfig.ControlConfig.Runtime.Core = sc.Core

	return cluster.Snapshot(ctx, &serverConfig.ControlConfig)
}

func Delete(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return delete(app, &cmds.ServerConfig)
}

func delete(app *cli.Context, cfg *cmds.Server) error {
	dataDir, err := commandSetup(app, cfg)
	if err != nil {
		return err
	}

	snapshots := app.Args()
	if len(snapshots) == 0 {
		return errors.New("no snapshots given for removal")
	}

	var serverConfig server.Config
	serverConfig.DisableAgent = true
	serverConfig.ControlConfig.DataDir = dataDir
	serverConfig.ControlConfig.EtcdSnapshotName = cfg.EtcdSnapshotName
	serverConfig.ControlConfig.EtcdSnapshotDir = cfg.EtcdSnapshotDir
	serverConfig.ControlConfig.EtcdS3 = cfg.EtcdS3
	serverConfig.ControlConfig.EtcdS3Endpoint = cfg.EtcdS3Endpoint
	serverConfig.ControlConfig.EtcdS3EndpointCA = cfg.EtcdS3EndpointCA
	serverConfig.ControlConfig.EtcdS3SkipSSLVerify = cfg.EtcdS3SkipSSLVerify
	serverConfig.ControlConfig.EtcdS3AccessKey = cfg.EtcdS3AccessKey
	serverConfig.ControlConfig.EtcdS3SecretKey = cfg.EtcdS3SecretKey
	serverConfig.ControlConfig.EtcdS3BucketName = cfg.EtcdS3BucketName
	serverConfig.ControlConfig.EtcdS3Region = cfg.EtcdS3Region
	serverConfig.ControlConfig.EtcdS3Folder = cfg.EtcdS3Folder
	serverConfig.ControlConfig.Runtime = &config.ControlRuntime{}
	serverConfig.ControlConfig.Runtime.KubeConfigAdmin = filepath.Join(dataDir, "cred", "admin.kubeconfig")

	ctx := signals.SetupSignalHandler(context.Background())
	e := etcd.NewETCD()
	e.SetControlConfig(&serverConfig.ControlConfig)

	sc, err := server.NewContext(ctx, serverConfig.ControlConfig.Runtime.KubeConfigAdmin)
	if err != nil {
		return err
	}
	serverConfig.ControlConfig.Runtime.Core = sc.Core

	return e.DeleteSnapshots(ctx, app.Args())
}
