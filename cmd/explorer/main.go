package main

import (
	"encoding/hex"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	jsoniter "github.com/json-iterator/go"
	_ "github.com/kshvakov/clickhouse"

	"github.com/ararog/timeago"
	"github.com/caarlos0/env/v6"
	"github.com/coocood/freecache"
	"github.com/foolin/goview"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/niubaoshu/gotiny"
	"github.com/nsqio/go-nsq"
	"github.com/vmihailenco/msgpack"
	"github.com/xboston/metahash-go"
	"github.com/xboston/metawatch-explorer/share/metawatch"
	"golang.org/x/text/message"
)

const (
	// регистратор прокси-нод
	addressNodeRegistrator = "0x007a1e062bdb4d57f9a93fd071f81f08ec8e6482c3135c55ea"
	// лимит постранички транзакций
	txLimit = 50
	// лимит постранички
	pageLimit = 50
)

var (
	rpcClientWallet  metahash.RPCClient
	rpcClientTorrent metahash.RPCClient

	connectClickhouse *sqlx.DB
	connectMysql      *sqlx.DB
	producer          *nsq.Producer

	json = jsoniter.ConfigCompatibleWithStandardLibrary

	err error

	balanceFormatter = message.NewPrinter(message.MatchLanguage("en"))

	cacheSize = 100 * 1024 * 1024
	cache     = freecache.NewCache(cacheSize)

	// динамические имена нод
	nodeNames = map[string]string{}

	currentPrice     float64
	currentPriceDIFF float64

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

	connectMysql, err = sqlx.Connect("mysql", config.AddrMysql)
	if err != nil {
		log.Fatalln("mysql", err)
	}

	nsqConfig := nsq.NewConfig()
	if producer, err = nsq.NewProducer(config.NsqAddr, nsqConfig); err != nil {
		log.Fatalln("nsq", err)
	}

	rpcClientWallet = metahash.NewClient(config.AddrWallet)
	rpcClientTorrent = metahash.NewClient(config.AddrTorrent)

	// обновление стоимости монеты по таймеру
	go updatePrice()
	go func() {
		for range time.Tick(time.Minute * 1) {
			updatePrice()
		}
	}()

	// обновления имён нод по таймеру
	go updateNodenames()
	go func() {
		for range time.Tick(time.Minute * 5) {
			updateNodenames()
		}
	}()

	// обновление системных статусов по таймеру
	go getUpdateSystemStatus()
	go func() {
		for range time.Tick(time.Minute * 2) {
			getUpdateSystemStatus()
		}
	}()
}

