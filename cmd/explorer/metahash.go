package main

import (
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

type CurrencyStatArgs struct {
	Type string `json:"type"`
}

// только для last
type CurrencyStat struct {
	TMH struct {
		ID          string `json:"id"`
		Active      string `json:"active"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Val         string `json:"val"`
	} `json:"1"`
	BTC struct {
		ID          string `json:"id"`
		Active      string `json:"active"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Val         string `json:"val"`
	} `json:"2"`
	ETH struct {
		ID          string `json:"id"`
		Active      string `json:"active"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Val         string `json:"val"`
	} `json:"3"`
	MHC struct {
		ID          string `json:"id"`
		Active      string `json:"active"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Val         string `json:"val"`
	} `json:"4"`
}

// для временных промежутков статистики
type CurrencyStatMulti struct {
	TMH []struct {
		Ts    string `json:"ts"`
		Val   string `json:"val"`
		ToBtc string `json:"to_btc"`
		ToEth string `json:"to_eth"`
	} `json:"1"`
	BTC []struct {
		Ts    string `json:"ts"`
		Val   string `json:"val"`
		ToBtc string `json:"to_btc"`
		ToEth string `json:"to_eth"`
	} `json:"2"`
	ETH []struct {
		Ts    string `json:"ts"`
		Val   string `json:"val"`
		ToBtc string `json:"to_btc"`
		ToEth string `json:"to_eth"`
	} `json:"3"`
	MHC []struct {
		Ts    string `json:"ts"`
		Val   string `json:"val"`
		ToBtc string `json:"to_btc"`
		ToEth string `json:"to_eth"`
	} `json:"4"`
}

type BalanceArgs struct {
	Address string `json:"address"`
}

type Balance struct {
	Address           string `json:"address"`
	Received          int64  `json:"received"`
	Spent             int64  `json:"spent"`
	CountReceived     int64  `json:"count_received"`
	CountSpent        int64  `json:"count_spent"`
	CountTxs          int64  `json:"count_txs"`
	BlockNumber       int64  `json:"block_number"`
	CurrentBlock      int64  `json:"currentBlock"`
	Hash              string `json:"hash"`
	CountDelegatedOps int64  `json:"countDelegatedOps"`
	Delegate          int64  `json:"delegate"`
	Undelegate        int64  `json:"undelegate"`
	Delegated         int64  `json:"delegated"`
	Undelegated       int64  `json:"undelegated"`
	Reserved          int64  `json:"reserved"`
	CountForgedOps    int64  `json:"countForgedOps"`
	Forged            int64  `json:"forged"`

	_ToHardCap int64
}

func (b *Balance) CurrentBalance() int64 {
	if b.Received == 0 && b.Spent == 0 {
		return 0
	}
	return b.Received - b.Spent
}

func (b *Balance) FullBalance() int64 {
	return b.CurrentBalance() + b.DelegatedFunds()
}

func (b *Balance) TransactionsCount() int64 {
	return b.CountTxs
}

func (b *Balance) DelegatedAmount() int64 {

	if b.Delegated == 0 {
		return 0
	}

	// 1e6 - первоначальный капитал регистрации ноды
	return b.Delegated - b.Undelegated + 1e6
}

func (b *Balance) ToHardCap() int64 {

	if b.Delegated == 0 {
		return 0
	}

	if b._ToHardCap != 0 {
		return b._ToHardCap
	}

	// 10000000000000 - сумма для хардкапа
	b._ToHardCap = 10000000000000 - b.DelegatedAmount()
	return b._ToHardCap
}

// сколько делегировано на адрес
func (b *Balance) Funded() int64 {
	return b.Delegated - b.Undelegated
}

// сколько делегировал адрес
func (b *Balance) DelegatedFunds() int64 {
	return b.Delegate - b.Undelegate
}

func (b *Balance) SeedCapital() int64 {
	return b.DelegatedFunds()
}

type TransactionArgs struct {
	Hash string `json:"hash"`
}

type Transaction struct {
	Transaction TransactionInfo `json:"transaction"`
	CountBlocks int64           `json:"countBlocks"`
	KnownBlocks int64           `json:"knownBlocks"`
}

type HistoryArgs struct {
	Address  string `json:"address"`
	BeginTx  int64  `json:"beginTx,omitempty"`
	CountTxs int64  `json:"countTxs,omitempty"`
}

type BlockByNumberArgs struct {
	Number   int64 `json:"number"`
	BeginTx  int64 `json:"beginTx,omitempty"`
	CountTxs int64 `json:"countTxs,omitempty"`
	Type     int8  `json:"type,omitempty"` // 0-4
	// 0 - простой тип с 7 подписями
	// 1 - 7 подписей и массив хешей всех транзакций
	// 2 - 7 подписей и все транзакции полностью
	// 3 - краткий формат блока, только прошлый и текущий хеш
	// 4 - краткий формат 3 + размер блока и расположение файла данных
}

type BlockByHashArgs struct {
	Hash     string `json:"hash"`
	BeginTx  int64  `json:"beginTx,omitempty"`
	CountTxs int64  `json:"countTxs,omitempty"`
	Type     int8   `json:"type,omitempty"` // 0-4
}

type BlocksArgs struct {
	CountBlocks int64 `json:"countBlocks,omitempty"`
	BeginBlock  int64 `json:"beginBlock,omitempty"`
}

type Block struct {
	Type       string `json:"type"`
	Hash       string `json:"hash"`
	PrevHash   string `json:"prev_hash"`
	TxHash     string `json:"tx_hash"`
	Number     int64  `json:"number"`
	TimeStamp  int64  `json:"timestamp"`
	CountTxs   int64  `json:"count_txs"`
	Sign       string `json:"sign"`
	Size       int64  `json:"size"`
	FileName   string `json:"fileName"`
	Signatures []struct {
		From        string `json:"from"`
		To          string `json:"to"`
		Value       int64  `json:"value"`
		Transaction string `json:"transaction"`
		Data        string `json:"data"`
		TimeStamp   int64  `json:"timestamp"`
		Type        string `json:"type"`
		BlockNumber int64  `json:"blockNumber"`
		Signature   string `json:"signature"`
		Publickey   string `json:"publickey"`
		Fee         int64  `json:"fee"`
		RealFee     int64  `json:"realFee"`
		Nonce       int64  `json:"nonce"`
		IntStatus   int64  `json:"intStatus"`
		Status      string `json:"status"`
	} `json:"signatures,omitempty"`
	Txs []*TransactionInfo `json:"txs"`
}

// если есть подписи - блок подписан
func (b *Block) IsSigned() bool {
	return len(b.Signatures) > 0
}

func (b *Block) Time() time.Time {
	return time.Unix(b.TimeStamp, 0)
}

func (b *Block) Output() int64 {

	var amount int64
	for _, tx := range b.Txs {
		amount += tx.Value
	}

	return amount
}

func (b *Block) PrevNumber() int64 {
	return b.Number - 1
}

func (b *Block) NextNumber() int64 {
	return b.Number + 1
}

type LastTxsArgs struct {
}

type TransactionInfo struct {
	From         string `json:"from" db:"fromA"`
	To           string `json:"to" db:"toA"`
	Value        int64  `json:"value"`
	Transaction  string `json:"transaction"`
	Data         string `json:"data"`
	TimeStamp    int64  `json:"timestamp" db:"timestamp,int64"`
	Type         string `json:"type" db:"typeTx"`
	BlockNumber  int64  `json:"blockNumber" db:"blockNumber"`
	Signature    string `json:"signature"`
	PublicKey    string `json:"publickey"`
	Fee          int64  `json:"fee"`
	RealFee      int64  `json:"realFee" db:"realFee"`
	Nonce        int64  `json:"nonce"`
	IntStatus    int64  `json:"intStatus" db:"intStatus"`
	Status       string `json:"status"`
	IsDelegate   bool   `json:"isDelegate,omitempty" db:"isDelegate"`
	DelegateInfo struct {
		IsDelegate   bool   `json:"isDelegate"`
		Delegate     int64  `json:"delegate,omitempty"`
		DelegateHash string `json:"delegateHash,omitempty"`
	} `json:"delegate_info,omitempty" db:"-"`
	Delegate     int64  `json:"delegate,omitempty"`
	DelegateHash string `json:"delegateHash,omitempty" db:"delegateHash"`
}

func (ti *TransactionInfo) DataString() string {
	dst := make([]byte, hex.DecodedLen(len(ti.Data)))
	n, err := hex.Decode(dst, []byte(ti.Data))
	if err != nil {
		return ""
	}

	return strings.ReplaceAll(string(dst[:n]), "�\\", "|")
}

func (ti *TransactionInfo) DataByte() []byte {

	dst := make([]byte, hex.DecodedLen(len(ti.Data)))
	n, err := hex.Decode(dst, []byte(ti.Data))
	if err != nil {
		return []byte{}
	}

	return dst[:n]
}

func (ti *TransactionInfo) NodeRegistrationData() (nodeInfo NodeRegistration) {

	nodeInfo.Params.Name = ti.From
	dataString := ti.DataString()
	if dataString != "" && (strings.Contains(dataString, "mh-noderegistration") || strings.Contains(dataString, "mhRegisterNode")) {
		json.Unmarshal([]byte(dataString), &nodeInfo)
	}

	if nodeInfo.Params.Type == "" {
		nodeInfo.Params.Type = "Proxy"
	}

	return nodeInfo
}

func (ti *TransactionInfo) NodeHost() string {
	return ti.NodeRegistrationData().Params.Host
}

func (ti *TransactionInfo) NodeType() string {
	return ti.NodeRegistrationData().Params.Type
}

func (ti *TransactionInfo) NodeName() string {

	nodeName := ti.NodeRegistrationData().Params.Name

	if len(nodeName) >= 48 {
		return nodeName[0:47] + "…"
	}

	return nodeName
}

func (ti *TransactionInfo) NodeTest() (nodeInfo NodeTest, isTest bool) {

	dataString := ti.DataString()
	if dataString != "" && strings.Contains(dataString, "proxy_load_results") {
		nodeInfo.TimeStamp = ti.TimeStamp
		json.Unmarshal([]byte(dataString), &nodeInfo)
		isTest = true
	}

	return nodeInfo, isTest
}

func (ti *TransactionInfo) Method() string {
	return ti.Action()
}

// @todo переименовать в method
func (ti *TransactionInfo) Action() string {

	// https://github.com/metahashorg/MetaHash/wiki/Transactions
	if ti.IntStatus == 1 {
		return "approve"
	}

	if ti.IntStatus == 20 {

		dataString := ti.DataString()
		if dataString != "" && strings.Contains(dataString, "method") {
			method := AbstractMethod{}
			if err := json.Unmarshal(EscapeCtrl([]byte(dataString)), &method); err != nil {
				return "accepted"
			}

			if ti.To == "0x666174686572206f662077616c6c65747320666f7267696e67" {
				if method.Method == "delegate" {
					return "start forging"
				}

				if method.Method == "undelegate" {
					return "stop forging"
				}
			}

			return method.Method
		}

		return "accepted"
	}

	if ti.IntStatus == 40 {
		return "not accepted"
	}

	if ti.IntStatus == 100 {
		return "forging"
	}

	if ti.IntStatus == 101 {
		return "wallet reward"
	}

	if ti.IntStatus == 102 {
		return "node reward"
	}

	if ti.IntStatus == 103 {
		return "coin reward"
	}

	if ti.IntStatus == 104 {
		return "random reward"
	}

	if ti.IntStatus == 200 {
		return "state block"
	}

	if ti.Type == "forging" {
		return "forging"
	}

	if ti.From == "InitialWalletTransaction" {
		return "InitialWalletTransaction"
	}

	if ti.IntStatus == 4353 {
		return "node test"
	}

	return "pay"
}

// @todo переименовать в methodparams
func (ti *TransactionInfo) ActionParams() interface{} {

	dataString := ti.DataString()
	if dataString != "" && strings.Contains(dataString, "method") {
		method := AbstractMethod{}
		if err := json.Unmarshal(EscapeCtrl([]byte(dataString)), &method); err != nil {
			return "error parse"
		}

		return method.Params
	}

	return nil
}

func (ti *TransactionInfo) ActionValue() string {

	if ti.Type == "forging" {
		return strconv.FormatInt(ti.Value, 10)
	}

	if ti.From == "InitialWalletTransaction" {
		return strconv.FormatInt(ti.Value, 10)
	}

	if ti.Delegate != 0 {
		return strconv.FormatInt(ti.Delegate, 10)
	}

	if ti.NodeRegistrationData().Params.Name == "" {
		return ti.Status
	}

	return strconv.FormatInt(ti.Value, 10)
}

func (ti *TransactionInfo) Time() time.Time {
	return time.Unix(ti.TimeStamp, 0)
}

func (ti *TransactionInfo) StatusOK() bool {
	return ti.Status == "ok" || ti.Status == "pending"
}

func substring(start, end int, s string) string {
	if start < 0 {
		return s[:end]
	}
	if end < 0 || end > len(s) {
		return s[start:]
	}
	return s[start:end]
}

type DumpBlockByNumberArgs struct {
	Number int64 `json:"number"`
	IsHex  bool  `json:"isHex,omitempty"`
}

type DumpBlockByHashArgs struct {
	Hash  string `json:"hash"`
	IsHex bool   `json:"isHex,omitempty"`
}

type DumpBlock struct {
	Dump string `json:"dump"`
}

type CountBlocksArgs struct {
}

type CountBlocks struct {
	CountBlocks int64 `json:"count_blocks"`
	BeginBlock  int64 `json:"beginBlock"`
}

// внутренние структуры-методы
type NodeRegistration struct {
	Jsonrpc string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  struct {
		Host string `json:"host"`
		Name string `json:"name"`
		Type string `json:"type,omitempty"`
	} `json:"params"`
}

type AbstractMethod struct {
	Jsonrpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type NodeTest struct {
	TimeStamp int64  `json:"timestamp"`
	Method    string `json:"method"`
	Params    struct {
		Mhaddr   string `json:"mhaddr"`
		IP       string `json:"ip"`
		QPS      int64  `json:"qps"`
		Rps      int64  `json:"rps"`
		Closed   string `json:"closed"`
		Timeouts string `json:"timeouts"`
		Ver      string `json:"ver"`
		Success  string `json:"success"`
	} `json:"params"`
}

func EscapeCtrl(ctrl []byte) (esc []byte) {
	u := []byte(`\u0000`)
	for i, ch := range ctrl {
		if ch <= 31 {
			if esc == nil {
				esc = append(make([]byte, 0, len(ctrl)+len(u)), ctrl[:i]...)
			}
			esc = append(esc, u...)
			hex.Encode(esc[len(esc)-2:], ctrl[i:i+1])
			continue
		}
		if esc != nil {
			esc = append(esc, ch)
		}
	}
	if esc == nil {
		return ctrl
	}

	return esc
}
