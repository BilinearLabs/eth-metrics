package metrics

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/attestantio/go-eth2-client/api"
	"github.com/attestantio/go-eth2-client/http"
	"github.com/attestantio/go-eth2-client/spec"
	"github.com/rs/zerolog"

	"github.com/bilinearlabs/eth-metrics/config"
	"github.com/bilinearlabs/eth-metrics/db"
	"github.com/bilinearlabs/eth-metrics/pools"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type Metrics struct {
	genesisSeconds uint64
	slotsInEpoch   uint64
	secondsPerSlot uint64
	depositedKeys  [][]byte
	validatingKeys [][]byte
	withCredList   []string
	fromAddrList   []string
	eth1Address    string
	eth2Address    string
	db             *db.Database

	httpClient *http.Service

	beaconState    *BeaconState
	proposalDuties *ProposalDuties

	// Slot and epoch and its raw data
	// TODO: Remove, each metric task has its pace
	Epoch uint64
	Slot  uint64

	PoolNames  []string
	epochDebug string
	config     *config.Config // TODO: Remove repeated parameters
}

func NewMetrics(
	ctx context.Context,
	config *config.Config) (*Metrics, error) {

	var pg *db.Database
	var err error
	if config.DatabasePath != "" {
		pg, err = db.New(config.DatabasePath)
		if err != nil {
			return nil, errors.Wrap(err, "could not create postgresql")
		}
		err = pg.CreateTables()
		if err != nil {
			return nil, errors.Wrap(err, "error creating pool table to store data")
		}
	}

	for _, poolName := range config.PoolNames {
		if strings.HasSuffix(poolName, ".txt") {
			pubKeysDeposited, err := pools.ReadCustomValidatorsFile(poolName)
			if err != nil {
				log.Fatal(err)
			}
			log.Info("File: ", poolName, " contains ", len(pubKeysDeposited), " keys")

		}
	}

	client, err := http.New(context.Background(),
		http.WithTimeout(60*time.Second),
		http.WithAddress(config.Eth2Address),
		http.WithLogLevel(zerolog.WarnLevel),
	)
	if err != nil {
		return nil, err
	}

	httpClient := client.(*http.Service)

	genesis, err := httpClient.Genesis(context.Background(), &api.GenesisOpts{})
	if err != nil {
		return nil, errors.Wrap(err, "error getting genesis info")
	}

	spec, err := httpClient.Spec(context.Background(), &api.SpecOpts{})
	if err != nil {
		return nil, errors.Wrap(err, "error getting spec info")
	}

	slotsPerEpochInterface, found := spec.Data["SLOTS_PER_EPOCH"]
	if !found {
		return nil, errors.New("SLOTS_PER_EPOCH not found in spec")
	}

	secondsPerSlotInterface, found := spec.Data["SECONDS_PER_SLOT"]
	if !found {
		return nil, errors.New("SECONDS_PER_SLOT not found in spec")
	}

	slotsPerEpoch := slotsPerEpochInterface.(uint64)

	secondsPerSlot := uint64(secondsPerSlotInterface.(time.Duration).Seconds())

	log.Info("Genesis time: ", genesis.Data.GenesisTime.Unix())
	log.Info("Slots per epoch: ", slotsPerEpoch)
	log.Info("Seconds per slot: ", secondsPerSlot)

	return &Metrics{
		withCredList:   config.WithdrawalCredentials,
		fromAddrList:   config.FromAddress,
		genesisSeconds: uint64(genesis.Data.GenesisTime.Unix()),
		slotsInEpoch:   slotsPerEpoch,
		secondsPerSlot: secondsPerSlot,
		eth1Address:    config.Eth1Address,
		eth2Address:    config.Eth2Address,
		db:             pg,
		PoolNames:      config.PoolNames,
		httpClient:     httpClient,
		epochDebug:     config.EpochDebug,
		config:         config,
	}, nil
}

func (a *Metrics) Run() {
	bc, err := NewBeaconState(
		a.eth1Address,
		a.eth2Address,
		a.db,
		a.fromAddrList,
		a.PoolNames,
		a.config.StateTimeout,
	)
	if err != nil {
		log.Fatal(err)
		// TODO: Add return here.
	}
	a.beaconState = bc

	pd, err := NewProposalDuties(
		a.eth1Address,
		a.eth2Address,
		a.fromAddrList,
		a.db,
		a.PoolNames)

	if err != nil {
		log.Fatal(err)
	}
	a.proposalDuties = pd

	for _, poolName := range a.PoolNames {
		//if poolName == "rocketpool" {
		//	go pools.RocketPoolFetcher(a.eth1Address)
		//	break
		//}

		// Check that the validator keys are correct
		_, _, err := a.GetValidatorKeys(poolName)
		if err != nil {
			log.Fatal(err)
		}

	}
	go a.Loop()
}

