package metrics

import (
	"context"
	"strconv"
	"strings"
	"time"

	apiOther "github.com/attestantio/go-eth2-client/api"
	api "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/attestantio/go-eth2-client/http"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/bilinearlabs/eth-metrics/config"
	"github.com/bilinearlabs/eth-metrics/db"

	"github.com/bilinearlabs/eth-metrics/schemas"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	log "github.com/sirupsen/logrus"
)

type ProposalDuties struct {
	httpClient    *http.Service
	database      *db.Database
	eth1Endpoint  string
	eth2Endpoint  string
	fromAddresses []string
	poolNames     []string
}

func NewProposalDuties(
	eth1Endpoint string,
	eth2Endpoint string,
	fromAddresses []string,
	database *db.Database,
	poolNames []string) (*ProposalDuties, error) {

	client, err := http.New(context.Background(),
		http.WithTimeout(60*time.Second),
		http.WithAddress(eth2Endpoint),
		http.WithLogLevel(zerolog.WarnLevel),
	)
	if err != nil {
		return nil, err
	}

	httpClient := client.(*http.Service)

	return &ProposalDuties{
		httpClient:    httpClient,
		eth2Endpoint:  eth2Endpoint,
		fromAddresses: fromAddresses,
		poolNames:     poolNames,
		eth1Endpoint:  eth1Endpoint,
		database:      database,
	}, nil
}

func (p *ProposalDuties) RunProposalMetrics(
	activeKeys []uint64,
	poolName string,
	metrics *schemas.ProposalDutiesMetrics) error {

	poolProposals := getPoolProposalDuties(
		metrics,
		poolName,
		activeKeys)

	logProposalDuties(poolProposals, poolName)

	if p.database != nil {
		err := p.database.StoreProposalDuties(metrics.Epoch, poolName, uint64(len(poolProposals.Scheduled)), uint64(len(poolProposals.Proposed)))
		if err != nil {
			return errors.Wrap(err, "could not store proposal duties")
		}
	}
	return nil

}

func (p *ProposalDuties) GetProposalDuties(epoch uint64) ([]*api.ProposerDuty, error) {
	log.Info("Fetching proposal duties for epoch: ", epoch)

	// Empty indexes to force fetching all duties
	indexes := make([]phase0.ValidatorIndex, 0)

	opts := apiOther.ProposerDutiesOpts{
		Indices: indexes,
		Epoch:   phase0.Epoch(epoch),
	}

	duties, err := p.httpClient.ProposerDuties(
		context.Background(),
		&opts)

	if err != nil {
		return make([]*api.ProposerDuty, 0), err
	}

	return duties.Data, nil
}

func (p *ProposalDuties) GetProposedBlocks(epoch uint64) ([]*api.BeaconBlockHeader, error) {
	log.Info("Fetching proposed blocks for epoch: ", epoch)

	epochBlockHeaders := make([]*api.BeaconBlockHeader, 0)
	slotsInEpoch := uint64(config.SlotsInEpoch)

	for i := uint64(0); i < slotsInEpoch; i++ {
		slot := epoch*slotsInEpoch + uint64(i)
		slotStr := strconv.FormatUint(slot, 10)
		log.Debug("Fetching block for slot:" + slotStr)

		opts := apiOther.BeaconBlockHeaderOpts{
			Block: slotStr,
		}

		blockHeader, err := p.httpClient.BeaconBlockHeader(context.Background(), &opts)
		if err != nil {
			// This error is expected in skipped or orphaned blocks
			if !strings.Contains(err.Error(), "NOT_FOUND") {
				return epochBlockHeaders, errors.Wrap(err, "error getting beacon block header")
			}
			log.Warn("Block at slot " + slotStr + " was not found")
			continue
		}
		// Some sleep to avoid rate limiting
		time.Sleep(1 * time.Second)
		epochBlockHeaders = append(epochBlockHeaders, blockHeader.Data)
	}

	return epochBlockHeaders, nil
}

