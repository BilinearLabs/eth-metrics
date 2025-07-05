package db

import (
	"math/big"
	"os"
	"testing"

	"github.com/bilinearlabs/eth-metrics/schemas"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func Test_TODO(t *testing.T) {

	// Create mock test
}

func Test_getDepositsWhereClause(t *testing.T) {
	whereClause := getDepositsWhereClause([]string{"0xkey1", "0xkey2"})
	require.Equal(t,
		"f_eth1_sender = decode('key1', 'hex') or f_eth1_sender = decode('key2', 'hex')",
		whereClause)
}

func Test_GetMissingEpochs(t *testing.T) {
	var dbPath = "test.db"
	db, err := New(dbPath)
	require.NoError(t, err)

	db.CreateTables()

	db.StoreValidatorPerformance(schemas.ValidatorPerformanceMetrics{
		Epoch:         100,
		EarnedBalance: big.NewInt(100),
		LosedBalance:  big.NewInt(100),
	})

	epochs, err := db.GetMissingEpochs(200, 4)
	log.Info("Epochs: ", epochs)
	require.NoError(t, err)
	require.Equal(t, []uint64{197, 198, 199, 200}, epochs)

	db.StoreValidatorPerformance(schemas.ValidatorPerformanceMetrics{
		Epoch:         197,
		EarnedBalance: big.NewInt(100),
		LosedBalance:  big.NewInt(100),
	})

	epochs, err = db.GetMissingEpochs(200, 4)
	log.Info("Epochs: ", epochs)
	require.NoError(t, err)
	require.Equal(t, []uint64{198, 199, 200}, epochs)

	t.Cleanup(func() {
		os.Remove(dbPath)
	})
}