func (a *Metrics) Loop() {
	// TODO: Move this somewhere. Backfill in time. Eg 1 month.
	var epochsToBackFill uint64 = 40

	var prevEpoch uint64 = uint64(0)
	var prevBeaconState *spec.VersionedBeaconState = nil
	// TODO: Refactor and hoist some stuff out to a function
	for {
		// Before doing anything, check if we are in the next epoch
		opts := api.NodeSyncingOpts{
			Common: api.CommonOpts{
				Timeout: 5 * time.Second,
			},
		}
		headSlot, err := a.httpClient.NodeSyncing(context.Background(), &opts)
		if err != nil {
			log.Error("Could not get node sync status:", err)
			time.Sleep(5 * time.Second)
			continue
		}

		if headSlot.Data.IsSyncing {
			log.Error("Node is not in sync")
			time.Sleep(5 * time.Second)
			continue
		}

		// Leave some maring of 2 epochs
		currentEpoch := uint64(headSlot.Data.HeadSlot)/uint64(config.SlotsInEpoch) - 2

		// If a debug epoch is set, overwrite the slot. Will compute just metrics for that epoch
		if a.epochDebug != "" {
			epochDebugUint64, err := strconv.ParseUint(a.epochDebug, 10, 64)
			if err != nil {
				log.Fatal(err)
			}
			log.Warn("Debugging mode, calculating metrics for epoch: ", a.epochDebug)
			currentEpoch = epochDebugUint64
		}

		if prevEpoch >= currentEpoch {
			// do nothing
			time.Sleep(5 * time.Second)
			continue
		}

		missingEpochs, err := a.db.GetMissingEpochs(currentEpoch, epochsToBackFill)
		if err != nil {
			log.Error(err)
			time.Sleep(5 * time.Second)
			continue
		}

		if len(missingEpochs) > 0 {
			log.Info("Backfilling epochs: ", missingEpochs)
		}

		// Do backfilling.
		for _, epoch := range missingEpochs {
			if prevBeaconState != nil {
				prevSlot, err := prevBeaconState.Slot()
				prevEpoch = uint64(prevSlot) % config.SlotsInEpoch
				if err != nil {
					// TODO: Handle this gracefully
					log.Fatal(err, "error getting slot from previous beacon state")
				}
				if (prevEpoch + 1) != epoch {
					prevBeaconState = nil
				}
			}
			currentBeaconState, err := a.ProcessEpoch(epoch, prevBeaconState)
			if err != nil {
				log.Error(err)
				time.Sleep(5 * time.Second)
				continue
			}
			prevBeaconState = currentBeaconState
		}

		currentBeaconState, err := a.ProcessEpoch(currentEpoch, prevBeaconState)
		if err != nil {
			log.Error(err)
			time.Sleep(5 * time.Second)
			continue
		}

		prevBeaconState = currentBeaconState
		prevEpoch = currentEpoch

		if a.epochDebug != "" {
			log.Warn("Running in debug mode, exiting ok.")
			os.Exit(0)
		}
	}
}

func (a *Metrics) ProcessEpoch(
	currentEpoch uint64,
	prevBeaconState *spec.VersionedBeaconState) (*spec.VersionedBeaconState, error) {
	// Fetch proposal duties, meaning who shall propose each block within this epoch
	duties, err := a.proposalDuties.GetProposalDuties(currentEpoch)
	if err != nil {
		return nil, errors.Wrap(err, "error getting proposal duties")
	}

	// Fetch who actually proposed the blocks in this epoch
	proposed, err := a.proposalDuties.GetProposedBlocks(currentEpoch)
	if err != nil {
		return nil, errors.Wrap(err, "error getting proposed blocks")
	}

	// Summarize duties + proposed in a struct
	proposalMetrics, err := a.proposalDuties.GetProposalMetrics(duties, proposed)
	if err != nil {
		return nil, errors.Wrap(err, "error getting proposal metrics")
	}

	currentBeaconState, err := a.beaconState.GetBeaconState(currentEpoch)
	if err != nil {
		return nil, errors.Wrap(err, "error fetching beacon state")
	}

	// if no prev beacon state is known, fetch it
	if prevBeaconState == nil {
		prevBeaconState, err = a.beaconState.GetBeaconState(currentEpoch - 1)
		if err != nil {
			return nil, errors.Wrap(err, "error fetching previous beacon state")
		}
	}

	// Map to quickly convert public keys to index
	valKeyToIndex := PopulateKeysToIndexesMap(currentBeaconState)

	// Iterate all pools and calculate metrics using the fetched data
	for _, poolName := range a.PoolNames {
		poolName, pubKeys, err := a.GetValidatorKeys(poolName)
		if err != nil {
			return nil, errors.Wrap(err, "error getting validator keys")
		}

		validatorIndexes := GetIndexesFromKeys(pubKeys, valKeyToIndex)

		// TODO Rename this
		a.beaconState.Run(pubKeys, poolName, currentBeaconState, prevBeaconState, valKeyToIndex)

		err = a.proposalDuties.RunProposalMetrics(validatorIndexes, poolName, &proposalMetrics)
		if err != nil {
			return nil, errors.Wrap(err, "error running proposal metrics")
		}
	}

	return currentBeaconState, nil
}

// Get the validator keys from different sources:
// - pool.txt: Opens the file and read the keys from it
// - rocketpool: Special case, see pools
// - poolname: Gets the keys from the address used for the deposit
func (a *Metrics) GetValidatorKeys(poolName string) (string, [][]byte, error) {
	var pubKeysDeposited [][]byte
	var err error
	if strings.HasSuffix(poolName, ".txt") {
		// Vanila file, one key per line
		pubKeysDeposited, err = pools.ReadCustomValidatorsFile(poolName)
		if err != nil {
			log.Fatal(err)
		}
		// trim the file path and extension
		poolName = filepath.Base(poolName)
		poolName = strings.TrimSuffix(poolName, filepath.Ext(poolName))
	} else if strings.HasSuffix(poolName, ".csv") {
		// ethsta.com format
		pubKeysDeposited, err = pools.ReadEthstaValidatorsFile(poolName)
		if err != nil {
			log.Fatal(err)
		}
		// trim the file path and extension
		poolName = filepath.Base(poolName)
		poolName = strings.TrimSuffix(poolName, filepath.Ext(poolName))

	}
	return poolName, pubKeysDeposited, nil
}