func main() {
	e := echo.New()
	e.DisableHTTP2 = true
	e.HideBanner = true

	if !config.Debug {
		e.Use(middleware.Logger())
		e.Use(middleware.Recover())
	} else {
		e.Debug = true
		e.Static("/", "public")
	}

	e.Renderer = NewViewEngine(goview.Config{
		Root:      "views",
		Extension: ".html",
		Master:    "layouts/master",
		Partials: []string{
			"partials/addressheader",
			"partials/walletheader",
		},
		Funcs: template.FuncMap{
			"addressname": func(hash string) string {

				if len(hash) == 52 || hash == "InitialWalletTransaction" {
					go updateAddress(hash)

					if n, ok := nodeNames[hash]; ok {
						return n
					}
				}
				return hash
			},
			"addressname_big": func(hash string) string {

				if len(hash) == 52 || hash == "InitialWalletTransaction" {
					go updateAddress(hash)

					if n, ok := nodeNames[hash]; ok {
						return hash + " / " + n
					}

				}
				return hash
			},
			"hashtrim": func(hash string) string {
				return hashtrim(hash)
			},
			"date": func(timeStamp int64) string {
				return time.Unix(timeStamp, 0).UTC().String()
			},
			"balance": func(balance int64) string {
				if balance < 1000000 {
					return strconv.FormatFloat(float64(balance)/float64(1000000), 'f', -1, 64)
				}

				balanceResult := balanceFormatter.Sprintf("%f", float64(balance)/float64(1000000))
				return strings.TrimRight(strings.TrimRight(balanceResult, "0"), ".")
			},
			"bignumber": func(balance int64) string {
				return balanceFormatter.Sprintf("%d", balance)
			},
			"bytes": func(size int64) template.HTML {
				got := byteCountDecimal(size)
				return template.HTML("<span class=\"tooltip is-tooltip-right\" data-tooltip=\"" + strconv.FormatInt(size, 10) + " bytes\">" + got + "</span>")
			},
			"timeago": func(timeStamp int64) template.HTML {
				start := time.Now()
				end := time.Unix(timeStamp, 0).UTC()
				got, _ := timeago.TimeAgoWithTime(start, end)
				return template.HTML("<span class=\"tooltip is-tooltip-right\" data-tooltip=\"" + end.String() + "\">" + got + "</span>")
			},
			"timeago_raw": func(timeStamp int64) template.HTML {
				return template.HTML(time.Unix(timeStamp, 0).UTC().Format("2006-01-02 15:04"))
			},
			"timeago_max": func(timeStamp int64) template.HTML {
				start := time.Now()
				end := time.Unix(timeStamp, 0).UTC()
				got, _ := timeago.TimeAgoWithTime(start, end)
				return template.HTML(got + " (" + end.String() + ")")
			},
			"timeago_time": func(timeStamp time.Time) template.HTML {
				start := time.Now()
				got, _ := timeago.TimeAgoWithTime(start, timeStamp)
				return template.HTML("<span class=\"tooltip is-tooltip-right\" data-tooltip=\"" + timeStamp.String() + "\">" + got + "</span>")
			},
			"js": func(v interface{}) template.JS {
				a, _ := json.Marshal(v)
				return template.JS(a)
			},
			"pp": func(v interface{}) string {
				a, _ := prettyPrintJSON(v)
				return string(a)
			},
			"inc": func(v int) int {
				return v + 1
			},
			"current_price": func(amount int) float64 {
				return currentPrice
			},
			"price": func(amount int64) string {
				balance := math.Round((float64(amount)/1e6*currentPrice)*100) / 100
				balanceResult := balanceFormatter.Sprintf("%f", float64(balance))
				return strings.TrimRight(strings.TrimRight(balanceResult, "0"), ".")
			},
		},
		DisableCache: false,
	})

	echo.NotFoundHandler = func(c echo.Context) error {
		q := c.QueryParam("q")
		return c.Render(http.StatusNotFound, "404", echo.Map{
			"title": "404 - metawat.ch",
			"q":     q,
		})
	}

	e.GET("/", func(c echo.Context) error {

		key := []byte("index_page")
		valuesRaw, _ := cache.Get(key)

		if valuesRaw == nil {
			nodes := []IndexNodePoint{}
			err = connectMysql.Select(&nodes, `SELECT nodes.address, node_type, name, mg_trust, mg_geo, mg_roi, addresses.delegated_amount AS delegated_amount
				FROM nodes
				INNER JOIN addresses ON (nodes.address=addresses.address AND addresses.delegated_amount>= 100000*1e6 AND addresses.delegated_amount <= 10000000*1e6)
				WHERE mg_status=1 AND mg_trust<>'0.001' 
				ORDER BY ROUND(delegated_amount/1e11,0) ASC, mg_trust DESC, mg_roi DESC
				LIMIT 500`) // AND mg_roi<>'0.000000'
			if err != nil {
				log.Fatal(err.Error())
			}

			values := echo.Map{
				"title":           "MetaHash / TraceChain Explorer",
				"hideSmallSearch": true,
				"nodes":           nodes,
			}

			values["statusData"] = getUpdateSystemStatus()

			valuesMarshal := gotiny.Marshal(&values)
			_ = cache.Set(key, valuesMarshal, 60)
			return c.Render(http.StatusOK, "index", values)
		}

		valuesCache := echo.Map{}
		gotiny.Unmarshal(valuesRaw, &valuesCache)

		valuesCache["statusData"] = getUpdateSystemStatus()

		return c.Render(http.StatusOK, "index", valuesCache)
	})

	{ // транзакции
		groupTxs := e.Group("/txs")

		// все транзакции с постраничкой
		groupTxs.GET("", func(c echo.Context) error {

			responseLastTxs, err := rpcClientTorrent.Call("get-last-txs", &metawatch.LastTxsArgs{})
			if err != nil {
				return c.JSON(http.StatusBadRequest, err.Error())
			}

			var resultLastTxs []*metawatch.TransactionInfo
			err = responseLastTxs.GetObject(&resultLastTxs)

			if err != nil {
				return c.JSON(http.StatusBadRequest, err.Error())
			}

			return c.Render(http.StatusOK, "transactions", echo.Map{
				"title":         "Last transactions",
				"resultLastTxs": resultLastTxs,
			})
		})

		// TOP транзакций
		groupTxs.GET("/top", func(c echo.Context) error {

			topLimit := 10

			type valuePoint struct {
				TimeStamp   time.Time `json:"timestamp" db:"timestamp"`
				Transaction string    `json:"transaction" db:"transaction"`
				From        string    `json:"from" db:"fromA"`
				To          string    `json:"to" db:"toA"`
				BlockNumber int64     `json:"block_number" db:"blockNumber"`
				Value       int64     `json:"value" db:"value"`
			}

			sqlValue := `SELECT
					timestamp, transaction, fromA, toA, blockNumber, value
				FROM Transactions
				WHERE date >= today()-1 AND timestamp >= (NOW()-INTERVAL 24 HOUR) AND fromA<>'InitialWalletTransaction'
				ORDER BY value DESC
				LIMIT ?`

			valuePoints := []valuePoint{}
			err = connectClickhouse.Select(&valuePoints, sqlValue, topLimit)
			if err != nil {
				log.Println(err.Error())
			}

			type amountFromPoint struct {
				From              string `json:"from" db:"fromA"`
				Value             int64  `json:"amount" db:"amount"`
				TransactionsCount int64  `json:"tx_count" db:"txCount"`
			}

			sqlAmountFrom := `SELECT
				fromA, sum(value) amount, count() txCount
			FROM Transactions
			WHERE date >= today()-1 AND timestamp >= (NOW()-INTERVAL 24 HOUR) AND fromA<>'InitialWalletTransaction'
			GROUP BY fromA
			ORDER BY amount DESC
			LIMIT ?`

			amountFromPoints := []amountFromPoint{}
			err = connectClickhouse.Select(&amountFromPoints, sqlAmountFrom, topLimit)
			if err != nil {
				log.Println(err.Error())
			}

			type amountToPoint struct {
				To                string `json:"to" db:"toA"`
				Value             int64  `json:"amount" db:"amount"`
				TransactionsCount int64  `json:"tx_count" db:"txCount"`
			}

			sqlAmountTo := `SELECT
				toA, sum(value) amount, count() txCount
			FROM Transactions
			WHERE date >= today()-1 AND timestamp >= (NOW()-INTERVAL 24 HOUR) AND fromA<>'InitialWalletTransaction'
			GROUP BY toA
			ORDER BY amount DESC
			LIMIT ?`

			amountToPoints := []amountToPoint{}
			err = connectClickhouse.Select(&amountToPoints, sqlAmountTo, topLimit)
			if err != nil {
				log.Println(err.Error())
			}
			type byValuePoint struct {
				Value             int64 `json:"value" db:"value"`
				TransactionsCount int64 `json:"tx_count" db:"txCount"`
			}

			sqlByValue := `SELECT
				value, count() txCount
			FROM Transactions
			WHERE date >= today()-1 AND timestamp >= (NOW()-INTERVAL 24 HOUR) AND fromA<>'InitialWalletTransaction' AND value>0
			GROUP BY value
			ORDER BY txCount DESC
			LIMIT ?`

			byValuePoints := []byValuePoint{}
			err = connectClickhouse.Select(&byValuePoints, sqlByValue, topLimit*2)
			if err != nil {
				log.Println(err.Error())
			}

			return c.Render(http.StatusOK, "transactions_top", echo.Map{
				"title":            "Top transactions",
				"topLimit":         topLimit,
				"valuePoints":      &valuePoints,
				"amountFromPoints": &amountFromPoints,
				"amountToPoints":   &amountToPoints,
				"byValuePoints":    &byValuePoints,
			})
		})

		// транзакция по хешу
		groupTxs.GET("/:hash", func(c echo.Context) error {
			hash := c.Param("hash")
			responseTransaction, err := rpcClientTorrent.Call("get-tx", &metawatch.TransactionArgs{Hash: hash})

			if responseTransaction.Error != nil {
				return c.Render(http.StatusOK, "404message", echo.Map{
					"title":   "Transaction not found",
					"message": "Transaction not found", // responseTransaction.Error.Message,
				})
			}

			if err != nil {
				return c.JSON(http.StatusBadRequest, err.Error())
			}

			var resultTransaction *metawatch.Transaction
			err = responseTransaction.GetObject(&resultTransaction)

			if err != nil {
				return c.JSON(http.StatusBadRequest, err.Error())
			}

			return c.Render(http.StatusOK, "transaction", echo.Map{
				"title":       "Transaction " + hash,
				"Transaction": &resultTransaction.Transaction,
			})
		})
	}

	{ // адреса
		groupAddress := e.Group("/address")

		// тут middleware с получением параметров адреса из базы и его проверка
		groupAddress.Use(addAddressInfo)

		groupAddress.GET("", func(c echo.Context) error {

			sort := c.QueryParam("sort")
			limit := strings.TrimSpace(c.QueryParam("limit"))
			intLimit := 1000
			if limit != "" && limit != "0" {
				intLimit, _ = strconv.Atoi(limit)
			}

			if intLimit > 5000 {
				intLimit = 1000
			}

			var (
				listsAddress = []Address{}
				sql          string
			)

			switch sort {
			case "amount":
				sql = "select * from addresses ORDER BY amount DESC LIMIT ?"
			case "frozen":
				sql = "select * from addresses ORDER BY frozen DESC LIMIT ?"
			case "forging":
				sql = "select * from addresses ORDER BY forging DESC LIMIT ?"
			case "txs":
				sql = "select * from addresses ORDER BY tx_count DESC LIMIT ?"
			case "date_asc":
				sql = "select * from addresses ORDER BY updated_at ASC LIMIT ?"
			case "date_desc":
				sql = "select * from addresses ORDER BY updated_at DESC LIMIT ?"
			default:
				sql = "select * from addresses ORDER BY amount DESC LIMIT ?"
			}

			err = connectMysql.Select(&listsAddress, sql, intLimit)
			if err != nil {
				log.Println(err.Error())
			}

			return c.Render(http.StatusOK, "address/index", echo.Map{
				"title":        "MHC addresses",
				"listsAddress": &listsAddress,
				"limit":        &intLimit,
			})
		})

		groupAddress.GET("/:address", func(c echo.Context) error {
			address := c.Param("address")

			sort := "txs_count"
			if c.QueryParam("sort") != "" {
				sort = "txs_amount"
			}

			if len(address) != 52 {

				name := ""
				if address == "InitialWalletTransaction" {
					address = "Initial Wallet"
					name = "system address"
				}

				return c.Render(http.StatusOK, "address/system", echo.Map{
					"title":   &address,
					"address": &address,
					"name":    name,
				})
			}

			responseBalance, err := rpcClientTorrent.Call("fetch-balance", &metawatch.BalanceArgs{Address: address})
			if err != nil {
				return c.JSON(http.StatusBadRequest, err.Error())
			}

			var resultBalance *metawatch.Balance
			err = responseBalance.GetObject(&resultBalance)
			if err != nil {
				return c.JSON(http.StatusBadRequest, err.Error())
			}

			sql := `SELECT 
					toA, fromA, count() txs_count, sum(value) txs_amount
				FROM Transactions
				WHERE 
					(toA=? OR fromA=?) 
					AND toA NOT IN('0x666174686572206f662077616c6c65747320666f7267696e67','0x666174686572206f662073657276657220666f7267696e6720','0x007a1e062bdb4d57f9a93fd071f81f08ec8e6482c3135c55ea') 
					AND fromA NOT IN('InitialWalletTransaction','0x00bc4787973cb36f47d4f274bc340cb3e1402030955c85e563','0x00cacf8f42f4ffa95bc4a5eea3cf5986f56e13eed8ae012a67','0x00ccbc94988be95731ce3ecdccca505fed5eac1f3498ad2966','0x00b888869e8d4a193e80c59f923fe9f93fd6552875c857edbe')
					AND value > 0
				GROUP 
					BY toA, fromA 
				ORDER BY 
					` + sort + ` DESC
				LIMIT 50`

			type item struct {
				From               string `json:"from" db:"fromA"`
				To                 string `json:"to" db:"toA"`
				TransactionsCount  uint64 `json:"to_sum" db:"txs_count"`
				TransactionsAmount int64  `json:"txs_amount" db:"txs_amount"`
			}

			relatedItems := []item{}

			err = connectClickhouse.Select(&relatedItems, sql, address, address)
			if err != nil {
				return c.JSON(http.StatusBadRequest, err.Error())
			}

			return c.Render(http.StatusOK, "address/address", echo.Map{
				"title":         c.Get("addressTitle"),
				"serverNode":    c.Get("isNode"),
				"address":       address,
				"resultBalance": &resultBalance,
				"relatedItems":  &relatedItems,
			})
		})

		groupAddress.GET("/:address/txs", func(c echo.Context) error {
			address := c.Param("address")

			page := c.QueryParam("page")
			pageInt, _ := strconv.Atoi(page)

			responseBalance, err := rpcClientTorrent.Call("fetch-balance", &metawatch.BalanceArgs{Address: address})
			if err != nil {
				return c.JSON(http.StatusBadRequest, err.Error())
			}

			var resultBalance *metawatch.Balance
			err = responseBalance.GetObject(&resultBalance)
			if err != nil {
				return c.JSON(http.StatusBadRequest, err.Error())
			}

			var (
				resultHistory []*metawatch.TransactionInfo
				pagination    *Pagination
			)

			transactionsCount := resultBalance.CountTxs
			if transactionsCount > 0 {

				pagination = NewPagination(transactionsCount, int64(pageInt), txLimit, "/address/"+address+"/txs?page=")
				pagination.Init()

				args := &metawatch.HistoryArgs{
					Address:  address,
					CountTxs: pagination.limit,
					BeginTx:  pagination.start - 1,
				}

				responseHistory, err := rpcClientTorrent.Call("fetch-history", args)
				if err != nil {
					return c.JSON(http.StatusBadRequest, err.Error())
				}

				err = responseHistory.GetObject(&resultHistory)
				if err != nil {
					return c.JSON(http.StatusBadRequest, err.Error())
				}
			}

			return c.Render(http.StatusOK, "address/txs", echo.Map{
				"title":         c.Get("addressTitle"),
				"serverNode":    c.Get("isNode"),
				"address":       address,
				"resultHistory": &resultHistory,
				"pagination":    &pagination,
			})
		})

		groupAddress.GET("/:address/delegations", func(c echo.Context) error {
			address := c.Param("address")

			rows, err := connectClickhouse.Query(`
				SELECT
					toInt64(timestamp),
					transaction,
					toA,
					delegate,
					abstractMethod,
					blockNumber
				FROM
					Transactions
				WHERE
					abstractMethod IN('delegate', 'undelegate')
					AND (fromA = ?)
				ORDER BY timestamp DESC
				LIMIT ?`, address, txLimit)
			defer rows.Close()

			if err != nil {
				return c.JSON(http.StatusBadRequest, err.Error())
			}

			var resultHistory []*metawatch.TransactionInfo
			var delegate, blockNumber, timestamp int64
			var transaction, toA, abstractMethod string
			for rows.Next() {
				tx := &metawatch.TransactionInfo{}
				err := rows.Scan(&timestamp, &transaction, &toA, &delegate, &abstractMethod, &blockNumber)

				if err != nil {
					return c.JSON(http.StatusBadRequest, err.Error())
				}

				tx.TimeStamp = timestamp
				tx.Transaction = transaction
				tx.BlockNumber = blockNumber
				tx.To = toA
				tx.Delegate = delegate
				tx.Status = abstractMethod

				resultHistory = append(resultHistory, tx)
			}

			return c.Render(http.StatusOK, "address/delegations", echo.Map{
				"title":         c.Get("addressTitle"),
				"serverNode":    c.Get("isNode"),
				"address":       &address,
				"resultHistory": &resultHistory,
			})
		})

		groupAddress.GET("/:address/forging", func(c echo.Context) error {
			address := c.Param("address")

			rows, err := connectClickhouse.Query(`
				SELECT
					toInt64(timestamp),
					transaction,
					toA,
					value,
					abstractMethod,
					intStatus,
					blockNumber
				FROM
					Transactions 
				WHERE
					intStatus IN(101,103,104)
					AND (fromA = ? OR toA=?)
					AND (fromA='InitialWalletTransaction' AND toA=?)
				ORDER BY timestamp DESC
				LIMIT ?`, address, address, address, txLimit*10)
			defer rows.Close()
			if err != nil {
				return c.JSON(http.StatusBadRequest, err.Error())
			}

			var (
				resultHistory                            []*metawatch.TransactionInfo
				value, intStatus, blockNumber, timestamp int64
				transaction, toA, abstractMethod         string
			)

			for rows.Next() {
				tx := &metawatch.TransactionInfo{}
				err := rows.Scan(&timestamp, &transaction, &toA, &value, &abstractMethod, &intStatus, &blockNumber)
				if err != nil {
					return c.JSON(http.StatusBadRequest, err.Error())
				}

				tx.TimeStamp = timestamp
				tx.Transaction = transaction
				tx.BlockNumber = blockNumber
				tx.To = toA
				tx.Value = value
				tx.Status = abstractMethod
				tx.IntStatus = intStatus

				resultHistory = append(resultHistory, tx)
			}

			return c.Render(http.StatusOK, "address/forging", echo.Map{
				"title":         c.Get("addressTitle"),
				"serverNode":    c.Get("isNode"),
				"address":       address,
				"resultHistory": &resultHistory,
			})
		})

		groupAddress.GET("/:address/info", func(c echo.Context) error {
			address := c.Param("address")

			rowsBench, err := connectClickhouse.Query(`
				SELECT 
					min(visitParamExtractInt(dataString, 'rps')) minRPS, 
					max(visitParamExtractInt(dataString, 'rps')) maxRPS, 
					floor(avg(visitParamExtractInt(dataString, 'rps'))) avgRPS,
					min(visitParamExtractInt(dataString, 'qps')) minQPS, 
					max(visitParamExtractInt(dataString, 'qps')) maxQPS, 
					floor(avg(visitParamExtractInt(dataString, 'qps'))) avgQPS,
					count() benchCount
				FROM Transactions 
				WHERE date=today() 
					AND intStatus = 4353
					AND visitParamExtractString( dataString,'mhaddr') =?
				`, address)
			if err != nil {
				log.Fatal(err)
			}
			defer rowsBench.Close()

			var minRPS, maxRPS, avgRPS, minQPS, maxQPS, avgQPS, benchCount float64
			for rowsBench.Next() {
				if err := rowsBench.Scan(&minRPS, &maxRPS, &avgRPS, &minQPS, &maxQPS, &avgQPS, &benchCount); err != nil {
					log.Fatal(err)
				}
			}

			responseBalance, err := rpcClientTorrent.Call("fetch-balance", &metawatch.BalanceArgs{Address: address})

			var resultBalance *metawatch.Balance
			err = responseBalance.GetObject(&resultBalance)
			if err == nil {

				return c.Render(http.StatusOK, "address/info", echo.Map{
					"title":         c.Get("addressTitle"),
					"serverNode":    c.Get("isNode"),
					"resultBalance": &resultBalance,
					"minRPS":        minRPS,
					"maxRPS":        maxRPS,
					"avgRPS":        avgRPS,
					"minQPS":        minQPS,
					"maxQPS":        maxQPS,
					"avgQPS":        avgQPS,
					"benchCount":    benchCount,
					"address":       &address,
					"nodeData":      c.Get("currentNodeData"),
				})
			}

			return c.JSON(http.StatusBadRequest, err.Error())
		})

		groupAddress.GET("/:address/rewards", func(c echo.Context) error {
			address := c.Param("address")

			rowsTRX, err := connectClickhouse.Query(`
				SELECT 
					toInt64(timestamp),
					transaction,
					value,
					toA,
					fromA,
					status,
					intStatus,
					blockNumber,
					dataString
				FROM Transactions
				WHERE fromA='InitialWalletTransaction' AND intStatus IN(102,103) AND toA=?
				ORDER BY timestamp DESC
				LIMIT ?
			`, address, txLimit)
			if err != nil {
				log.Fatal(err)
			}
			defer rowsTRX.Close()

			var resultHistory []*metawatch.TransactionInfo
			for rowsTRX.Next() {
				var (
					transaction, toA, fromA, dataString, status string
					value, intStatus, blockNumber, timestamp    int64
				)
				if err := rowsTRX.Scan(&timestamp, &transaction, &value, &toA, &fromA, &status, &intStatus, &blockNumber, &dataString); err != nil {
					log.Fatal(err)
				}
				trxInfo := &metawatch.TransactionInfo{
					TimeStamp:   timestamp,
					Transaction: transaction,
					Value:       value,
					From:        fromA,
					To:          toA,
					Status:      status,
					IntStatus:   intStatus,
					BlockNumber: blockNumber,
				}
				resultHistory = append(resultHistory, trxInfo)
			}

			return c.Render(http.StatusOK, "address/rewards", echo.Map{
				"title":         c.Get("addressTitle"),
				"serverNode":    c.Get("isNode"),
				"address":       address,
				"resultHistory": &resultHistory,
			})
		})

		groupAddress.GET("/:address/server-delegations", func(c echo.Context) error {
			address := c.Param("address")

			limit := 1000

			rows, err := connectClickhouse.Query(`
				SELECT
					toInt64(timestamp),
					transaction,
					fromA,
					delegate,
					abstractMethod,
					blockNumber
				FROM
					Transactions
				WHERE
					abstractMethod IN('delegate', 'undelegate')
					AND (fromA = ? OR toA=?)
				ORDER BY timestamp DESC
				LIMIT ?`, address, address, limit)
			if err != nil {
				log.Fatal(err)
			}
			defer rows.Close()

			var resultHistory []*metawatch.TransactionInfo
			var delegate, blockNumber, timestamp int64
			var transaction, fromA, abstractMethod string
			for rows.Next() {
				tx := &metawatch.TransactionInfo{}
				if err := rows.Scan(&timestamp, &transaction, &fromA, &delegate, &abstractMethod, &blockNumber); err != nil {
					log.Fatal(err)
				}

				tx.TimeStamp = timestamp
				tx.Transaction = transaction
				tx.BlockNumber = blockNumber
				tx.From = fromA
				tx.Delegate = delegate
				tx.Status = abstractMethod

				resultHistory = append(resultHistory, tx)
			}

			return c.Render(http.StatusOK, "address/delegations-server", echo.Map{
				"title":         c.Get("addressTitle"),
				"serverNode":    c.Get("isNode"),
				"address":       &address,
				"resultHistory": &resultHistory,
			})
		})
	}

	e.GET("/search", func(c echo.Context) error {
		q := strings.ToLower(c.QueryParam("q"))
		qLen := len(q)

		//
		if q == "metawat.ch" || q == "metawatch" || q == "мой" {
			return c.Redirect(http.StatusMovedPermanently, "/address/0x00fa2a5279f8f0fd2f0f9d3280ad70403f01f9d62f52373833")
		}

		if q == "InitialWalletTransaction" || q == "core" || q == "init" || q == "genesis" {
			return c.Redirect(http.StatusMovedPermanently, "/address/InitialWalletTransaction")
		}
		// адрес
		if qLen == 52 && (q[0:3] == "0x0" || q[0:3] == "0x6") {
			return c.Redirect(http.StatusMovedPermanently, "/address/"+q)
		}

		// номер блока
		if qLen < 15 && qLen > 0 {
			if _, err := strconv.ParseFloat(q, 64); err == nil {
				return c.Redirect(http.StatusMovedPermanently, "/blocks/"+q)
			}
		}

		// транзакция или блок
		if qLen == 64 {
			responseBlockByHash, err := rpcClientTorrent.Call("get-block-by-hash", &metawatch.BlockByHashArgs{Hash: q, Type: 0})
			if err == nil && responseBlockByHash.Error == nil {
				return c.Redirect(http.StatusMovedPermanently, "/blocks/"+q)
			}
			return c.Redirect(http.StatusMovedPermanently, "/txs/"+q)
		}

		return echo.NotFoundHandler(c)
	})

	// транзакции кошёлька регистрации нод
	e.GET("/nodes", func(c echo.Context) error {

		page := c.QueryParam("page")
		pageInt, _ := strconv.Atoi(page)

		key := []byte("nodes:" + page)
		valuesRaw, _ := cache.Get(key)

		if valuesRaw == nil {

			responseBalance, err := rpcClientTorrent.Call("fetch-balance", &metawatch.BalanceArgs{Address: addressNodeRegistrator})
			if err != nil {
				return c.JSON(http.StatusBadRequest, err.Error())
			}

			var resultBalance *metawatch.Balance
			err = responseBalance.GetObject(&resultBalance)
			if err != nil {
				return c.JSON(http.StatusBadRequest, err.Error())
			}

			pagination := NewPagination(resultBalance.CountTxs, int64(pageInt), txLimit, "/nodes?page=")
			pagination.Init()

			args := &metawatch.HistoryArgs{
				Address:  addressNodeRegistrator,
				CountTxs: pagination.limit,
				BeginTx:  pagination.start - 1,
			}

			responseHistory, err := rpcClientTorrent.Call("fetch-history", args)
			if err != nil {
				return c.JSON(http.StatusBadRequest, err.Error())
			}

			var resultHistory []*metawatch.TransactionInfo
			err = responseHistory.GetObject(&resultHistory)
			if err != nil {
				return c.JSON(http.StatusBadRequest, err.Error())
			}

			values := echo.Map{
				"title":         "Server nodes",
				"resultHistory": &resultHistory,
				"pagination":    pagination,
			}
			valuesMarshal := gotiny.Marshal(&values)
			_ = cache.Set(key, valuesMarshal, 300)
			return c.Render(http.StatusOK, "nodes", values)
		}

		valuesCache := echo.Map{}
		gotiny.Unmarshal(valuesRaw, &valuesCache)

		return c.Render(http.StatusOK, "nodes", valuesCache)
	})

	e.GET("/nodes/:hash", func(c echo.Context) error {
		hash := c.Param("hash")
		return c.Redirect(http.StatusMovedPermanently, "/address/"+hash+"/info")
	})

	e.GET("/blocks", func(c echo.Context) error {

		page := c.QueryParam("page")
		pageInt, _ := strconv.Atoi(page)

		responseCountBlocks, err := rpcClientTorrent.Call("get-count-blocks", &metawatch.CountBlocksArgs{})

		if err != nil {
			return c.JSON(http.StatusBadRequest, err.Error())
		}

		var resultCountBlocks *metawatch.CountBlocks
		err = responseCountBlocks.GetObject(&resultCountBlocks)
		if err != nil {
			return c.JSON(http.StatusBadRequest, err.Error())
		}

		pagination := NewPagination(resultCountBlocks.CountBlocks, int64(pageInt), pageLimit, "/blocks?page=")
		pagination.Init()

		args := &metawatch.BlocksArgs{
			CountBlocks: pagination.limit,
			BeginBlock:  pagination.start,
		}

		responseBlocks, err := rpcClientTorrent.Call("get-blocks", args)
		if err != nil {
			return c.JSON(http.StatusBadRequest, err.Error())
		}

		var resultBlocks []*metawatch.Block
		err = responseBlocks.GetObject(&resultBlocks)

		return c.Render(http.StatusOK, "blocks", echo.Map{
			"title":        "Blocks",
			"resultBlocks": &resultBlocks,
			"maxBlocks":    &resultCountBlocks.CountBlocks,
			"pagination":   &pagination,
			"currentPage":  pageInt,
		})
	})

	e.GET("/blocks/:hash", func(c echo.Context) error {
		hash := c.Param("hash")

		if len(hash) < 64 {
			blockNumber, _ := strconv.Atoi(hash)
			responseBlockByNumber, err := rpcClientTorrent.Call("get-block-by-number", &metawatch.BlockByNumberArgs{Number: int64(blockNumber), Type: 2})

			if err != nil {
				return c.JSON(http.StatusBadRequest, err.Error())
			}

			var resultBlockByNumber *metawatch.Block
			err = responseBlockByNumber.GetObject(&resultBlockByNumber)

			if err != nil {
				return c.JSON(http.StatusBadRequest, err.Error())
			}

			if resultBlockByNumber == nil {
				return c.Render(http.StatusOK, "404message", echo.Map{
					"title":   "Block not found",
					"message": "Block " + hash + " not found",
				})
			}

			return c.Render(http.StatusOK, "block", echo.Map{
				"title": "Block " + strconv.FormatInt(resultBlockByNumber.Number, 10) + " - " + resultBlockByNumber.Hash,
				"block": &resultBlockByNumber,
			})
		}
		responseBlockByHash, err := rpcClientTorrent.Call("get-block-by-hash", &metawatch.BlockByHashArgs{Hash: hash, Type: 2})
		if err == nil {
			var resultBlockByHash *metawatch.Block
			err = responseBlockByHash.GetObject(&resultBlockByHash)
			if err == nil {
				return c.Render(http.StatusOK, "block", echo.Map{
					"title": "Block " + strconv.FormatInt(resultBlockByHash.Number, 10) + " - " + resultBlockByHash.Hash,
					"block": &resultBlockByHash,
				})
			}
		}
		return c.JSON(http.StatusBadRequest, err.Error())
	})

	e.GET("/forging", func(c echo.Context) error {

		// @todo тут кеш надо
		rows, err := connectClickhouse.Query(`
			SELECT
				toInt64(timestamp),
				transaction,
				toA,
				value,
				abstractMethod
			FROM Transactions
			WHERE date=today() AND fromA='InitialWalletTransaction' AND intStatus=104
			ORDER BY value DESC`)
		if err != nil {
			log.Fatal(err)
		}
		defer rows.Close()

		var (
			resultHistory                    []*metawatch.TransactionInfo
			value, timestamp                 int64
			transaction, toA, abstractMethod string
		)

		for rows.Next() {
			tx := &metawatch.TransactionInfo{}
			if err := rows.Scan(&timestamp, &transaction, &toA, &value, &abstractMethod); err != nil {
				log.Fatal(err)
			}
			tx.TimeStamp = timestamp
			tx.Transaction = transaction
			tx.To = toA
			tx.Value = value
			tx.Status = abstractMethod

			resultHistory = append(resultHistory, tx)
		}

		return c.Render(http.StatusOK, "forging", echo.Map{
			"title":         "Forging bonus (today)",
			"date":          time.Now().UTC().Format("2006-01-02"),
			"resultHistory": &resultHistory,
		})
	})

	e.GET("/forging/:date", func(c echo.Context) error {

		date := c.Param("date")

		dateTime, err := time.Parse("2006-01-02", date)

		if err != nil {
			fmt.Println(err)
		}

		rows, err := connectClickhouse.Query(`
		SELECT
			toInt64(timestamp),
			transaction,
			toA,
			value,
			abstractMethod
		FROM Transactions
		WHERE date=toDate(?) AND fromA='InitialWalletTransaction' AND intStatus=104
		ORDER BY value DESC`, dateTime.Format("2006-01-02"))
		if err != nil {
			log.Fatal(err)
		}
		defer rows.Close()

		var resultHistory []*metawatch.TransactionInfo
		var value, timestamp int64
		var transaction, toA, abstractMethod string
		for rows.Next() {
			tx := &metawatch.TransactionInfo{}
			if err := rows.Scan(&timestamp, &transaction, &toA, &value, &abstractMethod); err != nil {
				log.Fatal(err)
			}
			tx.TimeStamp = timestamp
			tx.Transaction = transaction
			tx.To = toA
			tx.Value = value
			tx.Status = abstractMethod

			resultHistory = append(resultHistory, tx)
		}

		return c.Render(http.StatusOK, "forging", echo.Map{
			"title":         "Forging bonus (" + dateTime.Format("2006-01-02") + ")",
			"date":          dateTime.Format("2006-01-02"),
			"resultHistory": &resultHistory,
		})
	})

	// @todo - убрать в отдельный проект
	e.GET("/payment", func(c echo.Context) error {

		return c.Render(http.StatusOK, "payment", echo.Map{
			"title":   "Payment box",
			"address": "",
			"amount":  "",
			"message": "",
			"payurl":  "",
		})
	})

	e.GET("/payment/:address", func(c echo.Context) error {

		address := c.Param("address")

		return c.Render(http.StatusOK, "payment", echo.Map{
			"title":            "Payment box " + address,
			"address":          address,
			"address_readonly": "readonly",
			"amount":           "",
			"message":          "",
			"payurl":           "",
		})
	})

	e.GET("/payment/:address/:amount", func(c echo.Context) error {

		address := c.Param("address")
		amountString := c.Param("amount")

		amount, err := strconv.ParseFloat(amountString, 10)
		if amountString == "" || err != nil {
			return c.Redirect(http.StatusMovedPermanently, "/payment/"+address)
		}

		return c.Render(http.StatusOK, "payment", echo.Map{
			"title":            "Payment box " + address,
			"address":          address,
			"address_readonly": "readonly",
			"amount":           amount,
			"amount_readonly":  "readonly",
			"message":          "",
			"payurl":           "",
		})
	})

	e.GET("/payment/:address/:amount/pay", func(c echo.Context) error {

		address := c.Param("address")
		amountString := c.Param("amount")

		amount, err := strconv.ParseFloat(amountString, 10)
		if amountString == "" || err != nil {
			return c.Redirect(http.StatusMovedPermanently, "/payment/"+address)
		}

		urlForPay := "metapay://pay.metahash.org/?currency=mhc&to=" + address + "&description=autometapay%20from%20metawat.ch&data=&value=" + amountString
		log.Println("pay", urlForPay)

		return c.Render(http.StatusOK, "payment", echo.Map{
			"title":            "Payment box " + address,
			"address":          address,
			"address_readonly": "readonly",
			"amount":           amount,
			"amount_readonly":  "readonly",
			"message":          "",
			"payurl":           urlForPay,
		})
	})

	e.POST("/payment", func(c echo.Context) error {

		to := c.FormValue("to")
		amount := c.FormValue("amount")
		description := c.FormValue("message")

		src := []byte(description)
		data := make([]byte, hex.EncodedLen(len(src)))
		hex.Encode(data, src)
		if err != nil {
			log.Fatal(err)
		}

		dataString := string(data)
		urlForPay := "metapay://pay.metahash.org/?currency=mhc&to=" + to + "&description=" + url.QueryEscape(description) + "&data=" + dataString + "&value=" + amount

		return c.Render(http.StatusOK, "payment", echo.Map{
			"title":    "Payment box",
			"address":  to,
			"amount":   amount,
			"message":  description,
			"payurl":   urlForPay,
			"disabled": "disabled",
		})
	})

	e.GET("/status", func(c echo.Context) error {
		data := getUpdateSystemStatus()
		data["title"] = "MetaHash TraceChain status board"
		return c.Render(http.StatusOK, "status", data)
	})

	e.GET("/about", func(c echo.Context) error {

		return c.Render(http.StatusOK, "about", echo.Map{
			"title": "About",
		})
	})

	e.GET("/map", func(c echo.Context) error {

		type marker struct {
			Address   string  `json:"address"`
			Name      string  `json:"name"`
			Location  string  `json:"location"`
			Latitude  float32 `json:"lat"`
			Longitude float32 `json:"lng"`
			IsOnline  bool    `json:"is_online" db:"is_online"`
		}

		sql := "SELECT address, name, CONCAT(country_long, IF(city='','', CONCAT(', ',city))) AS location, latitude, longitude, is_online FROM nodes WHERE is_online=1"

		markers := []marker{}
		err = connectMysql.Select(&markers, sql)
		if err != nil {
			log.Println(err.Error())
		}

		return c.Render(http.StatusOK, "map", echo.Map{
			"title":   "Network GEO",
			"markers": markers,
		})
	})

	{ // API
		apiv1 := e.Group("/api/v1")

		apiv1.GET("/nodes/:hash/proxy-load.json", func(c echo.Context) error {
			hash := c.Param("hash")

			rows, _ := connectClickhouse.Query(`
		SELECT * FROM(
			SELECT 
				timestamp,
				transaction,
				fromA,
				toA,
				visitParamExtractInt(dataString,'rps') rps,
				visitParamExtractInt(dataString,'qps') qps,
				visitParamExtractString( dataString,'mhaddr') mhaddr
			FROM Transactions
			WHERE intStatus = 4353 AND mhaddr=?
			ORDER BY timestamp DESC
		) ORDER BY timestamp ASC
		`, hash)
			defer rows.Close()

			pointsRPS := [][]string{}
			pointsQPS := [][]string{}
			for rows.Next() {
				var (
					timestamp                       time.Time
					rps, qps                        int64
					transaction, fromA, toA, mhaddr string
				)
				if err := rows.Scan(&timestamp, &transaction, &fromA, &toA, &rps, &qps, &mhaddr); err != nil {
					log.Fatal(err)
				}

				nodeName := hashtrim(fromA)

				pointsRPS = append(pointsRPS, []string{timestamp.Format("2006-01-02 15:04"), nodeName, strconv.FormatInt(rps, 10)})
				pointsQPS = append(pointsQPS, []string{timestamp.Format("2006-01-02 15:04"), nodeName, strconv.FormatInt(qps, 10)})
			}

			result := struct {
				RPS [][]string `json:"rps"`
				QPS [][]string `json:"qps"`
			}{
				RPS: pointsRPS,
				QPS: pointsQPS,
			}

			return c.JSON(http.StatusOK, result)
		})

		apiv1.GET("/nodes/:hash/delegations.json", func(c echo.Context) error {
			hash := c.Param("hash")
			rows, _ := connectClickhouse.Query(`
		SELECT
			toStartOfInterval(timestamp, INTERVAL 1 hour) tt,
			countIf(abstractMethod='delegate') delegate,
			countIf(abstractMethod='undelegate') undelegate
		FROM Transactions
		WHERE abstractMethod IN('delegate','undelegate') AND toA=?
		GROUP BY tt
		ORDER BY tt ASC`, hash)
			defer rows.Close()

			times := []string{}
			delegates := []int64{}
			undelegates := []int64{}

			for rows.Next() {
				var (
					timestamp                      time.Time
					delegateCount, undelegateCount int64
				)
				if err := rows.Scan(&timestamp, &delegateCount, &undelegateCount); err != nil {
					log.Fatal(err)
					continue
				}

				times = append(times, timestamp.Format("Jan _2 15:04:05"))
				delegates = append(delegates, delegateCount)
				undelegates = append(undelegates, undelegateCount)
			}

			return c.JSON(http.StatusOK, echo.Map{
				"time":        times,
				"delegates":   delegates,
				"undelegates": undelegates,
			})
		})

		apiv1.GET("/nodes/:hash/rewards.json", func(c echo.Context) error {
			hash := c.Param("hash")
			rows, _ := connectClickhouse.Query(`
		SELECT 
			date,
			sum(value/1e6)
		FROM Transactions
		WHERE 
			intStatus=102 AND toA=?
		GROUP BY date
		ORDER BY date ASC`, hash)
			defer rows.Close()

			times := []string{}
			nodeRewards := []float64{}

			for rows.Next() {
				var (
					timestamp  time.Time
					forgingSum float64
				)
				if err := rows.Scan(&timestamp, &forgingSum); err != nil {
					log.Fatal(err)
					continue
				}

				nodeRewards = append(nodeRewards, forgingSum)
				times = append(times, timestamp.Format("Jan _2"))
			}

			return c.JSON(http.StatusOK, echo.Map{
				"time":    times,
				"rewards": nodeRewards,
			})
		})

		apiv1.GET("/nodes.json", func(c echo.Context) error {

			type point struct {
				Name            string  `json:"name"  db:"name"`
				Address         string  `json:"address"  db:"address"`
				CountryLong     string  `json:"country_long"  db:"country_long"`
				DelegatedAmount float64 `json:"delegated_amount"  db:"delegated_amount"`
			}

			nodes := []point{}
			err = connectMysql.Select(&nodes, `SELECT nodes.address, nodes.name, nodes.country_long, addresses.delegated_amount AS delegated_amount
			FROM nodes
			INNER JOIN addresses ON (nodes.address=addresses.address)
			ORDER BY nodes.last_updated DESC`)
			if err != nil {
				log.Fatal(err.Error())
			}

			return c.JSON(http.StatusOK, echo.Map{
				"data": nodes,
			})
		})

		apiv1.GET("/status.json", func(c echo.Context) error {
			return c.JSON(http.StatusOK, getUpdateSystemStatus())
		})

		apiv1.GET("/status/txs.json", func(c echo.Context) error {

			key := []byte("status_trx")
			valuesRaw, _ := cache.Get(key)

			if valuesRaw == nil {

				rows, _ := connectClickhouse.Query(`
				SELECT
					timestamp tt,
					count()
				FROM Transactions
				WHERE date >= today()-1 AND timestamp >= (NOW()-INTERVAL 24 HOUR)
				GROUP BY tt
				ORDER BY tt ASC`)
				defer rows.Close()

				times := []string{}
				trxCount := []int64{}
				for rows.Next() {
					var (
						timestamp time.Time
						count     int64
					)
					if err := rows.Scan(&timestamp, &count); err != nil {
						log.Fatal(err)
						continue
					}

					times = append(times, timestamp.Format("Jan _2 15:04:05"))
					trxCount = append(trxCount, count)
				}

				values := echo.Map{
					"time": times,
					"trx":  trxCount,
				}
				valuesMarshal := gotiny.Marshal(&values)
				_ = cache.Set(key, valuesMarshal, 5)
				return c.JSON(http.StatusOK, values)
			}

			valuesCache := echo.Map{}
			gotiny.Unmarshal(valuesRaw, &valuesCache)

			return c.JSON(http.StatusOK, valuesCache)
		})

		apiv1.GET("/status/txs_date.json", func(c echo.Context) error {

			key := []byte("txs_date")
			valuesRaw, _ := cache.Get(key)

			if valuesRaw == nil {

				rows, _ := connectClickhouse.Query(`
				SELECT
					date tt,
					count()
				FROM Transactions
				GROUP BY tt
				ORDER BY tt ASC`)
				defer rows.Close()

				times := []string{}
				trxCount := []int64{}
				for rows.Next() {
					var (
						timestamp time.Time
						count     int64
					)
					if err := rows.Scan(&timestamp, &count); err != nil {
						log.Fatal(err)
						continue
					}

					times = append(times, timestamp.Format("Jan _2"))
					trxCount = append(trxCount, count)
				}

				values := echo.Map{
					"time": times,
					"trx":  trxCount,
				}
				valuesMarshal := gotiny.Marshal(&values)
				_ = cache.Set(key, valuesMarshal, 5)
				return c.JSON(http.StatusOK, values)
			}

			valuesCache := echo.Map{}
			gotiny.Unmarshal(valuesRaw, &valuesCache)

			return c.JSON(http.StatusOK, valuesCache)
		})

		apiv1.GET("/status/wallets.json", func(c echo.Context) error {

			key := []byte("status_wallets")
			valuesRaw, _ := cache.Get(key)

			if valuesRaw == nil {

				rows, _ := connectClickhouse.Query(`
			SELECT
				date,
				countDistinct(toA),
				countDistinct(fromA)
			FROM Transactions
			GROUP BY date
			ORDER BY date ASC`)
				defer rows.Close()

				times := []string{}
				walletsUniq := []int64{}
				walletsTotal := []int64{}
				for rows.Next() {
					var (
						timestamp   time.Time
						uniq, total int64
					)
					if err := rows.Scan(&timestamp, &total, &uniq); err != nil {
						log.Fatal(err)
						continue
					}

					times = append(times, timestamp.Format("Jan _2"))
					walletsTotal = append(walletsTotal, total)
					walletsUniq = append(walletsUniq, uniq)
				}

				values := echo.Map{
					"time":          times,
					"wallets_uniq":  walletsUniq,
					"wallets_total": walletsTotal,
				}
				valuesMarshal := gotiny.Marshal(&values)
				_ = cache.Set(key, valuesMarshal, 60*15)
				return c.JSON(http.StatusOK, values)
			}

			valuesCache := echo.Map{}
			gotiny.Unmarshal(valuesRaw, &valuesCache)

			return c.JSON(http.StatusOK, valuesCache)
		})

		apiv1.GET("/status/delegations.json", func(c echo.Context) error {

			key := []byte("status_delegations")
			valuesRaw, _ := cache.Get(key)

			if valuesRaw == nil {

				rows, _ := connectClickhouse.Query(`
			SELECT
				toStartOfInterval(timestamp, INTERVAL 1 hour) tt,
				countIf(abstractMethod='delegate') delegate,
				countIf(abstractMethod='undelegate') undelegate
			FROM Transactions
			WHERE abstractMethod IN('delegate','undelegate') AND intStatus=20 AND toA<>'0x666174686572206f662077616c6c65747320666f7267696e67'
			GROUP BY tt
			ORDER BY tt ASC`)
				defer rows.Close()

				times := []string{}
				delegates := []int64{}
				undelegates := []int64{}

				for rows.Next() {
					var (
						timestamp                      time.Time
						delegateCount, undelegateCount int64
					)
					if err := rows.Scan(&timestamp, &delegateCount, &undelegateCount); err != nil {
						log.Fatal(err)
						continue
					}

					times = append(times, timestamp.Format("Jan _2 15:04:05"))
					delegates = append(delegates, delegateCount)
					undelegates = append(undelegates, undelegateCount)
				}

				values := echo.Map{
					"time":        times,
					"delegates":   delegates,
					"undelegates": undelegates,
				}
				valuesMarshal := gotiny.Marshal(&values)
				_ = cache.Set(key, valuesMarshal, 60*15)
				return c.JSON(http.StatusOK, values)
			}

			valuesCache := echo.Map{}
			gotiny.Unmarshal(valuesRaw, &valuesCache)

			return c.JSON(http.StatusOK, valuesCache)
		})

		apiv1.GET("/status/delegation_sum.json", func(c echo.Context) error {

			key := []byte("status_delegation_sum")
			valuesRaw, _ := cache.Get(key)

			if valuesRaw == nil {

				rows, _ := connectClickhouse.Query(`
				SELECT
					date,
					sum(delegate)/1e6 delegateSum
				FROM Transactions
				WHERE abstractMethod='delegate' AND intStatus=20 AND toA<>'0x666174686572206f662077616c6c65747320666f7267696e67'
				GROUP BY date
				ORDER BY date ASC`)
				defer rows.Close()

				times := []string{}
				delegates := []float64{}

				for rows.Next() {
					var (
						timestamp   time.Time
						delegateSum float64
					)
					if err := rows.Scan(&timestamp, &delegateSum); err != nil {
						log.Fatal(err)
						continue
					}

					times = append(times, timestamp.Format("Jan _2"))
					delegates = append(delegates, delegateSum)
				}

				values := echo.Map{
					"time":      times,
					"delegates": delegates,
				}
				valuesMarshal := gotiny.Marshal(&values)
				_ = cache.Set(key, valuesMarshal, 60*15)
				return c.JSON(http.StatusOK, values)
			}

			valuesCache := echo.Map{}
			gotiny.Unmarshal(valuesRaw, &valuesCache)

			return c.JSON(http.StatusOK, valuesCache)
		})

		apiv1.GET("/status/amount_sum.json", func(c echo.Context) error {

			key := []byte("status_amount_sum")
			valuesRaw, _ := cache.Get(key)

			if valuesRaw == nil {

				rows, _ := connectClickhouse.Query(`
				SELECT
					date tt,
					sum( value )/1e6 value
				FROM Transactions
				WHERE intStatus=20 AND typeTx='block'
				GROUP BY tt
				ORDER BY tt ASC`)
				defer rows.Close()

				times := []string{}
				sum := []float64{}

				for rows.Next() {
					var (
						timestamp   time.Time
						delegateSum float64
					)
					if err := rows.Scan(&timestamp, &delegateSum); err != nil {
						log.Fatal(err)
						continue
					}

					times = append(times, timestamp.Format("Jan _2"))
					sum = append(sum, delegateSum)
				}

				values := echo.Map{
					"time": times,
					"sum":  sum,
				}
				valuesMarshal := gotiny.Marshal(&values)
				_ = cache.Set(key, valuesMarshal, 60*15)
				return c.JSON(http.StatusOK, values)
			}

			valuesCache := echo.Map{}
			gotiny.Unmarshal(valuesRaw, &valuesCache)

			return c.JSON(http.StatusOK, valuesCache)
		})

		apiv1.GET("/status/forging.json", func(c echo.Context) error {

			key := []byte("status_forging")
			valuesRaw, _ := cache.Get(key)

			if valuesRaw == nil {

				rows, _ := connectClickhouse.Query(`
				SELECT 
					toStartOfHour(timestamp) tt,
					countIf(abstractMethod='start forging') starts,
					countIf(abstractMethod='stop forging') stops
				FROM Transactions
				WHERE intStatus=20 AND toA='0x666174686572206f662077616c6c65747320666f7267696e67'
				GROUP BY tt
				ORDER BY tt ASC`)
				defer rows.Close()

				times := []string{}
				starts := []int64{}
				stops := []int64{}

				for rows.Next() {
					var (
						timestamp               time.Time
						startsCount, stopsCount int64
					)
					if err := rows.Scan(&timestamp, &startsCount, &stopsCount); err != nil {
						log.Fatal(err)
						continue
					}

					times = append(times, timestamp.Format("Jan _2 15:04"))
					starts = append(starts, startsCount)
					stops = append(stops, stopsCount)
				}

				values := echo.Map{
					"time":   times,
					"starts": starts,
					"stops":  stops,
				}
				valuesMarshal := gotiny.Marshal(&values)
				_ = cache.Set(key, valuesMarshal, 60*15)
				return c.JSON(http.StatusOK, values)
			}

			valuesCache := echo.Map{}
			gotiny.Unmarshal(valuesRaw, &valuesCache)

			return c.JSON(http.StatusOK, valuesCache)
		})

		apiv1.GET("/address/:address/txs.json", func(c echo.Context) error {

			address := c.Param("address")

			key := []byte("address_txs_" + address)
			valuesRaw, _ := cache.Get(key)

			var result struct {
				To       []float32 `json:"to" db:"toAcount"`
				From     []float32 `json:"from" db:"fromAcount"`
				ToSum    []float32 `json:"to_sum" db:"toAcountSum"`
				FromSumm []float32 `json:"from_sum" db:"fromAcountSum"`
				Time     []string  `json:"time" db:"tt"`
			}

			if valuesRaw == nil {

				sql := `SELECT 
					toStartOfHour(timestamp) tt,
					sumIf(value,toA=?)/1e6 toAcountSum,
					sumIf(value, fromA=?)/1e6 fromAcountSum,
					countIf(toA=?) toAcount,
					countIf(fromA=?) fromAcount
				FROM Transactions
				WHERE intStatus=20 AND (toA=? OR fromA=?)
				GROUP BY tt
				ORDER BY tt ASC`

				type point struct {
					To       float32   `json:"to" db:"toAcount"`
					From     float32   `json:"from" db:"fromAcount"`
					ToSum    float32   `json:"to_sum" db:"toAcountSum"`
					FromSumm float32   `json:"from_sum" db:"fromAcountSum"`
					Time     time.Time `json:"time" db:"tt"`
				}

				points := []point{}

				err = connectClickhouse.Select(&points, sql, address, address, address, address, address, address)
				if err != nil {
					return c.JSON(http.StatusBadRequest, err.Error())
				}

				for _, point := range points {
					result.To = append(result.To, point.To)
					result.From = append(result.From, point.From)
					result.ToSum = append(result.ToSum, point.ToSum)
					result.FromSumm = append(result.FromSumm, point.FromSumm)
					result.Time = append(result.Time, point.Time.Format("2006-01-02 15:00"))
				}

				valuesMarshal := gotiny.Marshal(&result)
				_ = cache.Set(key, valuesMarshal, 60*15)
				return c.JSON(http.StatusOK, &result)
			}

			valuesCache := result
			gotiny.Unmarshal(valuesRaw, &valuesCache)

			return c.JSON(http.StatusOK, valuesCache)
		})

		apiv1.GET("/address/:address/txs_stat.json", func(c echo.Context) error {

			address := c.Param("address")

			all := c.QueryParam("all")
			countTxs := c.QueryParam("countTxs")

			countTxsI, _ := strconv.Atoi(countTxs)
			if countTxsI == 0 && all == "" {
				countTxsI = txLimit
			}

			responseHistory, err := rpcClientTorrent.Call("fetch-history", &metawatch.HistoryArgs{Address: address, CountTxs: int64(countTxsI)})

			if err == nil {
				return c.JSON(http.StatusOK, &responseHistory)
			}

			return c.JSON(http.StatusBadRequest, err.Error())
		})

		apiv1.GET("/status/size.json", func(c echo.Context) error {

			key := []byte("api_sizes")
			valuesRaw, _ := cache.Get(key)

			if valuesRaw == nil {

				type blockSizeModel struct {
					BlockHour string `json:"block_hour" db:"block_hour"`
					FullSize  int64  `json:"full_size" db:"full_size"`
				}

				var (
					listBlockSizes = []blockSizeModel{}
					sqlBlockSizes  = `SELECT block_hour, MAX(full_size) full_size FROM(
						SELECT DATE_FORMAT(timestamp,'%Y-%m-%d') as block_hour, SUM(size) over(order by number range between unbounded preceding and current row) full_size FROM blocks
					) s1
					GROUP BY block_hour`
				)

				err = connectMysql.Select(&listBlockSizes, sqlBlockSizes)
				if err != nil {
					log.Println(err.Error())
				}

				var (
					blockHour []string
					fullSize  []int64
				)

				for _, blockSizeInfo := range listBlockSizes {
					blockHour = append(blockHour, blockSizeInfo.BlockHour)
					fullSize = append(fullSize, blockSizeInfo.FullSize)
				}

				values := echo.Map{
					"block_hour": blockHour,
					"full_size":  fullSize,
				}
				valuesMarshal := gotiny.Marshal(&values)
				_ = cache.Set(key, valuesMarshal, 60*55)
				return c.JSON(http.StatusOK, values)
			}

			valuesCache := echo.Map{}
			gotiny.Unmarshal(valuesRaw, &valuesCache)

			return c.JSON(http.StatusOK, valuesCache)
		})

		apiv1.GET("/status/blocks.json", func(c echo.Context) error {

			key := []byte("api_blocks")
			valuesRaw, _ := cache.Get(key)

			if valuesRaw == nil {

				rows, _ := connectClickhouse.Query(`
					SELECT date, countDistinct(blockNumber)
					FROM Transactions
					GROUP BY date
					ORDER BY date ASC`)
				defer rows.Close()

				var (
					dates       = []string{}
					blockCounts = []int64{}
				)

				for rows.Next() {
					var (
						date       time.Time
						blockCount int64
					)
					if err := rows.Scan(&date, &blockCount); err != nil {
						log.Fatal(err)
						continue
					}

					dates = append(dates, date.Format("Jan _2"))
					blockCounts = append(blockCounts, blockCount)
				}

				values := echo.Map{
					"date":         dates,
					"block_counts": blockCounts,
				}
				valuesMarshal := gotiny.Marshal(&values)
				_ = cache.Set(key, valuesMarshal, 60*5)
				return c.JSON(http.StatusOK, values)
			}

			valuesCache := echo.Map{}
			gotiny.Unmarshal(valuesRaw, &valuesCache)

			return c.JSON(http.StatusOK, valuesCache)
		})

		apiv1.GET("/nodes/list.json", func(c echo.Context) error {

			key := []byte("index_page_api")
			valuesRaw, _ := cache.Get(key)

			if valuesRaw == nil {
				nodes := []IndexNodePoint{}
				err = connectMysql.Select(&nodes, `SELECT nodes.address, node_type, name, mg_trust, mg_geo, mg_roi, addresses.delegated_amount AS delegated_amount
					FROM nodes
					INNER JOIN addresses ON (nodes.address=addresses.address AND addresses.delegated_amount>= 100000*1e6 AND addresses.delegated_amount <= 10000000*1e6)
					WHERE mg_status=1 AND mg_trust<>'0.001' 
					ORDER BY ROUND(delegated_amount/1e11,0) ASC, mg_trust DESC, mg_roi DESC
					LIMIT 500`) // AND mg_roi<>'0.000000'
				if err != nil {
					log.Fatal(err.Error())
				}

				values := echo.Map{
					"nodes": nodes,
				}

				valuesMarshal := gotiny.Marshal(&values)
				_ = cache.Set(key, valuesMarshal, 60)
				return c.JSON(http.StatusOK, values)
			}

			valuesCache := echo.Map{}
			gotiny.Unmarshal(valuesRaw, &valuesCache)

			return c.JSON(http.StatusOK, valuesCache)
		})
	}

	e.Logger.Fatal(e.Start(":8000"))
}

