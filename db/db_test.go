package db

import (
	"math/big"
	"testing"

	"github.com/bilinearlabs/eth-metrics/schemas"
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
	require.NoError(t, err)
	require.Equal(t, []uint64{197, 198, 199, 200}, epochs)

	db.StoreValidatorPerformance(schemas.ValidatorPerformanceMetrics{
		Epoch:         197,
		EarnedBalance: big.NewInt(100),
		LosedBalance:  big.NewInt(100),
	})

	epochs, err = db.GetMissingEpochs(200, 4)
	require.NoError(t, err)
	require.Equal(t, []uint64{198, 199, 200}, epochs)

	epochs, err = db.GetMissingEpochs(200, 0)
	require.NoError(t, err)
	require.Equal(t, []uint64{}, epochs)
}
