package config

import (
	"flag"
	"os"

	log "github.com/sirupsen/logrus"
)

// By default the release is a custom build. CI takes care of upgrading it with
// go build -v -ldflags="-X 'github.com/bilinearlabs/eth-metrics/config.ReleaseVersion=x.y.z'"
var ReleaseVersion = "custom-build"

// Number of slots in an epoch
// 32 Ethereum mainnet
// 16 Gnosis mainnet
var SlotsInEpoch = uint64(32)

var Network = ""

type Config struct {
	PoolNames             []string
	Network               string
	WithdrawalCredentials []string
	FromAddress           []string
	BeaconRpcEndpoint     string
	PrometheusPort        int
	DatabasePath          string
	Eth1Address           string
	Eth2Address           string
	EpochDebug            string
	Verbosity             string
	StateTimeout          int
}

// custom implementation to allow providing the same flag multiple times
// --flag=value1 --flag=value2
type arrayFlags []string

func (i *arrayFlags) String() string {
	return ""
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func NewCliConfig() (*Config, error) {
	var fromAddress arrayFlags
	var withdrawalCredentials arrayFlags
	var poolNames arrayFlags

	flag.Var(&withdrawalCredentials, "withdrawal-credentials", "Withdrawal credentials used in the deposits. Can be used multiple times")
	flag.Var(&fromAddress, "from-address", "Wallet addresses used to deposit. Can be used multiple times")
	flag.Var(&poolNames, "pool-name", "Pool name to monitor. Can be useed multiple times")

	var network = flag.String("network", "mainnet", "mainnet|gnosis")
	var beaconRpcEndpoint = flag.String("beacon-rpc-endpoint", "localhost:4000", "Address:Port of a eth2 beacon node endpoint")
	var prometheusPort = flag.Int("prometheus-port", 9500, "Prometheus port to listen to")
	var version = flag.Bool("version", false, "Prints the release version and exits")
	//var poolName = flag.String("pool-name", "required", "Name of the pool being monitored. If known, addresses are loaded by default (see known pools)")
	var databasePath = flag.String("database-path", "", "Database path: db.db (optional)")
	var eth1Address = flag.String("eth1address", "", "Ethereum 1 http endpoint. To be used by rocket pool")
	var eth2Address = flag.String("eth2address", "", "Ethereum 2 http endpoint")
	var stateTimeout = flag.Int("state-timeout", 60, "Timeout in seconds for fetching the beacon state")
	var epochDebug = flag.String("epoch-debug", "", "Calculates the stats for a given epoch and exits, useful for debugging")
	var verbosity = flag.String("verbosity", "info", "Logging verbosity (trace, debug, info=default, warn, error, fatal, panic)")
	flag.Parse()

	if *version {
		log.Info("Version: ", ReleaseVersion)
		os.Exit(0)
	}

	if *network != "mainnet" && *network != "gnosis" {
		log.Info("Invalid network: ", *network)
		os.Exit(0)
	}

	// Change the slots in an epoch for gnosis chain
	if *network == "gnosis" {
		SlotsInEpoch = uint64(16)
	}

	// Used for the price
	Network = *network
	/*
		if *poolName == "required" {
			log.Fatal("pool-name flag is required")
		}
	*/
	// If the pool name is known, override from-address
	/*
		preLoadedAddresses := pools.PoolsAddresses[*poolName]
		if len(preLoadedAddresses) != 0 {
			log.Info("The pool-name is known, overriding from-address")
			fromAddress = preLoadedAddresses
		} else {
			if len(fromAddress) == 0 && len(withdrawalCredentials) == 0 {
				log.Fatal("Either withdrawal-credentials or from-address must be populated")
			}
		}*/

	conf := &Config{
		PoolNames:             poolNames,
		Network:               *network,
		BeaconRpcEndpoint:     *beaconRpcEndpoint,
		PrometheusPort:        *prometheusPort,
		WithdrawalCredentials: withdrawalCredentials,
		FromAddress:           fromAddress,
		DatabasePath:          *databasePath,
		Eth1Address:           *eth1Address,
		Eth2Address:           *eth2Address,
		EpochDebug:            *epochDebug,
		Verbosity:             *verbosity,
		StateTimeout:          *stateTimeout,
	}
	logConfig(conf)
	return conf, nil
}

func logConfig(cfg *Config) {
	log.WithFields(log.Fields{
		"PoolNames":             cfg.PoolNames,
		"BeaconRpcEndpoint":     cfg.BeaconRpcEndpoint,
		"WithdrawalCredentials": cfg.WithdrawalCredentials,
		"FromAddress":           cfg.FromAddress,
		"Network":               cfg.Network,
		"PrometheusPort":        cfg.PrometheusPort,
		"DatabasePath":          cfg.DatabasePath,
		"Eth1Address":           cfg.Eth1Address,
		"Eth2Address":           cfg.Eth2Address,
		"EpochDebug":            cfg.EpochDebug,
		"SlotsInEpoch":          SlotsInEpoch,
	}).Info("Cli Config:")
}