// пытается определить название для каждого адреса
func addAddressInfo(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {

		address := c.Param("address")

		if address == "" {
			return next(c)
		}

		if len(address) != 52 && address != "InitialWalletTransaction" {
			return echo.NotFoundHandler(c)
		}

		addressTitle := "Wallet " + address
		isNode := false

		node := Node{}
		err = connectMysql.Get(&node, "SELECT *, IFNULL(last_checked,last_updated) AS last_checked FROM nodes WHERE address=? LIMIT 1", address)
		if err == nil && node.Address != "" {
			addressTitle = "Node " + node.Name
			isNode = true
		} else {
			// log.Println("addAddressInfo", err.Error())
		}

		err = nil

		if n, ok := nodeNames[address]; ok {
			addressTitle = n
		}

		c.Set("addressTitle", addressTitle)
		c.Set("isNode", isNode)
		c.Set("currentNodeData", node)

		go updateAddress(address)

		return next(c)
	}
}

// отправляет адресс в очередь обновления
func updateAddress(address string) {

	if address == "InitialWalletTransaction" {
		return
	}

	var addressData struct {
		Address string
	}
	addressData.Address = address

	message, err := msgpack.Marshal(&addressData)
	if err != nil {
		log.Println("Ошибка сериализации данных модели")
	}

	if err = producer.Publish("update-address", message); err != nil {
		log.Println("producer.Publish (error)")
	}

}

