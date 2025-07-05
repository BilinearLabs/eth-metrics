package db

import (
	"math/big"
	"testing"

	"github.com/bilinearlabs/eth-metrics/schemas"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func Test_GetMissingEpochs(t *testing.T) {
	db, err := New(":memory:")
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
}
