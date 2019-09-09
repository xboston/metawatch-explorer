package main

import (
	jsoniter "github.com/json-iterator/go"
	"github.com/k0kubun/pp"

	"github.com/caarlos0/env/v6"
	"github.com/joho/godotenv"
	"github.com/nsqio/go-nsq"
	"github.com/xboston/metahash-go"
	"go.uber.org/zap"
)

const (
	// регистратор прокси-нод
	addressNodeRegistrator = "0x007a1e062bdb4d57f9a93fd071f81f08ec8e6482c3135c55ea"
)

var (
	rpcClientWallet  metahash.RPCClient
	rpcClientTorrent metahash.RPCClient

	producer *nsq.Producer

	json      = jsoniter.ConfigCompatibleWithStandardLibrary
	logger, _ = zap.NewProduction()

	err error

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
}

func init() {

	logger.Info("init start")

	err := godotenv.Load()
	if err != nil {
		logger.Fatal("Error loading .env file", zap.Error(err))
	}

	if err := env.Parse(&config); err != nil {
		logger.Fatal("Error parsing .env file", zap.Error(err))
	}

	nsqConfig := nsq.NewConfig()
	if producer, err = nsq.NewProducer(config.NsqAddr, nsqConfig); err != nil {
		logger.Fatal("Error start NSQ producer", zap.Error(err))
	}

	rpcClientTorrent = metahash.NewClient(config.AddrTorrent)

	logger.Info("init done")
}

func main() {
	defer logger.Sync()
	logger.Info("start")

	getBlocks()
}

func getBlocks() {
	responseBlocks, err := rpcClientTorrent.Call("get-blocks", &metahash.BlocksArgs{CountBlocks: 2})
	if err == nil {
		var resultBlocks []*metahash.Block
		err = responseBlocks.GetObject(&resultBlocks)
		if err == nil {
			pp.Print("get-blocks", resultBlocks)
		}
	} else {
		pp.Println("err", err.Error())
	}
}