func updatePrice() {

	var args = []*metawatch.CurrencyStatArgs{}
	args = append(args, &metawatch.CurrencyStatArgs{Type: "time24"})

	responseCurrencyStat, err := rpcClientWallet.CallWID("currency.stat", args)
	if err == nil {
		var resultCurrencyStat *metawatch.CurrencyStatMulti
		err = responseCurrencyStat.GetObject(&resultCurrencyStat)

		if err == nil && len(resultCurrencyStat.MHC) > 0 {
			lastPrice, _ := strconv.ParseFloat(resultCurrencyStat.MHC[len(resultCurrencyStat.MHC)-1].Val, 10)
			firstPrice, _ := strconv.ParseFloat(resultCurrencyStat.MHC[0].Val, 10)
			currentPrice = lastPrice

			currentPriceDIFF = (100 / firstPrice * lastPrice)
			currentPriceDIFF = (100 - currentPriceDIFF)
			if currentPriceDIFF < 100 {
				currentPriceDIFF = -currentPriceDIFF
			}

			currentPriceDIFF = math.Round(currentPriceDIFF*100) / 100

			log.Println("update price", currentPrice, currentPriceDIFF)

			key := []byte("status")
			cache.Del(key)
		}
	}
}

func updateNodenames() {

	type node struct {
		Address string `json:"address"`
		Name    string `json:"name"`
	}

	sql := "select name, address from nodes where name<>''"

	nodes := []node{}
	err = connectMysql.Select(&nodes, sql)
	if err != nil {
		log.Println(err.Error())
		return
	}

	nodeNames = map[string]string{}

	nodeNames = *&NodeNamesBase

	for _, node := range nodes {

		name := strings.TrimSpace(node.Name)
		if name == "" {
			name = "Node " + node.Address[0:11]
		}

		nodeNames[node.Address] = name
	}

	log.Println("update nodes", len(nodeNames))
}

