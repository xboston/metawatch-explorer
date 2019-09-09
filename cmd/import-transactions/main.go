package main

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	_ "github.com/kshvakov/clickhouse"

	"github.com/caarlos0/env/v6"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"github.com/panjf2000/ants"
	"github.com/xboston/metahash-go"
	"github.com/xboston/metawatch-explorer/share/metawatch"
)

var (
	rpcClientTorrent  metahash.RPCClient
	lastBlockNumber   int64
	connectClickhouse *sqlx.DB
	err               error

	fullMode = false

	config = AppConfig{}
)

type AppConfig struct {
	Debug bool `env:"DEBUG"`

	AddrClickHouse string `env:"ADDR_CLICKHOUSE"`
	AddrMysql      string `env:"ADDR_MYSQL"`

	NsqAddr        string `env:"NSQ_ADDR" envDefault:"127.0.0.1:4150"`
	NsqConcurrency int    `env:"NSQ_CONCURRENCY" envDefault:"30"`
	NsqMaxInFlight int    `env:"NSQ_MAXINFLIGHT" envDefault:"50"`
	NsqTopic       string `env:"NSQ_TOPIC" envDefault:"metawatch"`
	NsqChannel     string `env:"NSQ_CHANNEL" envDefault:"metawatch"`

	AddrTorrent string `env:"ADDR_TORRENT"`
	AddrWallet  string `env:"ADDR_WALLET"`

	PoolLimit int   `env:"POOL_LIMIT" envDefault:"2"`
	WaitAfter int64 `env:"WAIT_AFTER" envDefault:"1000"`
}

func init() {

	err := godotenv.Load()
	if err != nil {
		log.Fatalln("Error loading .env file", err)
	}

	if err := env.Parse(&config); err != nil {
		log.Fatalln("env", err)
	}

	connectClickhouse, err = sqlx.Open("clickhouse", config.AddrClickHouse)
	if err != nil {
		log.Fatalln("clickhouse", err)
	}

	rpcClientTorrent = metahash.NewClient(config.AddrTorrent)

	responseCountBlocks, err := rpcClientTorrent.Call("get-count-blocks", &metawatch.CountBlocksArgs{})
	if err == nil {
		var resultCountBlocks *metawatch.CountBlocks
		err = responseCountBlocks.GetObject(&resultCountBlocks)
		if err == nil {
			lastBlockNumber = resultCountBlocks.CountBlocks
		}
	} else {
		log.Println("err", err.Error())
	}

	argsFullMode := os.Args[1:]
	if len(argsFullMode) == 1 && argsFullMode[0] == "full" {
		fullMode = true
	}

	lastBlockNumber = 1318540

	log.Println("mode", argsFullMode)
	log.Println("start from", lastBlockNumber)
	time.Sleep(time.Second * 5)
}

func main() {

	var maxBlockNumber int64

	if !fullMode {
		err = connectClickhouse.Select(&maxBlockNumber, `SELECT MAX(block_number) AS block_number FROM Transactions`)
		if err != nil {
			log.Println(err.Error())
		}
	} else {
		maxBlockNumber = 5
	}

	defer ants.Release()

	var wg sync.WaitGroup
	pool, _ := ants.NewPoolWithFunc(config.PoolLimit, func(i interface{}) {
		get(i)
		wg.Done()
	})
	defer pool.Release()

	log.Println("start from block", lastBlockNumber, "to block", maxBlockNumber)

	var step int64
	for i := lastBlockNumber; i > (maxBlockNumber - 5); i-- {
		wg.Add(1)
		_ = pool.Invoke(int64(i))

		if step == config.WaitAfter {
			log.Println("start sleep")
			time.Sleep(time.Second * 25)
			step = 0
		}
		step++
	}
	wg.Wait()
	log.Println("running goroutines: ", pool.Running())
}

func get(i interface{}) {

	blockNumber := i.(int64)

	responseBlockByNumber, err := rpcClientTorrent.Call("get-block-by-number", &metawatch.BlockByNumberArgs{Number: int64(blockNumber), Type: 2})

	if err == nil {
		var resultBlockByNumber *metawatch.Block
		err = responseBlockByNumber.GetObject(&resultBlockByNumber)
		if err == nil {

			log.Println("block", resultBlockByNumber.Number, "from", lastBlockNumber)

			txTransactions, err := connectClickhouse.Begin()
			if err != nil {
				log.Fatal("txTransactions", err)
			}

			sql := "INSERT INTO Transactions " +
				" (date, timestamp, from_address, to_address, value, transaction_hash, type_tx, block_number, signature_hash, fee, fee_real, nonce, status, status_int, is_delegate, delegate_info, delegate, delegate_hash, data_raw, data_string, method) " +
				" VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
			stmtTransactions, err := txTransactions.Prepare(sql)
			if err != nil {
				log.Fatal("stmtTransactions", err)
			}

			for _, tx := range resultBlockByNumber.Txs {

				delegateInfo, _ := json.Marshal(tx.DelegateInfo)
				_, err := stmtTransactions.Exec(
					tx.TimeStamp,
					tx.TimeStamp,
					tx.From,
					tx.To,
					tx.Value,
					tx.Transaction,
					tx.Type,
					tx.BlockNumber,
					tx.Signature,
					tx.Fee,
					tx.RealFee,
					tx.Nonce,
					tx.Status,
					tx.IntStatus,
					tx.IsDelegate,
					string(delegateInfo),
					tx.Delegate,
					tx.DelegateHash,
					tx.Data,
					tx.DataString(),
					tx.Action(),
				)
				if err != nil {
					log.Fatal(err)
				}
			}

			_ = txTransactions.Commit()
			if blockNumber%config.WaitAfter == 1 {
				log.Println("mimi-sleep")
				time.Sleep(time.Second * 5)
			}

		} else {
			log.Fatalln("get-block-error", blockNumber, err.Error())
		}
	} else {
		log.Fatalln("err", blockNumber, err.Error())
	}
}
