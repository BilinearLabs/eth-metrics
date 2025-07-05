package price

import (
	"time"

	"github.com/bilinearlabs/eth-metrics/config"
	"github.com/bilinearlabs/eth-metrics/db"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	gecko "github.com/superoo7/go-gecko/v3"
)

var vc = []string{"usd", "eurr"}

type Price struct {
	database  *db.Database
	coingecko *gecko.Client
}

func NewPrice(postgresEndpoint string) (*Price, error) {

	cg := gecko.NewClient(nil)

	var database *db.Database
	var err error
	if postgresEndpoint != "" {
		database, err = db.New(postgresEndpoint)
		if err != nil {
			return nil, errors.Wrap(err, "could not create postgresql")
		}
		err = database.CreateEthPriceTable()
		if err != nil {
			return nil, errors.Wrap(err, "error creating pool table to store data")
		}
	}

	return &Price{
		database:  database,
		coingecko: cg,
	}, nil
}

func (p *Price) GetEthPrice() {
	id := ""
	if config.Network == "mainnet" {
		id = "ethereum"
	} else if config.Network == "gnosis" {
		id = "gnosis"
	} else {
		log.Fatal("Network not supported: ", config.Network)
	}

	sp, err := p.coingecko.SimplePrice([]string{id}, vc)
	if err != nil {
		log.Error(err)
	}

	eth := (*sp)[id]
	ethPriceUsd := eth["usd"]

	logPrice(ethPriceUsd)

	if p.database != nil {
		err := p.database.StoreEthPrice(ethPriceUsd)
		if err != nil {
			log.Error(err)
		}
	}
}

func (p *Price) Run() {
	todoSetAsFlag := 30 * time.Minute
	ticker := time.NewTicker(todoSetAsFlag)
	for ; true; <-ticker.C {
		p.GetEthPrice()
	}
}

func logPrice(price float32) {
	log.Info("Ethereum price in USD: ", price)
}