func getUpdateSystemStatus() echo.Map {
	key := []byte("status")
	valuesRaw, _ := cache.Get(key)

	if valuesRaw == nil {

		rows, _ := connectClickhouse.Query(`
		SELECT max(cc) tps_max,floor(avg(cc)) tps_avg, sum(cc) sum_tx FROM(
			SELECT
				timestamp tt,
				count() cc
			FROM Transactions
			WHERE date >= today()-1 AND timestamp>=(NOW()-INTERVAL 24 HOUR)
			GROUP BY tt
		)`)
		defer rows.Close()

		var tpsMax, tpsAvg, sumTx int64
		for rows.Next() {

			if err := rows.Scan(&tpsMax, &tpsAvg, &sumTx); err != nil {
				log.Fatal(err)
				continue
			}
		}

		rows1, _ := connectClickhouse.Query(`SELECT max(value), sum(value) FROM Transactions WHERE date >= today()-1 AND timestamp>=(NOW()-INTERVAL 24 HOUR) AND intStatus=20`)
		defer rows1.Close()

		var maxValue, sumValue int64
		for rows1.Next() {
			if err := rows1.Scan(&maxValue, &sumValue); err != nil {
				log.Fatal(err)
				continue
			}
		}

		rows2, _ := connectClickhouse.Query(`SELECT max(blockNumber), count() FROM Transactions`)
		defer rows2.Close()

		var maxBlockNumber, trxCount int64
		for rows2.Next() {
			if err := rows2.Scan(&maxBlockNumber, &trxCount); err != nil {
				log.Fatal(err)
				continue
			}
		}

		rows3, _ := connectClickhouse.Query(`SELECT countIf( timestamp>=(NOW()-INTERVAL 24 HOUR) ) FROM Transactions WHERE abstractMethod IN('mh-noderegistration','mhRegisterNode')`)
		defer rows3.Close()

		var nodesCount, nodes24h int64
		for rows3.Next() {
			if err := rows3.Scan(&nodes24h); err != nil {
				log.Fatal(err)
				continue
			}
		}

		err = connectMysql.Get(&nodesCount, "SELECT count(*) FROM nodes WHERE is_online=1")
		if err != nil {
			log.Fatal(err)
		}

		rows4, _ := connectClickhouse.Query(`SELECT countDistinct(toA) FROM Transactions WHERE value>0 AND fromA<>'InitialWalletTransaction'`)
		defer rows4.Close()

		var walletsCount, wallets24h int64
		for rows4.Next() {
			if err := rows4.Scan(&walletsCount); err != nil {
				log.Fatal(err)
				continue
			}
		}

		rows5, _ := connectClickhouse.Query(`SELECT countDistinct(toA) FROM Transactions WHERE value>0 AND fromA<>'InitialWalletTransaction' AND timestamp>=(NOW()-INTERVAL 24 HOUR)`)
		defer rows5.Close()

		for rows5.Next() {
			if err := rows5.Scan(&wallets24h); err != nil {
				log.Fatal(err)
				continue
			}
		}

		var delegatedAmount int64
		err = connectMysql.Get(&delegatedAmount, "SELECT ROUND(SUM(delegated_amount)/1e6) Delegated FROM addresses WHERE delegated_amount>0")
		if err != nil {
			log.Fatal(err)
		}

		values := echo.Map{
			"tpsMax":              tpsMax,
			"tpsAvg":              tpsAvg,
			"sumTx":               sumTx,
			"maxValue":            maxValue,
			"sumValue":            sumValue,
			"maxBlockNumber":      maxBlockNumber,
			"trxCount":            trxCount,
			"nodesCount":          nodesCount,
			"nodes24h":            nodes24h,
			"walletsCount":        walletsCount,
			"wallets24h":          wallets24h,
			"currentPriceUSD":     currentPrice,
			"currentPriceUSDDIFF": currentPriceDIFF,
			"delegatedAmount":     delegatedAmount,
		}

		valuesMarshal, _ := msgpack.Marshal(&values)
		_ = cache.Set(key, valuesMarshal, 120)

		log.Println("update status without cache")

		return values
	}

	valuesCache := echo.Map{}
	msgpack.Unmarshal(valuesRaw, &valuesCache)

	log.Println("update status with cache")

	return valuesCache
}

type IndexNodePoint struct {
	Address   string `json:"address" db:"address"`
	Type      string `json:"type" db:"node_type"`
	Name      string `json:"name" db:"name"`
	Delegated int64  `json:"delegated_amount" db:"delegated_amount"`
	Trust     string `json:"mg_trust" db:"mg_trust"`
	Geo       string `json:"mg_geo" db:"mg_geo"`
	ROI       string `json:"mg_roi" db:"mg_roi"`
}

// @todo выводить разные хардкапы для разных типов нод
func (b *IndexNodePoint) ToHardCap() int64 {

	if b.Delegated == 0 {
		return 0
	}

	// 10000000000000 - сумма для хардкапа
	return 10000000000000 - b.Delegated
}
