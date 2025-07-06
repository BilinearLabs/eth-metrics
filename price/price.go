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
	config    *config.Config
}

func NewPrice(dbPath string, config *config.Config) (*Price, error) {

	cg := gecko.NewClient(nil)

	var database *db.Database
	var err error
	if dbPath != "" {
		database, err = db.New(dbPath)
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
		config:    config,
	}, nil
}

func (p *Price) GetEthPrice() {
	id := ""
	if p.config.Network == "ethereum" {
		id = "ethereum"
	} else if p.config.Network == "gnosis" {
		id = "gnosis"
	} else {
		log.Fatal("Network not supported: ", p.config.Network)
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
