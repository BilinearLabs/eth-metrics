package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bilinearlabs/eth-metrics/config"
	"github.com/bilinearlabs/eth-metrics/metrics"
	"github.com/bilinearlabs/eth-metrics/price"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
)

var db *sql.DB

func main() {
	config, err := config.NewCliConfig()
	if err != nil {
		log.Fatal(err)
	}

	logLevel, err := log.ParseLevel(config.Verbosity)
	if err != nil {
		log.Fatal(err)
	}
	log.SetLevel(logLevel)

	metrics, err := metrics.NewMetrics(
		context.Background(),
		config)

	if err != nil {
		log.Fatal(err)
	}

	price, err := price.NewPrice(config.DatabasePath, config)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize the database
	db, err = sql.Open("sqlite3", config.DatabasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Set up the Gin server
	r := gin.Default()
	r.Use(cors.Default())

	gin.SetMode(gin.ReleaseMode)

	r.POST("/query", func(c *gin.Context) {
		var query struct {
			SQL string `json:"sql"`
		}

		if err := c.BindJSON(&query); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		if !isSafeQuery(query.SQL) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Unsafe query detected"})
			return
		}

		rows, err := executeQuery(query.SQL)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"data": rows})
	})

	// Run the server in a goroutine
	go func() {
		if err := r.Run(); err != nil {
			log.Fatal("Failed to run server: ", err)
		}
	}()

	go price.Run()
	metrics.Run()

	// Wait for signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	for {
		sig := <-sigCh
		if sig == syscall.SIGINT || sig == syscall.SIGTERM || sig == os.Interrupt || sig == os.Kill {
			break
		}
	}

	log.Info("Stopping eth-metrics")
}

// TODO: Move all api logic to a separate file
func isSafeQuery(query string) bool {
	query = strings.ToLower(query)
	unsafeKeywords := []string{"drop", "delete", "update", "insert", "alter", "create", "replace"}
	for _, keyword := range unsafeKeywords {
		if strings.Contains(query, keyword) {
			return false
		}
	}
	return true
}

func executeQuery(query string) ([]map[string]interface{}, error) {
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	results := make([]map[string]interface{}, 0)
	for rows.Next() {
		columnsData := make([]interface{}, len(columns))
		columnsPointers := make([]interface{}, len(columns))
		for i := range columnsData {
			columnsPointers[i] = &columnsData[i]
		}

		if err := rows.Scan(columnsPointers...); err != nil {
			return nil, err
		}

		rowMap := make(map[string]interface{})
		for i, colName := range columns {
			val := columnsData[i]
			b, ok := val.([]byte)
			if ok {
				rowMap[colName] = string(b)
			} else {
				rowMap[colName] = val
			}
		}
		results = append(results, rowMap)
	}

	return results, nil
}