func (p *ProposalDuties) GetProposalMetrics(
	proposalDuties []*api.ProposerDuty,
	proposedBlocks []*api.BeaconBlockHeader) (schemas.ProposalDutiesMetrics, error) {

	proposalMetrics := schemas.ProposalDutiesMetrics{
		Epoch:     0,
		Scheduled: make([]schemas.Duty, 0),
		Proposed:  make([]schemas.Duty, 0),
		Missed:    make([]schemas.Duty, 0),
	}

	if len(proposalDuties) != len(proposedBlocks) {
		log.Warn("Duties and blocks have different sizes, ok if n blocks were missed/orphaned")
		//return proposalMetrics, errors.New("duties and blocks have different sizes")
	}

	if proposalDuties == nil || proposedBlocks == nil {
		return proposalMetrics, errors.New("duties and blocks can't be nil")
	}

	/* proposedBlocks[0].Header.Message.Slot is nil if the block was missed
	if proposalDuties[0].Slot != proposedBlocks[0].Header.Message.Slot {
		return proposalMetrics, errors.New("duties and proposals contains different slots")
	}*/

	proposalMetrics.Epoch = uint64(proposalDuties[0].Slot) / config.SlotsInEpoch

	for _, duty := range proposalDuties {
		proposalMetrics.Scheduled = append(
			proposalMetrics.Scheduled,
			schemas.Duty{
				ValIndex: uint64(duty.ValidatorIndex),
				Slot:     uint64(duty.Slot),
				Graffiti: "NA",
			})
	}

	for _, block := range proposedBlocks {
		// If block was missed its nil
		if block == nil {
			continue
		}
		proposalMetrics.Proposed = append(
			proposalMetrics.Proposed,
			schemas.Duty{
				ValIndex: uint64(block.Header.Message.ProposerIndex),
				Slot:     uint64(block.Header.Message.Slot),
				Graffiti: "TODO",
			})

	}

	return proposalMetrics, nil
}

func getMissedDuties(scheduled []schemas.Duty, proposed []schemas.Duty) []schemas.Duty {
	missed := make([]schemas.Duty, 0)

	for _, s := range scheduled {
		found := false
		for _, p := range proposed {
			if s.Slot == p.Slot && s.ValIndex == p.ValIndex {
				found = true
				break
			}
		}
		if found == false {
			missed = append(missed, s)
		}
	}

	return missed
}

// TODO: This is very inefficient
func getPoolProposalDuties(
	metrics *schemas.ProposalDutiesMetrics,
	poolName string,
	activeValidatorIndexes []uint64) *schemas.ProposalDutiesMetrics {

	poolDuties := schemas.ProposalDutiesMetrics{
		Epoch:     metrics.Epoch,
		Scheduled: make([]schemas.Duty, 0),
		Proposed:  make([]schemas.Duty, 0),
		Missed:    make([]schemas.Duty, 0),
	}

	// Check if this pool has any assigned proposal duties
	for i := range metrics.Scheduled {
		if IsValidatorIn(metrics.Scheduled[i].ValIndex, activeValidatorIndexes) {
			poolDuties.Scheduled = append(poolDuties.Scheduled, metrics.Scheduled[i])
		}
	}

	// Check the proposed blocks from the pool
	for i := range metrics.Proposed {
		if IsValidatorIn(metrics.Proposed[i].ValIndex, activeValidatorIndexes) {
			poolDuties.Proposed = append(poolDuties.Proposed, metrics.Proposed[i])
		}
	}

	poolDuties.Missed = getMissedDuties(poolDuties.Scheduled, poolDuties.Proposed)

	return &poolDuties
}

func logProposalDuties(
	poolDuties *schemas.ProposalDutiesMetrics,
	poolName string) {

	for _, d := range poolDuties.Scheduled {
		log.WithFields(log.Fields{
			"PoolName":       poolName,
			"ValIndex":       d.ValIndex,
			"Slot":           d.Slot,
			"Epoch":          poolDuties.Epoch,
			"TotalScheduled": len(poolDuties.Scheduled),
		}).Info("Scheduled Duty")
	}

	for _, d := range poolDuties.Proposed {
		log.WithFields(log.Fields{
			"PoolName":      poolName,
			"ValIndex":      d.ValIndex,
			"Slot":          d.Slot,
			"Epoch":         poolDuties.Epoch,
			"Graffiti":      d.Graffiti,
			"TotalProposed": len(poolDuties.Proposed),
		}).Info("Proposed Duty")
	}

	for _, d := range poolDuties.Missed {
		log.WithFields(log.Fields{
			"PoolName":    poolName,
			"ValIndex":    d.ValIndex,
			"Slot":        d.Slot,
			"Epoch":       poolDuties.Epoch,
			"TotalMissed": len(poolDuties.Missed),
		}).Info("Missed Duty")
	}
}
